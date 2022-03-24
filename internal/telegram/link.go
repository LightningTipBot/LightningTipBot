package telegram

import (
	"bytes"
	"fmt"
	"github.com/LightningTipBot/LightningTipBot/internal/telegram/intercept"
	"time"

	"github.com/LightningTipBot/LightningTipBot/internal"

	log "github.com/sirupsen/logrus"
	"github.com/skip2/go-qrcode"
	tb "gopkg.in/telebot.v3"
)

func (bot *TipBot) lndhubHandler(handler intercept.Handler) (intercept.Handler, error) {
	m := handler.Message()
	if internal.Configuration.Lnbits.LnbitsPublicUrl == "" {
		bot.trySendMessage(m.Sender, Translate(handler.Ctx, "couldNotLinkMessage"))
		return handler, fmt.Errorf("invalid configuration")
	}
	// check and print all commands
	bot.anyTextHandler(handler)
	// reply only in private message
	if m.Chat.Type != tb.ChatPrivate {
		// delete message
		bot.tryDeleteMessage(m)
	}
	// first check whether the user is initialized
	fromUser := LoadUser(handler.Ctx)
	bot.trySendMessage(m.Sender, Translate(handler.Ctx, "walletConnectMessage"))

	// do not respond to banned users
	if bot.UserIsBanned(fromUser) {
		log.Warnln("[lndhubHandler] user is banned. not responding.")
		return handler, fmt.Errorf("user is banned")
	}

	lndhubUrl := fmt.Sprintf("lndhub://admin:%s@%slndhub/ext/", fromUser.Wallet.Adminkey, internal.Configuration.Lnbits.LnbitsPublicUrl)

	// create qr code
	qr, err := qrcode.Encode(lndhubUrl, qrcode.Medium, 256)
	if err != nil {
		errmsg := fmt.Sprintf("[/invoice] Failed to create QR code for invoice: %s", err.Error())
		log.Errorln(errmsg)
		return handler, err
	}

	// send the link to the user
	linkmsg := bot.trySendMessage(m.Sender, &tb.Photo{File: tb.File{FileReader: bytes.NewReader(qr)}, Caption: fmt.Sprintf("`%s`", lndhubUrl)})

	go func() {
		time.Sleep(time.Second * 60)
		bot.tryDeleteMessage(linkmsg)
		bot.trySendMessage(m.Sender, Translate(handler.Ctx, "linkHiddenMessage"))
	}()
	// auto delete the message
	// NewMessage(linkmsg, WithDuration(time.Second*time.Duration(internal.Configuration.Telegram.MessageDisposeDuration), bot))
	return handler, nil
}
