package telegram

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/LightningTipBot/LightningTipBot/internal"

	log "github.com/sirupsen/logrus"

	"github.com/LightningTipBot/LightningTipBot/internal/i18n"
	"github.com/LightningTipBot/LightningTipBot/internal/lnbits"
	"github.com/LightningTipBot/LightningTipBot/internal/runtime"
	"github.com/skip2/go-qrcode"
	tb "gopkg.in/lightningtipbot/telebot.v2"
)

type InvoiceEventCallback map[int]func(*InvocieEvent)

var InvoiceCallback InvoiceEventCallback

func initInvoiceEventCallbacks(bot TipBot) {
	InvoiceCallback = InvoiceEventCallback{
		InvoiceCallbackGeneric:       bot.triggerInvoiceEvent,
		InvoiceCallbackInlineReceive: bot.inlineReceiveEvent,
	}
}

type InvoiceEventKey int

const (
	InvoiceCallbackGeneric = iota + 1
	InvoiceCallbackInlineReceive
)

type InvocieEvent struct {
	PaymentHash    string       `json:"payment_hash"`
	PaymentRequest string       `json:"payment_request"`
	Amount         int64        `json:"amount"`
	Memo           string       `json:"memo"`
	User           *lnbits.User `json:"user"`
	Message        *tb.Message  `json:"message"`
	InvoiceMessage *tb.Message  `json:"invoice_message"`
	LanguageCode   string       `json:"languagecode"`
	Callback       int          `json:"func"`
	CallbackData   string       `json:"callbackdata"`
}

func (invoiceEvent InvocieEvent) Key() string {
	return fmt.Sprintf("invoice:%s", invoiceEvent.PaymentHash)
}

func helpInvoiceUsage(ctx context.Context, errormsg string) string {
	if len(errormsg) > 0 {
		return fmt.Sprintf(Translate(ctx, "invoiceHelpText"), fmt.Sprintf("%s", errormsg))
	} else {
		return fmt.Sprintf(Translate(ctx, "invoiceHelpText"), "")
	}
}

func (bot *TipBot) invoiceHandler(ctx context.Context, m *tb.Message) {
	// check and print all commands
	bot.anyTextHandler(ctx, m)
	user := LoadUser(ctx)
	if user.Wallet == nil {
		return
	}
	userStr := GetUserStr(user.Telegram)
	if m.Chat.Type != tb.ChatPrivate {
		// delete message
		bot.tryDeleteMessage(m)
		return
	}
	// if no amount is in the command, ask for it
	amount, err := decodeAmountFromCommand(m.Text)
	if (err != nil || amount < 1) && m.Chat.Type == tb.ChatPrivate {
		// // no amount was entered, set user state and ask fo""r amount
		bot.askForAmount(ctx, "", "CreateInvoiceState", 0, 0, m.Text)
		return
	}

	// check for memo in command
	memo := "Powered by @LightningTipBot"
	if len(strings.Split(m.Text, " ")) > 2 {
		memo = GetMemoFromCommand(m.Text, 2)
		tag := " (@LightningTipBot)"
		memoMaxLen := 159 - len(tag)
		if len(memo) > memoMaxLen {
			memo = memo[:memoMaxLen-len(tag)]
		}
		memo = memo + tag
	}

	creatingMsg := bot.trySendMessage(m.Sender, Translate(ctx, "lnurlGettingUserMessage"))
	log.Infof("[/invoice] Creating invoice for %s of %d sat.", userStr, amount)
	// generate invoice
	// invoice, err := user.Wallet.Invoice(
	// 	lnbits.InvoiceParams{
	// 		Out:     false,
	// 		Amount:  int64(amount),
	// 		Memo:    memo,
	// 		Webhook: internal.Configuration.Lnbits.WebhookServer},
	// 	bot.Client)
	invoice, err := bot.createInvoiceEvent(ctx, user, amount, memo, InvoiceCallbackGeneric, "")
	if err != nil {
		errmsg := fmt.Sprintf("[/invoice] Could not create an invoice: %s", err)
		bot.tryEditMessage(creatingMsg, Translate(ctx, "errorTryLaterMessage"))
		log.Errorln(errmsg)
		return
	}

	// create qr code
	qr, err := qrcode.Encode(invoice.PaymentRequest, qrcode.Medium, 256)
	if err != nil {
		errmsg := fmt.Sprintf("[/invoice] Failed to create QR code for invoice: %s", err)
		bot.tryEditMessage(creatingMsg, Translate(ctx, "errorTryLaterMessage"))
		log.Errorln(errmsg)
		return
	}

	bot.tryDeleteMessage(creatingMsg)
	// send the invoice data to user
	bot.trySendMessage(m.Sender, &tb.Photo{File: tb.File{FileReader: bytes.NewReader(qr)}, Caption: fmt.Sprintf("`%s`", invoice.PaymentRequest)})
	log.Printf("[/invoice] Incvoice created. User: %s, amount: %d sat.", userStr, amount)
	return
}

func (bot *TipBot) createInvoiceEvent(ctx context.Context, user *lnbits.User, amount int64, memo string, callback int, callbackData string) (InvocieEvent, error) {
	invoice, err := user.Wallet.Invoice(
		lnbits.InvoiceParams{
			Out:     false,
			Amount:  int64(amount),
			Memo:    memo,
			Webhook: internal.Configuration.Lnbits.WebhookServer},
		bot.Client)
	if err != nil {
		errmsg := fmt.Sprintf("[/invoice] Could not create an invoice: %s", err)
		log.Errorln(errmsg)
		return InvocieEvent{}, err
	}

	invoiceEvent := InvocieEvent{
		PaymentHash:    invoice.PaymentHash,
		PaymentRequest: invoice.PaymentRequest,
		Amount:         amount,
		Memo:           memo,
		User:           user,
		Callback:       callback,
		CallbackData:   callbackData,
		LanguageCode:   ctx.Value("publicLanguageCode").(string),
	}
	// save invoice struct for later use
	runtime.IgnoreError(bot.Bunt.Set(invoiceEvent))
	return invoiceEvent, nil
}

func (bot *TipBot) triggerInvoiceEvent(invoiceEvent *InvocieEvent) {
	bot.trySendMessage(invoiceEvent.User.Telegram, fmt.Sprintf(i18n.Translate(invoiceEvent.User.Telegram.LanguageCode, "invoiceReceivedMessage"), invoiceEvent.Amount))
}
