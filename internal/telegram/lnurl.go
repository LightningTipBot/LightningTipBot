package telegram

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"github.com/LightningTipBot/LightningTipBot/internal"

	"github.com/LightningTipBot/LightningTipBot/internal/lnbits"
	lnurl "github.com/fiatjaf/go-lnurl"
	log "github.com/sirupsen/logrus"
	"github.com/skip2/go-qrcode"
	"github.com/tidwall/gjson"
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

	// assume payment
	// HandleLNURL by fiatjaf/go-lnurl
	statusMsg := bot.trySendMessage(m.Sender, Translate(ctx, "lnurlResolvingUrlMessage"))
	_, params, err := bot.HandleLNURL(m.Text)
	if err != nil {
		bot.tryEditMessage(statusMsg, fmt.Sprintf(Translate(ctx, "lnurlPaymentFailed"), "could not resolve LNURL."))
		log.Errorln(err)
		return
	}
	switch params.(type) {
	case lnurl.LNURLPayResponse1:
		payParams := LnurlPayState{LNURLPayResponse1: params.(lnurl.LNURLPayResponse1)}
		log.Infof("[lnurlHandler] %s", payParams.LNURLPayResponse1.Callback)
		bot.tryDeleteMessage(statusMsg)
		bot.lnurlPayHandler(ctx, m, payParams)
		return
	default:
		err := fmt.Errorf("invalid LNURL type.")
		log.Errorln(err)
		bot.tryEditMessage(statusMsg, fmt.Sprintf(Translate(ctx, "lnurlPaymentFailed"), err))
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

// // lnurlEnterAmountHandler is invoked if the user didn't deliver an amount for the lnurl payment
// func (bot *TipBot) lnurlEnterAmountHandler(ctx context.Context, m *tb.Message) {
// 	user := LoadUser(ctx)
// 	if user.Wallet == nil {
// 		return
// 	}

// 	if user.StateKey == lnbits.UserStateLNURLEnterAmount || user.StateKey == lnbits.UserEnterAmount {
// 		a, err := strconv.Atoi(m.Text)
// 		if err != nil {
// 			log.Errorln(err)
// 			bot.trySendMessage(m.Sender, Translate(ctx, "lnurlInvalidAmountMessage"))
// 			ResetUserState(user, bot)
// 			return
// 		}
// 		amount := int64(a)
// 		var stateResponse LnurlPayState
// 		err = json.Unmarshal([]byte(user.StateData), &stateResponse)
// 		if err != nil {
// 			log.Errorln(err)
// 			ResetUserState(user, bot)
// 			return
// 		}
// 		// amount not in allowed range from LNURL
// 		if amount > (stateResponse.LNURLPayResponse1.MaxSendable/1000) || amount < (stateResponse.LNURLPayResponse1.MinSendable/1000) {
// 			err = fmt.Errorf("amount not in range")
// 			log.Errorln(err)
// 			bot.trySendMessage(m.Sender, fmt.Sprintf(Translate(ctx, "lnurlInvalidAmountRangeMessage"), stateResponse.LNURLPayResponse1.MinSendable/1000, stateResponse.LNURLPayResponse1.MaxSendable/1000))
// 			ResetUserState(user, bot)
// 			return
// 		}
// 		stateResponse.Amount = a
// 		state, err := json.Marshal(stateResponse)
// 		if err != nil {
// 			log.Errorln(err)
// 			ResetUserState(user, bot)
// 			return
// 		}
// 		SetUserState(user, bot, lnbits.UserStateConfirmLNURLPay, string(state))
// 		bot.lnurlPayHandler(ctx, m)
// 	}
// }

// // LnurlStateResponse saves the state of the user for an LNURL payment
// type LnurlStateResponse struct {
// 	lnurl.LNURLPayResponse1
// 	Amount  int    `json:"amount"`
// 	Comment string `json:"comment"`
// }

// // lnurlPayHandler is invoked when the user has delivered an amount and is ready to pay
// func (bot *TipBot) lnurlPayHandler(ctx context.Context, c *tb.Message) {
// 	msg := bot.trySendMessage(c.Sender, Translate(ctx, "lnurlGettingUserMessage"))

// 	user := LoadUser(ctx)
// 	if user.Wallet == nil {
// 		return
// 	}

// 	if user.StateKey == lnbits.UserStateConfirmLNURLPay {
// 		client, err := getHttpClient()
// 		if err != nil {
// 			log.Errorln(err)
// 			// bot.trySendMessage(c.Sender, err.Error())
// 			bot.tryEditMessage(msg, fmt.Sprintf(Translate(ctx, "lnurlPaymentFailed"), err))
// 			return
// 		}
// 		var stateResponse LnurlStateResponse
// 		err = json.Unmarshal([]byte(user.StateData), &stateResponse)
// 		if err != nil {
// 			log.Errorln(err)
// 			// bot.trySendMessage(c.Sender, err.Error())
// 			bot.tryEditMessage(msg, fmt.Sprintf(Translate(ctx, "lnurlPaymentFailed"), err))
// 			return
// 		}
// 		callbackUrl, err := url.Parse(stateResponse.Callback)
// 		if err != nil {
// 			log.Errorln(err)
// 			// bot.trySendMessage(c.Sender, err.Error())
// 			bot.tryEditMessage(msg, fmt.Sprintf(Translate(ctx, "lnurlPaymentFailed"), err))
// 			return
// 		}
// 		qs := callbackUrl.Query()
// 		// add amount to query string
// 		qs.Set("amount", strconv.Itoa(stateResponse.Amount*1000))
// 		// add comment to query string
// 		if len(stateResponse.Comment) > 0 {
// 			qs.Set("comment", stateResponse.Comment)
// 		}

// 		callbackUrl.RawQuery = qs.Encode()

// 		res, err := client.Get(callbackUrl.String())
// 		if err != nil {
// 			log.Errorln(err)
// 			// bot.trySendMessage(c.Sender, err.Error())
// 			bot.tryEditMessage(msg, fmt.Sprintf(Translate(ctx, "lnurlPaymentFailed"), err))
// 			return
// 		}
// 		var response2 lnurl.LNURLPayResponse2
// 		body, err := ioutil.ReadAll(res.Body)
// 		if err != nil {
// 			log.Errorln(err)
// 			// bot.trySendMessage(c.Sender, err.Error())
// 			bot.tryEditMessage(msg, fmt.Sprintf(Translate(ctx, "lnurlPaymentFailed"), err))
// 			return
// 		}
// 		json.Unmarshal(body, &response2)

// 		if len(response2.PR) < 1 {
// 			error_reason := "Could not receive invoice."
// 			if len(response2.Reason) > 0 {
// 				error_reason = response2.Reason
// 			}
// 			bot.tryEditMessage(msg, fmt.Sprintf(Translate(ctx, "lnurlPaymentFailed"), error_reason))
// 			return
// 		}
// 		bot.Telegram.Delete(msg)
// 		c.Text = fmt.Sprintf("/pay %s", response2.PR)
// 		bot.payHandler(ctx, c)
// 	}
// }

// from https://github.com/fiatjaf/go-lnurl
func (bot *TipBot) HandleLNURL(rawlnurl string) (string, lnurl.LNURLParams, error) {
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
	} else {
		foundUrl, ok := lnurl.FindLNURLInText(rawlnurl)
		if !ok {
			return "", nil,
				errors.New("invalid bech32-encoded lnurl: " + rawlnurl)
		}
		rawurl, err = lnurl.LNURLDecode(foundUrl)
		if err != nil {
			return "", nil, err
		}
	}

	parsed, err := url.Parse(rawurl)
	if err != nil {
		return rawurl, nil, err
	}

	// query := parsed.Query()

	// switch query.Get("tag") {
	// case "login":
	// 	value, err := lnurl.HandleAuth(rawurl, parsed, query)
	// 	return rawurl, value, err
	// case "withdrawRequest":
	// 	if value, ok := lnurl.HandleFastWithdraw(query); ok {
	// 		return rawurl, value, nil
	// 	}
	// }
	client, err := bot.GetHttpClient()
	if err != nil {
		return "", nil, err
	}
	resp, err := client.Get(rawurl)
	if err != nil {
		return rawurl, nil, err
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
	// case "withdrawRequest":
	// 	value, err := lnurl.HandleWithdraw(j)
	// 	return rawurl, value, err
	case "payRequest":
		value, err := lnurl.HandlePay(j)
		return rawurl, value, err
	// case "channelRequest":
	// 	value, err := lnurl.HandleChannel(j)
	// 	return rawurl, value, err
	default:
		return rawurl, nil, errors.New("unknown response tag " + j.String())
	}
}
