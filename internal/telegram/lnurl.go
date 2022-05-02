package telegram

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/url"
	"strings"

	"github.com/LightningTipBot/LightningTipBot/internal/network"
	"github.com/LightningTipBot/LightningTipBot/internal/telegram/intercept"

	"github.com/LightningTipBot/LightningTipBot/internal/errors"

	"github.com/LightningTipBot/LightningTipBot/internal"
	"github.com/tidwall/gjson"

	"github.com/LightningTipBot/LightningTipBot/internal/lnbits"
	lnurl "github.com/fiatjaf/go-lnurl"
	log "github.com/sirupsen/logrus"
	"github.com/skip2/go-qrcode"
	tb "gopkg.in/lightningtipbot/telebot.v3"
)

// func (bot *TipBot) GetHttpClient() (*http.Client, error) {
// 	client := http.Client{
// 		Timeout: 10 * time.Second,
// 	}
// 	if internal.Configuration.Bot.HttpProxy != "" {
// 		proxyUrl, err := url.Parse(internal.Configuration.Bot.HttpProxy)
// 		if err != nil {
// 			log.Errorln(err)
// 			return nil, err
// 		}
// 		client.Transport = &http.Transport{Proxy: http.ProxyURL(proxyUrl)}
// 	}
// 	return &client, nil
// }
// func (bot *TipBot) GetSocksClient() (*http.Client, error) {
// 	client := http.Client{
// 		Timeout: 10 * time.Second,
// 	}
// 	if internal.Configuration.Bot.SocksProxy != "" {
// 		proxyURL, _ := url.Parse(internal.Configuration.Bot.SocksProxy)
// 		specialTransport := &http.Transport{}
// 		specialTransport.Proxy = http.ProxyURL(proxyURL)
// 		d, err := proxy.SOCKS5("tcp", internal.Configuration.Bot.SocksProxy, nil, &net.Dialer{
// 			Timeout:   20 * time.Second,
// 			Deadline:  time.Now().Add(time.Second * 10),
// 			KeepAlive: -1,
// 		})
// 		if err != nil {
// 			log.Errorln(err)
// 			return &client, nil
// 		}
// 		specialTransport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
// 			return d.Dial(network, addr)
// 		}
// 		client.Transport = specialTransport
// 	}
// 	return &client, nil
// }

func (bot TipBot) cancelLnUrlHandler(c *tb.Callback) {
}

// lnurlHandler is invoked on /lnurl command
func (bot *TipBot) lnurlHandler(ctx intercept.Context) (intercept.Context, error) {
	// commands:
	// /lnurl
	// /lnurl <LNURL>
	// or /lnurl <amount> <LNURL>
	m := ctx.Message()
	if m.Chat.Type != tb.ChatPrivate {
		return ctx, errors.Create(errors.NoPrivateChatError)
	}
	log.Infof("[lnurlHandler] %s", m.Text)
	user := LoadUser(ctx)
	if user.Wallet == nil {
		return ctx, errors.Create(errors.UserNoWalletError)
	}

	// if only /lnurl is entered, show the lnurl of the user
	if m.Text == "/lnurl" {
		return bot.lnurlReceiveHandler(ctx)
	}
	statusMsg := bot.trySendMessageEditable(m.Sender, Translate(ctx, "lnurlResolvingUrlMessage"))

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
		return ctx, errors.Create(errors.InvalidSyntaxError)
	}

	// get rid of the URI prefix
	lnurlSplit = strings.TrimPrefix(lnurlSplit, "lightning:")

	log.Debugf("[lnurlHandler] lnurlSplit: %s", lnurlSplit)
	// HandleLNURL by fiatjaf/go-lnurl
	_, params, err := bot.HandleLNURL(lnurlSplit)
	if err != nil {
		bot.tryEditMessage(statusMsg, fmt.Sprintf(Translate(ctx, "errorReasonMessage"), "LNURL error."))
		// bot.tryEditMessage(statusMsg, fmt.Sprintf(Translate(ctx, "errorReasonMessage"), err.Error()))
		log.Warnf("[HandleLNURL] Error: %s", err.Error())
		return ctx, err
	}
	switch params.(type) {
	case lnurl.LNURLAuthParams:
		authParams := &LnurlAuthState{LNURLAuthParams: params.(lnurl.LNURLAuthParams)}
		log.Infof("[LNURL-auth] %s", authParams.LNURLAuthParams.Callback)
		bot.tryDeleteMessage(statusMsg)
		ctx.Context, err = bot.lnurlAuthHandler(ctx, m, authParams)
		return ctx, err

	case lnurl.LNURLPayParams:
		payParams := &LnurlPayState{LNURLPayParams: params.(lnurl.LNURLPayParams)}
		log.Infof("[LNURL-p] %s", payParams.LNURLPayParams.Callback)
		bot.tryDeleteMessage(statusMsg)

		// display the metadata image from the first LNURL-p response
		if len(payParams.LNURLPayParams.Metadata.Image.Bytes) > 0 {
			bot.trySendMessage(m.Sender, &tb.Photo{
				File:    tb.File{FileReader: bytes.NewReader(payParams.LNURLPayParams.Metadata.Image.Bytes)},
				Caption: fmt.Sprintf("%s", payParams.LNURLPayParams.Metadata.Description)})
		} else if len(payParams.LNURLPayParams.Metadata.Description) > 0 {
			// display the metadata text from the first LNURL-p response
			// if there was no photo in the last step
			bot.trySendMessage(m.Sender, fmt.Sprintf("`%s`", payParams.LNURLPayParams.Metadata.Description))
		}
		// ask whether to make payment
		bot.lnurlPayHandler(ctx, payParams)

	case lnurl.LNURLWithdrawResponse:
		withdrawParams := &LnurlWithdrawState{LNURLWithdrawResponse: params.(lnurl.LNURLWithdrawResponse)}
		log.Infof("[LNURL-w] %s", withdrawParams.LNURLWithdrawResponse.Callback)
		bot.tryDeleteMessage(statusMsg)
		bot.lnurlWithdrawHandler(ctx, withdrawParams)
	default:
		if err == nil {
			err = fmt.Errorf("invalid LNURL type")
		}
		log.Warnln(err)
		bot.tryEditMessage(statusMsg, fmt.Sprintf(Translate(ctx, "errorReasonMessage"), err.Error()))
		// bot.trySendMessage(m.Sender, err.Error())
		return ctx, err
	}
	return ctx, nil
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
	return fmt.Sprintf("%s@%s", fmt.Sprint(user.AnonIDSha256), strings.ToLower(internal.Configuration.Bot.LNURLHostUrl.Hostname())), nil
}

func UserGetLNURL(user *lnbits.User) (string, error) {
	name := fmt.Sprint(user.UUID)
	callback := fmt.Sprintf("%s/.well-known/lnurlp/%s", internal.Configuration.Bot.LNURLHostName, name)
	log.Debugf("[lnurlReceiveHandler] %s's LNURL: %s", GetUserStr(user.Telegram), callback)

	lnurlEncode, err := lnurl.LNURLEncode(callback)
	if err != nil {
		return "", err
	}
	return lnurlEncode, nil
}

// lnurlReceiveHandler outputs the LNURL of the user
func (bot TipBot) lnurlReceiveHandler(ctx intercept.Context) (intercept.Context, error) {
	m := ctx.Message()
	fromUser := LoadUser(ctx)
	lnurlEncode, err := UserGetLNURL(fromUser)
	if err != nil {
		errmsg := fmt.Sprintf("[userLnurlHandler] Failed to get LNURL: %s", err.Error())
		log.Errorln(errmsg)
		bot.trySendMessage(m.Sender, Translate(ctx, "lnurlNoUsernameMessage"))
		return ctx, err
	}
	// create qr code
	qr, err := qrcode.Encode(lnurlEncode, qrcode.Medium, 256)
	if err != nil {
		errmsg := fmt.Sprintf("[userLnurlHandler] Failed to create QR code for LNURL: %s", err.Error())
		log.Errorln(errmsg)
		return ctx, err
	}

	bot.trySendMessage(m.Sender, Translate(ctx, "lnurlReceiveInfoText"))
	// send the lnurl QR code
	bot.trySendMessage(m.Sender, &tb.Photo{File: tb.File{FileReader: bytes.NewReader(qr)}, Caption: fmt.Sprintf("`%s`", lnurlEncode)})
	return ctx, nil
}

// fiatjaf/go-lnurl 1.8.4 with proxy
func (bot TipBot) HandleLNURL(rawlnurl string) (string, lnurl.LNURLParams, error) {
	var err error
	var rawurl string

	if name, domain, ok := lnurl.ParseInternetIdentifier(rawlnurl); ok {
		isOnion := strings.Index(domain, ".onion") == len(domain)-6
		rawurl = domain + "/.well-known/lnurlp/" + name
		if isOnion {
			rawurl = "http://" + rawurl
		} else {
			rawurl = "https://" + rawurl
		}
	} else if strings.HasPrefix(rawlnurl, "http") {
		rawurl = rawlnurl
	} else if strings.HasPrefix(rawlnurl, "lnurlp://") ||
		strings.HasPrefix(rawlnurl, "lnurlw://") ||
		strings.HasPrefix(rawlnurl, "lnurla://") ||
		strings.HasPrefix(rawlnurl, "keyauth://") {

		scheme := "https:"
		if strings.Contains(rawurl, ".onion/") || strings.HasSuffix(rawurl, ".onion") {
			scheme = "http:"
		}
		location := strings.SplitN(rawlnurl, ":", 2)[1]
		rawurl = scheme + location
	} else {
		lnurl_str, ok := lnurl.FindLNURLInText(rawlnurl)
		if !ok {
			return "", nil,
				fmt.Errorf("invalid bech32-encoded lnurl: " + rawlnurl)
		}
		rawurl, err = lnurl.LNURLDecode(lnurl_str)
		if err != nil {
			return "", nil, err
		}
	}

	parsed, err := url.Parse(rawurl)
	if err != nil {
		return rawurl, nil, err
	}

	query := parsed.Query()

	switch query.Get("tag") {
	case "login":
		value, err := lnurl.HandleAuth(rawurl, parsed, query)
		return rawurl, value, err
	case "withdrawRequest":
		if value, ok := lnurl.HandleFastWithdraw(query); ok {
			return rawurl, value, nil
		}
	}

	// // original withouth proxy
	// resp, err := http.Get(rawurl)
	// if err != nil {
	// 	return rawurl, nil, err
	// }

	client, err := network.GetHttpClient()
	if err != nil {
		return "", nil, err
	}
	resp, err := client.Get(rawurl)
	if err != nil {
		return rawurl, nil, err
	}
	if resp.StatusCode >= 300 {
		return rawurl, nil, fmt.Errorf("HTTP error: " + resp.Status)
	}

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return rawurl, nil, err
	}

	j := gjson.ParseBytes(b)
	if j.Get("status").String() == "ERROR" {
		return rawurl, nil, lnurl.LNURLErrorResponse{
			URL:    parsed,
			Reason: j.Get("reason").String(),
			Status: "ERROR",
		}
	}

	switch j.Get("tag").String() {
	case "withdrawRequest":
		value, err := lnurl.HandleWithdraw(b)
		return rawurl, value, err
	case "payRequest":
		value, err := lnurl.HandlePay(b)
		return rawurl, value, err
	// case "channelRequest":
	// 	value, err := lnurl.HandleChannel(b)
	// 	return rawurl, value, err
	default:
		return rawurl, nil, fmt.Errorf("Unkown LNURL response.")
	}
}
