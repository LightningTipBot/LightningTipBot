package main

import (
	"context"
	"fmt"
	"image"
	"image/jpeg"
	"strings"

	"github.com/LightningTipBot/LightningTipBot/pkg/lightning"
	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/qrcode"
	log "github.com/sirupsen/logrus"
	tb "gopkg.in/tucnak/telebot.v2"
)

var (
	photoQrNotRecognizedMessage   = "🚫 Could not regocognize a Lightning invoice. Try to center the QR code, crop the photo, or zoom in."
	photoInvoiceRecognizedMessage = "✅ Invoice:\n`%s`"
)

// TryRecognizeInvoiceFromQrCode will try to read an invoice string from a qr code and invoke the payment handler.
func TryRecognizeInvoiceFromQrCode(img image.Image) (*gozxing.Result, error) {
	// check for qr code
	bmp, _ := gozxing.NewBinaryBitmapFromImage(img)
	// decode image
	qrReader := qrcode.NewQRCodeReader()
	result, err := qrReader.Decode(bmp, nil)
	if err != nil {
		return nil, err
	}
	payload := strings.ToLower(result.String())
	if lightning.IsInvoice(payload) {
		// create payment command payload
		// invoke payment confirmation handler
		return result, nil
	}
	return nil, fmt.Errorf("no codes found")
}

// privatePhotoHandler is the handler function for every photo from a private chat that the bot receives
func (bot TipBot) privatePhotoHandler(ctx context.Context, m *tb.Message) {
	if m.Chat.Type != tb.ChatPrivate {
		return
	}
	if m.Photo == nil {
		return
	}
	log.Infof("[%s:%d %s:%d] %s", m.Chat.Title, m.Chat.ID, GetUserStr(m.Sender), m.Sender.ID, "<Photo>")
	// get file reader closer from telegram api
	reader, err := bot.telegram.GetFile(m.Photo.MediaFile())
	if err != nil {
		log.Errorf("Getfile error: %v\n", err)
		return
	}
	// decode to jpeg image
	img, err := jpeg.Decode(reader)
	if err != nil {
		log.Errorf("image.Decode error: %v\n", err)
		return
	}
	data, err := TryRecognizeInvoiceFromQrCode(img)
	if err != nil {
		log.Errorf("tryRecognizeQrCodes error: %v\n", err)
		bot.trySendMessage(m.Sender, photoQrNotRecognizedMessage)
		return
	}

	bot.trySendMessage(m.Sender, fmt.Sprintf(photoInvoiceRecognizedMessage, data.String()))

	// invoke payment handler
	m.Text = fmt.Sprintf("/pay %s", data.String())
	bot.confirmPaymentHandler(ctx, m)
}
