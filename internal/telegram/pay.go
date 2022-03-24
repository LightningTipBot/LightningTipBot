package telegram

import (
	"context"
	"fmt"
	"github.com/LightningTipBot/LightningTipBot/internal/telegram/intercept"
	"strings"

	"github.com/LightningTipBot/LightningTipBot/internal/errors"

	"github.com/LightningTipBot/LightningTipBot/internal/runtime/mutex"
	"github.com/LightningTipBot/LightningTipBot/internal/storage"

	"github.com/LightningTipBot/LightningTipBot/internal/i18n"
	"github.com/LightningTipBot/LightningTipBot/internal/lnbits"
	"github.com/LightningTipBot/LightningTipBot/internal/runtime"

	"github.com/LightningTipBot/LightningTipBot/internal/str"
	decodepay "github.com/fiatjaf/ln-decodepay"
	log "github.com/sirupsen/logrus"
	tb "gopkg.in/telebot.v3"
)

var (
	paymentConfirmationMenu = &tb.ReplyMarkup{ResizeKeyboard: true}
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
func (bot *TipBot) payHandler(handler intercept.Handler) (intercept.Handler, error) {
	// check and print all commands
	bot.anyTextHandler(handler)
	user := LoadUser(handler.Ctx)
	if user.Wallet == nil {
		return handler, errors.Create(errors.UserNoWalletError)
	}
	if len(strings.Split(handler.Text(), " ")) < 2 {
		NewMessage(handler.Message(), WithDuration(0, bot))
		bot.trySendMessage(handler.Sender(), helpPayInvoiceUsage(handler.Ctx, ""))
		return handler, errors.Create(errors.InvalidSyntaxError)
	}
	userStr := GetUserStr(handler.Sender())
	paymentRequest, err := getArgumentFromCommand(handler.Text(), 1)
	if err != nil {
		NewMessage(handler.Message(), WithDuration(0, bot))
		bot.trySendMessage(handler.Sender(), helpPayInvoiceUsage(handler.Ctx, Translate(handler.Ctx, "invalidInvoiceHelpMessage")))
		errmsg := fmt.Sprintf("[/pay] Error: Could not getArgumentFromCommand: %s", err.Error())
		log.Errorln(errmsg)
		return handler, errors.New(errors.InvalidSyntaxError, err)
	}
	paymentRequest = strings.ToLower(paymentRequest)
	// get rid of the URI prefix
	paymentRequest = strings.TrimPrefix(paymentRequest, "lightning:")

	// decode invoice
	bolt11, err := decodepay.Decodepay(paymentRequest)
	if err != nil {
		bot.trySendMessage(handler.Sender(), helpPayInvoiceUsage(handler.Ctx, Translate(handler.Ctx, "invalidInvoiceHelpMessage")))
		errmsg := fmt.Sprintf("[/pay] Error: Could not decode invoice: %s", err.Error())
		log.Errorln(errmsg)
		return handler, errors.New(errors.InvalidSyntaxError, err)
	}
	amount := int64(bolt11.MSatoshi / 1000)

	if amount <= 0 {
		bot.trySendMessage(handler.Sender(), Translate(handler.Ctx, "invoiceNoAmountMessage"))
		errmsg := fmt.Sprint("[/pay] Error: invoice without amount")
		log.Warnln(errmsg)
		return handler, errors.Create(errors.InvalidAmountError)
	}

	// check user balance first
	balance, err := bot.GetUserBalance(user)
	if err != nil {
		NewMessage(handler.Message(), WithDuration(0, bot))
		errmsg := fmt.Sprintf("[/pay] Error: Could not get user balance: %s", err.Error())
		log.Errorln(errmsg)
		bot.trySendMessage(handler.Sender(), Translate(handler.Ctx, "errorTryLaterMessage"))
		return handler, errors.New(errors.GetBalanceError, err)
	}

	if amount > balance {
		NewMessage(handler.Message(), WithDuration(0, bot))
		bot.trySendMessage(handler.Sender(), fmt.Sprintf(Translate(handler.Ctx, "insufficientFundsMessage"), balance, amount))
		return handler, errors.Create(errors.InvalidSyntaxError)
	}
	// send warning that the invoice might fail due to missing fee reserve
	if float64(amount) > float64(balance)*0.98 {
		bot.trySendMessage(handler.Sender(), Translate(handler.Ctx, "feeReserveMessage"))
	}

	confirmText := fmt.Sprintf(Translate(handler.Ctx, "confirmPayInvoiceMessage"), amount)
	if len(bolt11.Description) > 0 {
		confirmText = confirmText + fmt.Sprintf(Translate(handler.Ctx, "confirmPayAppendMemo"), str.MarkdownEscape(bolt11.Description))
	}

	log.Infof("[/pay] Invoice entered. User: %s, amount: %d sat.", userStr, amount)

	// object that holds all information about the send payment
	id := fmt.Sprintf("pay:%d-%d-%s", handler.Sender().ID, amount, RandStringRunes(5))

	// // // create inline buttons
	payButton := paymentConfirmationMenu.Data(Translate(handler.Ctx, "payButtonMessage"), "confirm_pay", id)
	cancelButton := paymentConfirmationMenu.Data(Translate(handler.Ctx, "cancelButtonMessage"), "cancel_pay", id)

	paymentConfirmationMenu.Inline(
		paymentConfirmationMenu.Row(
			payButton,
			cancelButton),
	)
	payMessage := bot.trySendMessageEditable(handler.Chat(), confirmText, paymentConfirmationMenu)
	payData := &PayData{
		Base:            storage.New(storage.ID(id)),
		From:            user,
		Invoice:         paymentRequest,
		Amount:          int64(amount),
		Memo:            bolt11.Description,
		Message:         confirmText,
		LanguageCode:    handler.Ctx.Value("publicLanguageCode").(string),
		TelegramMessage: payMessage,
	}
	// add result to persistent struct
	runtime.IgnoreError(payData.Set(payData, bot.Bunt))

	SetUserState(user, bot, lnbits.UserStateConfirmPayment, paymentRequest)
	return handler, nil
}

// confirmPayHandler when user clicked pay on payment confirmation
func (bot *TipBot) confirmPayHandler(handler intercept.Handler) (intercept.Handler, error) {
	tx := &PayData{Base: storage.New(storage.ID(handler.Data()))}
	mutex.LockWithContext(handler.Ctx, tx.ID)
	defer mutex.UnlockWithContext(handler.Ctx, tx.ID)
	sn, err := tx.Get(tx, bot.Bunt)
	// immediatelly set intransaction to block duplicate calls
	if err != nil {
		log.Errorf("[confirmPayHandler] %s", err.Error())
		return handler, err
	}
	payData := sn.(*PayData)

	// onnly the correct user can press
	if payData.From.Telegram.ID != handler.Sender().ID {
		return handler, errors.Create(errors.UnknownError)
	}
	if !payData.Active {
		log.Errorf("[confirmPayHandler] send not active anymore")
		bot.tryEditMessage(handler.Message(), i18n.Translate(payData.LanguageCode, "errorTryLaterMessage"), &tb.ReplyMarkup{})
		bot.tryDeleteMessage(handler.Message())
		return handler, errors.Create(errors.NotActiveError)
	}
	defer payData.Set(payData, bot.Bunt)

	// remove buttons from confirmation message
	// bot.tryEditMessage(handler.Message(), MarkdownEscape(payData.Message), &tb.ReplyMarkup{})

	user := LoadUser(handler.Ctx)
	if user.Wallet == nil {
		bot.tryDeleteMessage(handler.Message())
		return handler, errors.Create(errors.UserNoWalletError)
	}

	invoiceString := payData.Invoice

	// reset state immediately
	ResetUserState(user, bot)

	userStr := GetUserStr(handler.Sender())

	// update button text
	bot.tryEditMessage(
		handler.Message(),
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
		bot.tryEditMessage(handler.Message(), fmt.Sprintf(i18n.Translate(payData.LanguageCode, "invoicePaymentFailedMessage"), err.Error()), &tb.ReplyMarkup{})
		// verbose error message, turned off for now
		// if len(err.Error()) == 0 {
		// 	err = fmt.Errorf(i18n.Translate(payData.LanguageCode, "invoiceUndefinedErrorMessage"))
		// }
		// bot.tryEditMessage(c.Message, fmt.Sprintf(i18n.Translate(payData.LanguageCode, "invoicePaymentFailedMessage"), str.MarkdownEscape(err.Error())), &tb.ReplyMarkup{})
		log.Errorln(errmsg)
		return handler, err
	}
	payData.Hash = invoice.PaymentHash

	// do balance check for keyboard update
	_, err = bot.GetUserBalance(user)
	if err != nil {
		errmsg := fmt.Sprintf("could not get balance of user %s", userStr)
		log.Errorln(errmsg)
	}

	if handler.Message().Private() {
		// if the command was invoked in private chat
		// the edit below was cool, but we need to pop up the keyboard again
		// bot.tryEditMessage(c.Message, i18n.Translate(payData.LanguageCode, "invoicePaidMessage"), &tb.ReplyMarkup{})
		bot.tryDeleteMessage(handler.Message())
		bot.trySendMessage(handler.Sender(), i18n.Translate(payData.LanguageCode, "invoicePaidMessage"))
	} else {
		// if the command was invoked in group chat
		bot.trySendMessage(handler.Sender(), i18n.Translate(payData.LanguageCode, "invoicePaidMessage"))
		bot.tryEditMessage(handler.Message(), fmt.Sprintf(i18n.Translate(payData.LanguageCode, "invoicePublicPaidMessage"), userStr), &tb.ReplyMarkup{})
	}
	log.Infof("[⚡️ pay] User %s paid invoice %s (%d sat)", userStr, payData.ID, payData.Amount)
	return handler, nil
}

// cancelPaymentHandler invoked when user clicked cancel on payment confirmation
func (bot *TipBot) cancelPaymentHandler(handler intercept.Handler) (intercept.Handler, error) {
	// reset state immediately
	user := LoadUser(handler.Ctx)
	ResetUserState(user, bot)
	tx := &PayData{Base: storage.New(storage.ID(handler.Data()))}
	mutex.LockWithContext(handler.Ctx, tx.ID)
	defer mutex.UnlockWithContext(handler.Ctx, tx.ID)
	// immediatelly set intransaction to block duplicate calls
	sn, err := tx.Get(tx, bot.Bunt)
	if err != nil {
		log.Errorf("[cancelPaymentHandler] %s", err.Error())
		return handler, err
	}
	payData := sn.(*PayData)
	// onnly the correct user can press
	if payData.From.Telegram.ID != handler.Callback().Sender.ID {
		return handler, errors.Create(errors.UnknownError)
	}
	// delete and send instead of edit for the keyboard to pop up after sending
	bot.tryDeleteMessage(handler.Message())
	bot.trySendMessage(handler.Message().Chat, i18n.Translate(payData.LanguageCode, "paymentCancelledMessage"))
	// bot.tryEditMessage(c.Message, i18n.Translate(payData.LanguageCode, "paymentCancelledMessage"), &tb.ReplyMarkup{})
	return handler, payData.Inactivate(payData, bot.Bunt)

}
