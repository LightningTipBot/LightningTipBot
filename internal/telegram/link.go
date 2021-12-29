package telegram

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/LightningTipBot/LightningTipBot/internal"

	log "github.com/sirupsen/logrus"
	"github.com/skip2/go-qrcode"
	tb "gopkg.in/lightningtipbot/telebot.v2"
)

func (bot *TipBot) lndhubHandler(ctx context.Context, m *tb.Message) {
	if internal.Configuration.Lnbits.LnbitsPublicUrl == "" {
		bot.trySendMessage(m.Sender, Translate(ctx, "couldNotLinkMessage"))
		return
	}
	// check and print all commands
	bot.anyTextHandler(ctx, m)
	// reply only in private message
	if m.Chat.Type != tb.ChatPrivate {
		// delete message
		bot.tryDeleteMessage(m)
	}
	// first check whether the user is initialized
	fromUser := LoadUser(ctx)
	bot.trySendMessage(m.Sender, Translate(ctx, "walletConnectMessage"))

	// do not respond to banned users
	if strings.HasPrefix(fromUser.Wallet.Adminkey, "banned") || strings.HasPrefix(fromUser.Wallet.Adminkey, "_") {
		return
	}

	lndhubUrl := fmt.Sprintf("lndhub://admin:%s@%slndhub/ext/", fromUser.Wallet.Adminkey, internal.Configuration.Lnbits.LnbitsPublicUrl)

	// create qr code
	qr, err := qrcode.Encode(lndhubUrl, qrcode.Medium, 256)
	if err != nil {
		errmsg := fmt.Sprintf("[/invoice] Failed to create QR code for invoice: %s", err.Error())
		log.Errorln(errmsg)
		return
	}

	// send the link to the user
	linkmsg := bot.trySendMessage(m.Sender, &tb.Photo{File: tb.File{FileReader: bytes.NewReader(qr)}, Caption: fmt.Sprintf("`%s`", lndhubUrl)})

	go func() {
		time.Sleep(time.Second * 60)
		bot.tryDeleteMessage(linkmsg)
		bot.trySendMessage(m.Sender, Translate(ctx, "linkHiddenMessage"))
	}()
	// auto delete the message
	// NewMessage(linkmsg, WithDuration(time.Second*time.Duration(internal.Configuration.Telegram.MessageDisposeDuration), bot))

}
