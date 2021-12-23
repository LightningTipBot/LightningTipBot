package telegram

import (
	"context"
	"fmt"
	"strings"

	"github.com/LightningTipBot/LightningTipBot/internal/runtime/mutex"
	"github.com/LightningTipBot/LightningTipBot/internal/storage"

	"github.com/LightningTipBot/LightningTipBot/internal/i18n"
	"github.com/LightningTipBot/LightningTipBot/internal/lnbits"
	"github.com/LightningTipBot/LightningTipBot/internal/runtime"

	"github.com/LightningTipBot/LightningTipBot/internal/str"
	decodepay "github.com/fiatjaf/ln-decodepay"
	log "github.com/sirupsen/logrus"
	tb "gopkg.in/lightningtipbot/telebot.v2"
)

var (
	paymentConfirmationMenu = &tb.ReplyMarkup{ResizeReplyKeyboard: true}
	btnCancelPay            = paymentConfirmationMenu.Data("🚫 Cancel", "cancel_pay")
	btnPay                  = paymentConfirmationMenu.Data("✅ Pay", "confirm_pay")
)

func helpPayInvoiceUsage(ctx context.Context, errormsg string) string {
	if len(errormsg) > 0 {
		return fmt.Sprintf(Translate(ctx, "payHelpText"), fmt.Sprintf("%s", errormsg))
	} else {
		return fmt.Sprintf(Translate(ctx, "payHelpText"), "")
	}
}

type PayData struct {
	*storage.Base
	From            *lnbits.User `json:"from"`
	Invoice         string       `json:"invoice"`
	Hash            string       `json:"hash"`
	Proof           string       `json:"proof"`
	Memo            string       `json:"memo"`
	Message         string       `json:"message"`
	Amount          int64        `json:"amount"`
	LanguageCode    string       `json:"languagecode"`
	TelegramMessage *tb.Message  `json:"telegrammessage"`
}

// payHandler invoked on "/pay lnbc..." command
func (bot *TipBot) payHandler(ctx context.Context, m *tb.Message) {
	// check and print all commands
	bot.anyTextHandler(ctx, m)
	user := LoadUser(ctx)
	if user.Wallet == nil {
		return
	}
	if len(strings.Split(m.Text, " ")) < 2 {
		NewMessage(m, WithDuration(0, bot))
		bot.trySendMessage(m.Sender, helpPayInvoiceUsage(ctx, ""))
		return
	}
	userStr := GetUserStr(m.Sender)
	paymentRequest, err := getArgumentFromCommand(m.Text, 1)
	if err != nil {
		NewMessage(m, WithDuration(0, bot))
		bot.trySendMessage(m.Sender, helpPayInvoiceUsage(ctx, Translate(ctx, "invalidInvoiceHelpMessage")))
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
		bot.trySendMessage(m.Sender, helpPayInvoiceUsage(ctx, Translate(ctx, "invalidInvoiceHelpMessage")))
		errmsg := fmt.Sprintf("[/pay] Error: Could not decode invoice: %s", err)
		log.Errorln(errmsg)
		return
	}
	amount := int64(bolt11.MSatoshi / 1000)

	if amount <= 0 {
		bot.trySendMessage(m.Sender, Translate(ctx, "invoiceNoAmountMessage"))
		errmsg := fmt.Sprint("[/pay] Error: invoice without amount")
		log.Warnln(errmsg)
		return
	}

	// check user balance first
	balance, err := bot.GetUserBalance(user)
	if err != nil {
		NewMessage(m, WithDuration(0, bot))
		errmsg := fmt.Sprintf("[/pay] Error: Could not get user balance: %s", err)
		log.Errorln(errmsg)
		bot.trySendMessage(m.Sender, Translate(ctx, "errorTryLaterMessage"))
		return
	}

	if amount > balance {
		NewMessage(m, WithDuration(0, bot))
		bot.trySendMessage(m.Sender, fmt.Sprintf(Translate(ctx, "insufficientFundsMessage"), balance, amount))
		return
	}
	// send warning that the invoice might fail due to missing fee reserve
	if float64(amount) > float64(balance)*0.99 {
		bot.trySendMessage(m.Sender, Translate(ctx, "feeReserveMessage"))
	}

	confirmText := fmt.Sprintf(Translate(ctx, "confirmPayInvoiceMessage"), amount)
	if len(bolt11.Description) > 0 {
		confirmText = confirmText + fmt.Sprintf(Translate(ctx, "confirmPayAppendMemo"), str.MarkdownEscape(bolt11.Description))
	}

	log.Infof("[/pay] Invoice entered. User: %s, amount: %d sat.", userStr, amount)

	// object that holds all information about the send payment
	id := fmt.Sprintf("pay-%d-%d-%s", m.Sender.ID, amount, RandStringRunes(5))

	// // // create inline buttons
	payButton := paymentConfirmationMenu.Data(Translate(ctx, "payButtonMessage"), "confirm_pay")
	cancelButton := paymentConfirmationMenu.Data(Translate(ctx, "cancelButtonMessage"), "cancel_pay")
	payButton.Data = id
	cancelButton.Data = id

	paymentConfirmationMenu.Inline(
		paymentConfirmationMenu.Row(
			payButton,
			cancelButton),
	)
	payMessage := bot.trySendMessage(m.Chat, confirmText, paymentConfirmationMenu)
	payData := &PayData{
		Base:            storage.New(storage.ID(id)),
		From:            user,
		Invoice:         paymentRequest,
		Amount:          int64(amount),
		Memo:            bolt11.Description,
		Message:         confirmText,
		LanguageCode:    ctx.Value("publicLanguageCode").(string),
		TelegramMessage: payMessage,
	}
	// add result to persistent struct
	runtime.IgnoreError(payData.Set(payData, bot.Bunt))

	SetUserState(user, bot, lnbits.UserStateConfirmPayment, paymentRequest)
}

// confirmPayHandler when user clicked pay on payment confirmation
func (bot *TipBot) confirmPayHandler(ctx context.Context, c *tb.Callback) {
	tx := &PayData{Base: storage.New(storage.ID(c.Data))}
	mutex.Lock(tx.ID)
	defer mutex.Unlock(tx.ID)
	sn, err := tx.Get(tx, bot.Bunt)
	// immediatelly set intransaction to block duplicate calls
	if err != nil {
		log.Errorf("[confirmPayHandler] %s", err)
		return
	}
	payData := sn.(*PayData)

	// onnly the correct user can press
	if payData.From.Telegram.ID != c.Sender.ID {
		return
	}
	if !payData.Active {
		log.Errorf("[confirmPayHandler] send not active anymore")
		bot.tryEditMessage(c.Message, i18n.Translate(payData.LanguageCode, "errorTryLaterMessage"), &tb.ReplyMarkup{})
		bot.tryDeleteMessage(c.Message)
		return
	}
	defer payData.Set(payData, bot.Bunt)

	// remove buttons from confirmation message
	// bot.tryEditMessage(c.Message, MarkdownEscape(payData.Message), &tb.ReplyMarkup{})

	user := LoadUser(ctx)
	if user.Wallet == nil {
		bot.tryDeleteMessage(c.Message)
		return
	}

	invoiceString := payData.Invoice

	// reset state immediately
	ResetUserState(user, bot)

	userStr := GetUserStr(c.Sender)

	// update button text
	bot.tryEditMessage(
		c.Message,
		payData.Message,
		&tb.ReplyMarkup{
			InlineKeyboard: [][]tb.InlineButton{
				{tb.InlineButton{Text: i18n.Translate(payData.LanguageCode, "lnurlGettingUserMessage")}},
			},
		},
	)

	log.Infof("[/pay] Attempting %s's invoice %s (%d sat)", userStr, payData.ID, payData.Amount)
	// pay invoice
	invoice, err := user.Wallet.Pay(lnbits.PaymentParams{Out: true, Bolt11: invoiceString}, bot.Client)
	if err != nil {
		errmsg := fmt.Sprintf("[/pay] Could not pay invoice of %s: %s", userStr, err)
		err = fmt.Errorf(i18n.Translate(payData.LanguageCode, "invoiceUndefinedErrorMessage"))
		bot.tryEditMessage(c.Message, fmt.Sprintf(i18n.Translate(payData.LanguageCode, "invoicePaymentFailedMessage"), err.Error()), &tb.ReplyMarkup{})
		// verbose error message, turned off for now
		// if len(err.Error()) == 0 {
		// 	err = fmt.Errorf(i18n.Translate(payData.LanguageCode, "invoiceUndefinedErrorMessage"))
		// }
		// bot.tryEditMessage(c.Message, fmt.Sprintf(i18n.Translate(payData.LanguageCode, "invoicePaymentFailedMessage"), str.MarkdownEscape(err.Error())), &tb.ReplyMarkup{})
		log.Errorln(errmsg)
		return
	}
	payData.Hash = invoice.PaymentHash

	if c.Message.Private() {
		// if the command was invoked in private chat
		bot.tryEditMessage(c.Message, i18n.Translate(payData.LanguageCode, "invoicePaidMessage"), &tb.ReplyMarkup{})
	} else {
		// if the command was invoked in group chat
		bot.trySendMessage(c.Sender, i18n.Translate(payData.LanguageCode, "invoicePaidMessage"))
		bot.tryEditMessage(c.Message, fmt.Sprintf(i18n.Translate(payData.LanguageCode, "invoicePublicPaidMessage"), userStr), &tb.ReplyMarkup{})
	}
	log.Infof("[⚡️ pay] User %s paid invoice %s (%d sat)", userStr, payData.ID, payData.Amount)
	return
}

// cancelPaymentHandler invoked when user clicked cancel on payment confirmation
func (bot *TipBot) cancelPaymentHandler(ctx context.Context, c *tb.Callback) {
	// reset state immediately
	user := LoadUser(ctx)
	ResetUserState(user, bot)
	tx := &PayData{Base: storage.New(storage.ID(c.Data))}
	mutex.Lock(tx.ID)
	defer mutex.Unlock(tx.ID)
	// immediatelly set intransaction to block duplicate calls
	sn, err := tx.Get(tx, bot.Bunt)
	if err != nil {
		log.Errorf("[cancelPaymentHandler] %s", err.Error())
		return
	}
	payData := sn.(*PayData)
	// onnly the correct user can press
	if payData.From.Telegram.ID != c.Sender.ID {
		return
	}
	bot.tryEditMessage(c.Message, i18n.Translate(payData.LanguageCode, "paymentCancelledMessage"), &tb.ReplyMarkup{})
	payData.Inactivate(payData, bot.Bunt)
}
