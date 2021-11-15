package telegram

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/LightningTipBot/LightningTipBot/internal"

	"github.com/LightningTipBot/LightningTipBot/internal/lnbits"
	lnurl "github.com/fiatjaf/go-lnurl"
	log "github.com/sirupsen/logrus"
	"github.com/skip2/go-qrcode"
	tb "gopkg.in/tucnak/telebot.v2"
)

func (bot *TipBot) GetHttpClient() (*http.Client, error) {
	client := http.Client{}
	if internal.Configuration.Bot.HttpProxy != "" {
		proxyUrl, err := url.Parse(internal.Configuration.Bot.HttpProxy)
		if err != nil {
			log.Errorln(err)
			return nil, err
		}
		client.Transport = &http.Transport{Proxy: http.ProxyURL(proxyUrl)}
	}
	return &client, nil
}
func (bot TipBot) cancelLnUrlHandler(c *tb.Callback) {
}

// lnurlHandler is invoked on /lnurl command
func (bot *TipBot) lnurlHandler(ctx context.Context, m *tb.Message) {
	// commands:
	// /lnurl
	// /lnurl <LNURL>
	// or /lnurl <amount> <LNURL>
	if m.Chat.Type != tb.ChatPrivate {
		return
	}
	log.Infof("[lnurlHandler] %s", m.Text)
	user := LoadUser(ctx)
	if user.Wallet == nil {
		return
	}

	// if only /lnurl is entered, show the lnurl of the user
	if m.Text == "/lnurl" {
		bot.lnurlReceiveHandler(ctx, m)
		return
	}
	statusMsg := bot.trySendMessage(m.Sender, Translate(ctx, "lnurlResolvingUrlMessage"))

	var lnurlSplit string
	split := strings.Split(m.Text, " ")
	if _, err := decodeAmountFromCommand(m.Text); err == nil {
		// command is /lnurl 123 <LNURL> [memo]
		if len(split) > 2 {
			lnurlSplit = split[2]
		}
	} else if len(split) > 1 {
		lnurlSplit = split[1]
	} else {
		bot.tryEditMessage(statusMsg, fmt.Sprintf(Translate(ctx, "errorReasonMessage"), "Could not parse command."))
		log.Warnln("[/lnurl] Could not parse command.")
		return
	}

	// assume payment
	// HandleLNURL by fiatjaf/go-lnurl
	_, params, err := lnurl.HandleLNURL(lnurlSplit)
	if err != nil {
		bot.tryEditMessage(statusMsg, fmt.Sprintf(Translate(ctx, "errorReasonMessage"), err.Error()))
		log.Warnf("[HandleLNURL] Error: %s", err.Error())
		return
	}
	switch params.(type) {
	case lnurl.LNURLPayParams:
		payParams := LnurlPayState{LNURLPayParams: params.(lnurl.LNURLPayParams)}
		log.Infof("[LNURL-p] %s", payParams.LNURLPayParams.Callback)
		bot.tryDeleteMessage(statusMsg)
		bot.lnurlPayHandler(ctx, m, payParams)
		return
	case lnurl.LNURLWithdrawResponse:
		withdrawParams := LnurlWithdrawState{LNURLWithdrawResponse: params.(lnurl.LNURLWithdrawResponse)}
		log.Infof("[LNURL-w] %s", withdrawParams.LNURLWithdrawResponse.Callback)
		bot.tryDeleteMessage(statusMsg)
		bot.lnurlWithdrawHandler(ctx, m, withdrawParams)
	default:
		err := fmt.Errorf("Invalid LNURL type.")
		log.Warnln(err)
		bot.tryEditMessage(statusMsg, fmt.Sprintf(Translate(ctx, "errorReasonMessage"), err.Error()))
		// bot.trySendMessage(m.Sender, err.Error())
		return
	}
}

func (bot *TipBot) UserGetLightningAddress(user *lnbits.User) (string, error) {
	if len(user.Telegram.Username) > 0 {
		return fmt.Sprintf("%s@%s", strings.ToLower(user.Telegram.Username), strings.ToLower(internal.Configuration.Bot.LNURLHostUrl.Hostname())), nil
	} else {
		lnaddr, err := bot.UserGetAnonLightningAddress(user)
		return lnaddr, err
	}
}

func (bot *TipBot) UserGetAnonLightningAddress(user *lnbits.User) (string, error) {
	return fmt.Sprintf("%s@%s", fmt.Sprint(user.AnonID), strings.ToLower(internal.Configuration.Bot.LNURLHostUrl.Hostname())), nil
}

func UserGetLNURL(user *lnbits.User) (string, error) {
	// before: we used the username for the LNURL
	// name := strings.ToLower(strings.ToLower(user.Telegram.Username))
	// if len(name) == 0 {
	// 	name = fmt.Sprint(user.AnonID)
	// 	// return "", fmt.Errorf("user has no username.")
	// }
	// now: use only the anon ID as LNURL
	name := fmt.Sprint(user.AnonID)
	callback := fmt.Sprintf("%s/.well-known/lnurlp/%s", internal.Configuration.Bot.LNURLHostName, name)
	log.Debugf("[lnurlReceiveHandler] %s's LNURL: %s", GetUserStr(user.Telegram), callback)

	lnurlEncode, err := lnurl.LNURLEncode(callback)
	if err != nil {
		return "", err
	}
	return lnurlEncode, nil
}

// lnurlReceiveHandler outputs the LNURL of the user
func (bot TipBot) lnurlReceiveHandler(ctx context.Context, m *tb.Message) {
	fromUser := LoadUser(ctx)
	lnurlEncode, err := UserGetLNURL(fromUser)
	if err != nil {
		errmsg := fmt.Sprintf("[userLnurlHandler] Failed to get LNURL: %s", err)
		log.Errorln(errmsg)
		bot.Telegram.Send(m.Sender, Translate(ctx, "lnurlNoUsernameMessage"))
	}
	// create qr code
	qr, err := qrcode.Encode(lnurlEncode, qrcode.Medium, 256)
	if err != nil {
		errmsg := fmt.Sprintf("[userLnurlHandler] Failed to create QR code for LNURL: %s", err)
		log.Errorln(errmsg)
		return
	}

	bot.trySendMessage(m.Sender, Translate(ctx, "lnurlReceiveInfoText"))
	// send the lnurl data to user
	bot.trySendMessage(m.Sender, &tb.Photo{File: tb.File{FileReader: bytes.NewReader(qr)}, Caption: fmt.Sprintf("`%s`", lnurlEncode)})
}
