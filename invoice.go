package main

import (
	"bytes"
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/LightningTipBot/LightningTipBot/internal/lnbits"
	"github.com/skip2/go-qrcode"
	tb "gopkg.in/tucnak/telebot.v2"
)

const (
	invoiceEnterAmountMessage = "Did you enter an amount?"
	invoiceValidAmountMessage = "Did you enter a valid amount?"
	invoiceHelpText           = "📖 Oops, that didn't work. %s\n\n" +
		"*Usage:* `/invoice <amount> [<memo>]`\n" +
		"*Example:* `/invoice 1000 Take this! 💸`"
)

func helpInvoiceUsage(errormsg string) string {
	if len(errormsg) > 0 {
		return fmt.Sprintf(invoiceHelpText, fmt.Sprintf("%s", errormsg))
	} else {
		return fmt.Sprintf(invoiceHelpText, "")
	}
}

// func helpInvoiceUsage(errormsg string) string {
// 	helpstr := "📖 Oops, that didn't work. %s\n\n" +
// 		"*Usage:* `/invoice <amount> [<memo>]`\n" +
// 		"*Example:* `/invoice 1000 Take this! 💸`"
// 	if len(errormsg) > 0 {
// 		helpstr = fmt.Sprintf(helpstr, fmt.Sprintf("%s", errormsg))
// 	} else {
// 		helpstr = fmt.Sprintf(helpstr, "")
// 	}
// 	return helpstr
// }

func (bot TipBot) invoiceHandler(m *tb.Message) {
	log.Infof("[%s:%d %s:%d] %s", m.Chat.Title, m.Chat.ID, GetUserStr(m.Sender), m.Sender.ID, m.Text)
	if m.Chat.Type != tb.ChatPrivate {
		// delete message
		NewMessage(m).Dispose(0, bot.telegram)
		return
	}
	if len(strings.Split(m.Text, " ")) < 2 {
		bot.telegram.Send(m.Sender, helpInvoiceUsage(invoiceEnterAmountMessage))
		return
	}

	user, err := GetUser(m.Sender, bot)
	userStr := GetUserStr(m.Sender)
	amount, err := decodeAmountFromCommand(m.Text)
	if err != nil {
		return
	}
	if amount > 0 {
	} else {
		bot.telegram.Send(m.Sender, helpInvoiceUsage(invoiceValidAmountMessage))
		return
	}

	// check for memo in command
	memo := "Powered by @LightningTipBot"
	if len(strings.Split(m.Text, " ")) > 2 {
		memo = strings.SplitN(m.Text, " ", 3)[2]
		tag := " (@LightningTipBot)"
		memoMaxLen := 159 - len(tag)
		if len(memo) > memoMaxLen {
			memo = memo[:memoMaxLen-len(tag)]
		}
		memo = memo + tag
	}

	log.Infof("[/invoice] Creating invoice for %s of %d sat.", userStr, amount)
	// generate invoice
	invoice, err := user.Wallet.Invoice(
		lnbits.InvoiceParams{
			Out:     false,
			Amount:  int64(amount),
			Memo:    memo,
			Webhook: Configuration.WebhookServer},
		*user.Wallet)
	if err != nil {
		errmsg := fmt.Sprintf("[/invoice] Could not create an invoice: %s", err)
		log.Errorln(errmsg)
		return
	}

	// create qr code
	qr, err := qrcode.Encode(invoice.PaymentRequest, qrcode.Medium, 256)
	if err != nil {
		errmsg := fmt.Sprintf("[/invoice] Failed to create QR code for invoice: %s", err)
		log.Errorln(errmsg)
		return
	}

	// send the invoice data to user
	bot.telegram.Send(m.Sender, &tb.Photo{File: tb.File{FileReader: bytes.NewReader(qr)}, Caption: fmt.Sprintf("`%s`", invoice.PaymentRequest)})
	log.Printf("[/invoice] Incvoice created. User: %s, amount: %d sat.", userStr, amount)
	return
}
