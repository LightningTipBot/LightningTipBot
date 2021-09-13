package main

import (
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/LightningTipBot/LightningTipBot/internal/lnbits"
	decodepay "github.com/fiatjaf/ln-decodepay"
	tb "gopkg.in/tucnak/telebot.v2"
)

const (
	paymentCancelledMessage            = "🚫 Payment cancelled."
	invoicePaidMessage                 = "⚡️ Payment sent."
	invoicePrivateChatOnlyErrorMessage = "You can pay invoices only in the private chat with the bot."
	invalidInvoiceHelpMessage          = "Did you enter a valid Lightning invoice? Try /send if you want to send to a Telegram user or Lightning address."
	invoiceNoAmountMessage             = "🚫 Can't pay invoices without an amount."
	insufficientFundsMessage           = "🚫 Insufficient funds. You have %d sat but you need at least %d sat."
	feeReserveMessage                  = "⚠️ Sending your entire balance might fail because of network fees. If it fails, try sending a bit less."
	invoicePaymentFailedMessage        = "🚫 Payment failed: %s"
	confirmPayInvoiceMessage           = "Do you want to send this payment?\n\n💸 Amount: %d sat"
	confirmPayAppendMemo               = "\n✉️ %s"
	payHelpText                        = "📖 Oops, that didn't work. %s\n\n" +
		"*Usage:* `/pay <invoice>`\n" +
		"*Example:* `/pay lnbc20n1psscehd...`"
)

func helpPayInvoiceUsage(errormsg string) string {
	if len(errormsg) > 0 {
		return fmt.Sprintf(payHelpText, fmt.Sprintf("%s", errormsg))
	} else {
		return fmt.Sprintf(payHelpText, "")
	}
}

// confirmPaymentHandler invoked on "/pay lnbc..." command
func (bot TipBot) confirmPaymentHandler(m *tb.Message) {
	// check and print all commands
	bot.anyTextHandler(m)
	if m.Chat.Type != tb.ChatPrivate {
		// delete message
		NewMessage(m, WithDuration(0, bot.telegram))
		bot.trySendMessage(m.Sender, helpPayInvoiceUsage(invoicePrivateChatOnlyErrorMessage))
		return
	}
	if len(strings.Split(m.Text, " ")) < 2 {
		NewMessage(m, WithDuration(0, bot.telegram))
		bot.trySendMessage(m.Sender, helpPayInvoiceUsage(""))
		return
	}
	user, err := GetUser(m.Sender, bot)
	if err != nil {
		NewMessage(m, WithDuration(0, bot.telegram))
		errmsg := fmt.Sprintf("[/pay] Error: Could not GetUser: %s", err)
		log.Errorln(errmsg)
		return
	}
	userStr := GetUserStr(m.Sender)
	paymentRequest, err := getArgumentFromCommand(m.Text, 1)
	if err != nil {
		NewMessage(m, WithDuration(0, bot.telegram))
		bot.trySendMessage(m.Sender, helpPayInvoiceUsage(invalidInvoiceHelpMessage))
		errmsg := fmt.Sprintf("[/pay] Error: Could not getArgumentFromCommand: %s", err)
		log.Errorln(errmsg)
		return
	}
	paymentRequest = strings.ToLower(paymentRequest)
	// get rid of the URI prefix
	paymentRequest = strings.TrimPrefix(paymentRequest, "lightning:")

	// decode invoice
	bolt11, err := decodepay.Decodepay(paymentRequest)
	if err != nil {
		bot.trySendMessage(m.Sender, helpPayInvoiceUsage(invalidInvoiceHelpMessage))
		errmsg := fmt.Sprintf("[/pay] Error: Could not decode invoice: %s", err)
		log.Errorln(errmsg)
		return
	}
	amount := int(bolt11.MSatoshi / 1000)

	if amount <= 0 {
		bot.trySendMessage(m.Sender, invoiceNoAmountMessage)
		errmsg := fmt.Sprint("[/pay] Error: invoice without amount")
		log.Errorln(errmsg)
		return
	}

	// check user balance first
	balance, err := bot.GetUserBalance(m.Sender)
	if err != nil {
		NewMessage(m, WithDuration(0, bot.telegram))
		errmsg := fmt.Sprintf("[/pay] Error: Could not get user balance: %s", err)
		log.Errorln(errmsg)
		return
	}
	if amount > balance {
		NewMessage(m, WithDuration(0, bot.telegram))
		bot.trySendMessage(m.Sender, fmt.Sprintf(insufficientFundsMessage, balance, amount))
		return
	}
	// send warning that the invoice might fail due to missing fee reserve
	if float64(amount) > float64(balance)*0.99 {
		bot.trySendMessage(m.Sender, feeReserveMessage)
	}

	log.Printf("[/pay] User: %s, amount: %d sat.", userStr, amount)

	SetUserState(user, bot, lnbits.UserStateConfirmPayment, paymentRequest)

	// // // create inline buttons
	paymentConfirmationMenu.Inline(paymentConfirmationMenu.Row(btnPay, btnCancelPay))
	confirmText := fmt.Sprintf(confirmPayInvoiceMessage, amount)
	if len(bolt11.Description) > 0 {
		confirmText = confirmText + fmt.Sprintf(confirmPayAppendMemo, MarkdownEscape(bolt11.Description))
	}
	bot.trySendMessage(m.Sender, confirmText, paymentConfirmationMenu)
}

// cancelPaymentHandler invoked when user clicked cancel on payment confirmation
func (bot TipBot) cancelPaymentHandler(c *tb.Callback) {
	// reset state immediately
	user, err := GetUser(c.Sender, bot)
	if err != nil {
		return
	}
	ResetUserState(user, bot)

	bot.tryDeleteMessage(c.Message)
	_, err = bot.telegram.Send(c.Sender, paymentCancelledMessage)
	if err != nil {
		log.WithField("message", paymentCancelledMessage).WithField("user", c.Sender.ID).Printf("[Send] %s", err.Error())
		return
	}

}

// payHandler when user clicked pay "X" on payment confirmation
func (bot TipBot) payHandler(c *tb.Callback) {
	bot.tryEditMessage(c.Message, c.Message.Text, &tb.ReplyMarkup{})
	user, err := GetUser(c.Sender, bot)
	if err != nil {
		log.Printf("[GetUser] User: %d: %s", c.Sender.ID, err.Error())
		return
	}
	if user.StateKey == lnbits.UserStateConfirmPayment {
		invoiceString := user.StateData

		// reset state immediatelly
		ResetUserState(user, bot)

		userStr := GetUserStr(c.Sender)
		// pay invoice
		invoice, err := user.Wallet.Pay(lnbits.PaymentParams{Out: true, Bolt11: invoiceString}, *user.Wallet)
		if err != nil {
			errmsg := fmt.Sprintf("[/pay] Could not pay invoice of user %s: %s", userStr, err)
			bot.trySendMessage(c.Sender, fmt.Sprintf(invoicePaymentFailedMessage, err))
			log.Errorln(errmsg)
			return
		}
		bot.trySendMessage(c.Sender, invoicePaidMessage)
		log.Printf("[/pay] User %s paid invoice %s", userStr, invoice.PaymentHash)
		return
	}

}
