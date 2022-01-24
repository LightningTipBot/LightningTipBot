package telegram

import (
	"bytes"
	"context"
	"fmt"
	"github.com/LightningTipBot/LightningTipBot/internal/errors"
	"image"
	"image/jpeg"
	"strings"

	"github.com/LightningTipBot/LightningTipBot/pkg/lightning"
	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/qrcode"
	log "github.com/sirupsen/logrus"
	tb "gopkg.in/lightningtipbot/telebot.v2"
)

// TryRecognizeInvoiceFromQrCode will try to read an invoice string from a qr code and invoke the payment handler.
func TryRecognizeQrCode(img image.Image) (*gozxing.Result, error) {
	// check for qr code
	bmp, _ := gozxing.NewBinaryBitmapFromImage(img)
	// decode image
	qrReader := qrcode.NewQRCodeReader()
	result, err := qrReader.Decode(bmp, nil)
	if err != nil {
		return nil, err
	}
	payload := strings.ToLower(result.String())
	if lightning.IsInvoice(payload) || lightning.IsLnurl(payload) {
		// create payment command payload
		// invoke payment confirmation handler
		return result, nil
	}
	return nil, fmt.Errorf("no codes found")
}

// photoHandler is the handler function for every photo from a private chat that the bot receives
func (bot *TipBot) photoHandler(ctx context.Context, m *tb.Message) (context.Context, error) {
	if m.Chat.Type != tb.ChatPrivate {
		return ctx, errors.Create(errors.NoPrivateChatError)
	}
	if m.Photo == nil {
		return ctx, errors.Create(errors.NoPhotoError)
	}
	user := LoadUser(ctx)
	if c := stateCallbackMessage[user.StateKey]; c != nil {
		ctx, err := c(ctx, m)
		ResetUserState(user, bot)
		return ctx, err
	}

	// get file reader closer from Telegram api
	reader, err := bot.Telegram.GetFile(m.Photo.MediaFile())
	if err != nil {
		log.Errorf("[photoHandler] getfile error: %v\n", err.Error())
		return ctx, err
	}
	// decode to jpeg image
	img, err := jpeg.Decode(reader)
	if err != nil {
		log.Errorf("[photoHandler] image.Decode error: %v\n", err.Error())
		return ctx, err
	}
	data, err := TryRecognizeQrCode(img)
	if err != nil {
		log.Errorf("[photoHandler] tryRecognizeQrCodes error: %v\n", err.Error())
		bot.trySendMessage(m.Sender, Translate(ctx, "photoQrNotRecognizedMessage"))
		return ctx, err
	}

	bot.trySendMessage(m.Sender, fmt.Sprintf(Translate(ctx, "photoQrRecognizedMessage"), data.String()))
	// invoke payment handler
	if lightning.IsInvoice(data.String()) {
		m.Text = fmt.Sprintf("/pay %s", data.String())
		return bot.payHandler(ctx, m)
	} else if lightning.IsLnurl(data.String()) {
		m.Text = fmt.Sprintf("/lnurl %s", data.String())
		return bot.lnurlHandler(ctx, m)
	}
	return ctx, nil
}

var BotProfilePicture []byte

func (bot *TipBot) downloadProfilePicture(user *tb.User) []byte {
	photo, err := bot.Telegram.ProfilePhotosOf(user)
	if err != nil {
		log.Errorf("[downloadMyProfilePicture] %v", err)
		return nil
	}
	if len(photo) == 0 {
		log.Error("[downloadMyProfilePicture] could not download profile picture")
		return nil
	}
	buf := new(bytes.Buffer)
	reader, err := bot.Telegram.GetFile(&photo[0].File)
	if err != nil {
		log.Errorf("[downloadMyProfilePicture] %v", err)
		return nil
	}
	img, err := jpeg.Decode(reader)

	if err != nil {
		log.Errorf("[downloadMyProfilePicture] %v", err)
		return nil
	}
	err = jpeg.Encode(buf, img, nil)
	return buf.Bytes()
}

func (bot *TipBot) downloadMyProfilePicture() {
	BotProfilePicture = bot.downloadProfilePicture(bot.Telegram.Me)
}
