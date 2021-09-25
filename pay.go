package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/LightningTipBot/LightningTipBot/internal/lnbits"
	"github.com/LightningTipBot/LightningTipBot/internal/runtime"
	decodepay "github.com/fiatjaf/ln-decodepay"
	tb "gopkg.in/tucnak/telebot.v2"
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
	From          *lnbits.User `json:"from"`
	ID            string       `json:"id"`
	Invoice       string       `json:"invoice"`
	Hash          string       `json:"hash"`
	Proof         string       `json:"proof"`
	Memo          string       `json:"memo"`
	Message       string       `json:"message"`
	Amount        int64        `json:"amount"`
	InTransaction bool         `json:"intransaction"`
	Active        bool         `json:"active"`
	LanguageCode  string       `json:"languagecode"`
}

func NewPay() *PayData {
	payData := &PayData{
		Active:        true,
		InTransaction: false,
	}
	return payData
}

func (msg PayData) Key() string {
	return msg.ID
}

func (bot *TipBot) LockPay(tx *PayData) error {
	// immediatelly set intransaction to block duplicate calls
	tx.InTransaction = true
	err := bot.bunt.Set(tx)
	if err != nil {
		return err
	}
	return nil
}

func (bot *TipBot) ReleasePay(tx *PayData) error {
	// immediatelly set intransaction to block duplicate calls
	tx.InTransaction = false
	err := bot.bunt.Set(tx)
	if err != nil {
		return err
	}
	return nil
}

func (bot *TipBot) InactivatePay(tx *PayData) error {
	tx.Active = false
	err := bot.bunt.Set(tx)
	if err != nil {
		return err
	}
	return nil
}

func (bot *TipBot) getPay(c *tb.Callback) (*PayData, error) {
	payData := NewPay()
	payData.ID = c.Data

	err := bot.bunt.Get(payData)

	// to avoid race conditions, we block the call if there is
	// already an active transaction by loop until InTransaction is false
	ticker := time.NewTicker(time.Second * 10)

	for payData.InTransaction {
		select {
		case <-ticker.C:
			return nil, fmt.Errorf("pay timeout")
		default:
			log.Infoln("[pay] in transaction")
			time.Sleep(time.Duration(500) * time.Millisecond)
			err = bot.bunt.Get(payData)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("could not get payData")
	}

	return payData, nil

}

// payHandler invoked on "/pay lnbc..." command
func (bot TipBot) payHandler(ctx context.Context, m *tb.Message) {
	// check and print all commands
	bot.anyTextHandler(ctx, m)
	user := LoadUser(ctx)
	if user.Wallet == nil {
		return
	}
	if len(strings.Split(m.Text, " ")) < 2 {
		NewMessage(m, WithDuration(0, bot.telegram))
		bot.trySendMessage(m.Sender, helpPayInvoiceUsage(ctx, ""))
		return
	}
	userStr := GetUserStr(m.Sender)
	paymentRequest, err := getArgumentFromCommand(m.Text, 1)
	if err != nil {
		NewMessage(m, WithDuration(0, bot.telegram))
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
	amount := int(bolt11.MSatoshi / 1000)

	if amount <= 0 {
		bot.trySendMessage(m.Sender, Translate(ctx, "invoiceNoAmountMessage"))
		errmsg := fmt.Sprint("[/pay] Error: invoice without amount")
		log.Errorln(errmsg)
		return
	}

	// check user balance first
	balance, err := bot.GetUserBalance(user)
	if err != nil {
		NewMessage(m, WithDuration(0, bot.telegram))
		errmsg := fmt.Sprintf("[/pay] Error: Could not get user balance: %s", err)
		log.Errorln(errmsg)
		return
	}
	if amount > balance {
		NewMessage(m, WithDuration(0, bot.telegram))
		bot.trySendMessage(m.Sender, fmt.Sprintf(Translate(ctx, "insufficientFundsMessage"), balance, amount))
		return
	}
	// send warning that the invoice might fail due to missing fee reserve
	if float64(amount) > float64(balance)*0.99 {
		bot.trySendMessage(m.Sender, Translate(ctx, "feeReserveMessage"))
	}

	confirmText := fmt.Sprintf(Translate(ctx, "confirmPayInvoiceMessage"), amount)
	if len(bolt11.Description) > 0 {
		confirmText = confirmText + fmt.Sprintf(Translate(ctx, "confirmPayAppendMemo"), MarkdownEscape(bolt11.Description))
	}

	log.Printf("[/pay] User: %s, amount: %d sat.", userStr, amount)

	// object that holds all information about the send payment
	id := fmt.Sprintf("pay-%d-%d-%s", m.Sender.ID, amount, RandStringRunes(5))
	payData := PayData{
		From:          user,
		Invoice:       paymentRequest,
		Active:        true,
		InTransaction: false,
		ID:            id,
		Amount:        int64(amount),
		Memo:          bolt11.Description,
		Message:       confirmText,
		LanguageCode:  ctx.Value("publicLanguageCode").(string),
	}
	// add result to persistent struct
	runtime.IgnoreError(bot.bunt.Set(payData))

	SetUserState(user, bot, lnbits.UserStateConfirmPayment, paymentRequest)

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
	bot.trySendMessage(m.Chat, confirmText, paymentConfirmationMenu)
}

// confirmPayHandler when user clicked pay on payment confirmation
func (bot TipBot) confirmPayHandler(ctx context.Context, c *tb.Callback) {
	payData, err := bot.getPay(c)
	if err != nil {
		log.Errorf("[acceptSendHandler] %s", err)
		return
	}
	// onnly the correct user can press
	if payData.From.Telegram.ID != c.Sender.ID {
		return
	}
	// immediatelly set intransaction to block duplicate calls
	err = bot.LockPay(payData)
	if err != nil {
		log.Errorf("[acceptSendHandler] %s", err)
		bot.tryDeleteMessage(c.Message)
		return
	}
	if !payData.Active {
		log.Errorf("[acceptSendHandler] send not active anymore")
		bot.tryDeleteMessage(c.Message)
		return
	}
	defer bot.ReleasePay(payData)

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
	// pay invoice
	invoice, err := user.Wallet.Pay(lnbits.PaymentParams{Out: true, Bolt11: invoiceString}, bot.client)
	if err != nil {
		errmsg := fmt.Sprintf("[/pay] Could not pay invoice of %s: %s", userStr, err)
		if len(err.Error()) == 0 {
			err = fmt.Errorf(bot.Translate(payData.LanguageCode, "invoiceUndefinedErrorMessage"))
		}
		// bot.trySendMessage(c.Sender, fmt.Sprintf(invoicePaymentFailedMessage, err))
		bot.tryEditMessage(c.Message, fmt.Sprintf(bot.Translate(payData.LanguageCode, "invoicePaymentFailedMessage"), err), &tb.ReplyMarkup{})
		log.Errorln(errmsg)
		return
	}
	payData.Hash = invoice.PaymentHash
	payData.InTransaction = false

	if c.Message.Private() {
		// if the command was invoked in private chat
		bot.tryEditMessage(c.Message, bot.Translate(payData.LanguageCode, "invoicePaidMessage"), &tb.ReplyMarkup{})
	} else {
		// if the command was invoked in group chat
		bot.trySendMessage(c.Sender, bot.Translate(payData.LanguageCode, "invoicePaidMessage"))
		bot.tryEditMessage(c.Message, fmt.Sprintf(bot.Translate(payData.LanguageCode, "invoicePublicPaidMessage"), userStr), &tb.ReplyMarkup{})
	}
	log.Printf("[pay] User %s paid invoice %d (%d sat)", userStr, payData.ID, payData.Amount)
	return
}

// cancelPaymentHandler invoked when user clicked cancel on payment confirmation
func (bot TipBot) cancelPaymentHandler(ctx context.Context, c *tb.Callback) {
	// reset state immediately
	user := LoadUser(ctx)

	ResetUserState(user, bot)
	payData, err := bot.getPay(c)
	if err != nil {
		log.Errorf("[acceptSendHandler] %s", err)
		return
	}
	// onnly the correct user can press
	if payData.From.Telegram.ID != c.Sender.ID {
		return
	}
	bot.tryEditMessage(c.Message, bot.Translate(payData.LanguageCode, "paymentCancelledMessage"), &tb.ReplyMarkup{})
	payData.InTransaction = false
	bot.InactivatePay(payData)
}
