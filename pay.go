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
	invoicePaidMessage                 = "⚡️ Invoice paid."
	invoicePrivateChatOnlyErrorMessage = "You can pay invoices only in the private chat with the bot."
	invalidInvoiceHelpMessage          = "Did you enter a valid Lightning invoice?"
	invoiceNoAmountMessage             = "🚫 Can't pay invoices without an amount."
	insufficiendFundsMessage           = "🚫 Insufficient funds. You have %d sat but you need at least %d sat."
	invoicePaymentFailedMessage        = "🚫 Failed to pay invoice: %s"
	confirmPayInvoiceMessage           = "Do you want to pay this invoice?\n💸 Amount: %d sat"
	confirmPayAppendMemo               = "\n✉️ %s"
	payHelpText                        = "📖 Oops, that didn't work. %s\n\n" +
		"*Usage:* `/pay <invoice>`\n" +
		"*Example:* `/pay lnbc20n1psscehd...`"
)

func helpPayInvoiceUsage(errormsg string) string {
	if len(errormsg) > 0 {
		return fmt.Sprintf(payHelpText, fmt.Sprintf("_%s_", errormsg))
	} else {
		return fmt.Sprintf(payHelpText, "")
	}
}

// confirmPaymentHandler invoked on "/pay lnbc..." command
func (bot TipBot) confirmPaymentHandler(m *tb.Message) {
	log.Infof("[%s:%d %s:%d] %s", m.Chat.Title, m.Chat.ID, GetUserStr(m.Sender), m.Sender.ID, m.Text)
	if m.Chat.Type != tb.ChatPrivate {
		// delete message
		NewMessage(m).Dispose(0, bot.telegram)
		bot.telegram.Send(m.Sender, helpPayInvoiceUsage(invoicePrivateChatOnlyErrorMessage))
		return
	}
	if len(strings.Split(m.Text, " ")) < 2 {
		NewMessage(m).Dispose(0, bot.telegram)
		bot.telegram.Send(m.Sender, helpPayInvoiceUsage(""))
		return
	}
	user, err := GetUser(m.Sender, bot)
	userStr := GetUserStr(m.Sender)
	payment_request, err := getArgumentFromCommand(m.Text, 1)
	payment_request = strings.ToLower(payment_request)
	// get rid of the URI prefix
	payment_request = strings.TrimPrefix(payment_request, "lightning:")

	// decode invoice
	bolt11, err := decodepay.Decodepay(payment_request)
	if err != nil {
		bot.telegram.Send(m.Sender, helpPayInvoiceUsage(invalidInvoiceHelpMessage))
		errmsg := fmt.Sprintf("[/pay] Error: Could not decode invoice: %s", err)
		log.Errorln(errmsg)
		return
	}
	amount := int(bolt11.MSatoshi / 1000)

	if !(amount > 0) {
		bot.telegram.Send(m.Sender, invoiceNoAmountMessage)
		errmsg := fmt.Sprint("[/pay] Error: invoice without amount")
		log.Errorln(errmsg)
		return
	}
	// description := bolt11.Description
	// log.Printf("[Pay Invoice] Description: %s", description)

	// check user balance first
	balance, err := bot.GetUserBalance(m.Sender)
	if err != nil {
		NewMessage(m).Dispose(0, bot.telegram)
		errmsg := fmt.Sprintf("[/pay] Error: Could not get user balance: %s", err)
		log.Errorln(errmsg)
		return
	}
	if amount > balance {
		NewMessage(m).Dispose(0, bot.telegram)
		bot.telegram.Send(m.Sender, fmt.Sprintf(insufficiendFundsMessage, balance, amount))
		return
	}

	log.Printf("[/pay] User: %s, amount: %d sat.", userStr, amount)
	user.StateKey = lnbits.UserStateConfirmPayment
	user.StateData = payment_request
	err = UpdateUserRecord(user, bot)
	if err != nil {
		log.Printf("[UpdateUserRecord] User: %s Error: %s", userStr, err.Error())
		return
	}

	// // // create inline buttons
	paymentConfirmationMenu.Inline(paymentConfirmationMenu.Row(btnPay, btnCancelPay))
	confirmText := fmt.Sprintf(confirmPayInvoiceMessage, amount)
	if len(bolt11.Description) > 0 {
		confirmText = confirmText + fmt.Sprintf(confirmPayAppendMemo, MarkdownEscape(bolt11.Description))
	}
	bot.telegram.Send(m.Sender, confirmText, paymentConfirmationMenu)

}

// cancelPaymentHandler invoked when user clicked cancel on payment confirmation
func (bot TipBot) cancelPaymentHandler(c *tb.Callback) {
	// reset state immediately
	user, err := GetUser(c.Sender, bot)
	if err != nil {
		return
	}
	user.ResetState()
	err = UpdateUserRecord(user, bot)

	bot.telegram.Delete(c.Message)
	_, err = bot.telegram.Send(c.Sender, paymentCancelledMessage)
	if err != nil {
		log.WithField("message", paymentCancelledMessage).WithField("user", c.Sender.ID).Printf("[Send] %s", err.Error())
		return
	}

}

// payHandler when user clicked pay "X" on payment confirmation
func (bot TipBot) payHandler(c *tb.Callback) {
	bot.telegram.Edit(c.Message, c.Message.Text, &tb.ReplyMarkup{})
	user, err := GetUser(c.Sender, bot)
	if err != nil {
		log.Printf("[GetUser] User: %d: %s", c.Sender.ID, err.Error())
		return
	}
	if user.StateKey == lnbits.UserStateConfirmPayment {
		invoiceString := user.StateData

		// reset state immediatelly
		user.ResetState()
		err = UpdateUserRecord(user, bot)

		userStr := GetUserStr(c.Sender)
		// pay invoice
		invoice, err := user.Wallet.Pay(lnbits.PaymentParams{Out: true, Bolt11: invoiceString}, *user.Wallet)
		if err != nil {
			errmsg := fmt.Sprintf("[/pay] Could not pay invoice of user %s: %s", userStr, err)
			bot.telegram.Send(c.Sender, fmt.Sprintf(invoicePaymentFailedMessage, err))
			log.Errorln(errmsg)
			return
		}
		bot.telegram.Send(c.Sender, invoicePaidMessage)
		log.Printf("[/pay] User %s paid invoice %s", userStr, invoice.PaymentHash)
		return
	}

}
