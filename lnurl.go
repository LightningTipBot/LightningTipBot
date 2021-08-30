package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/LightningTipBot/LightningTipBot/internal/lnbits"
	lnurl "github.com/fiatjaf/go-lnurl"
	log "github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
	tb "gopkg.in/tucnak/telebot.v2"
)

var (
	lnUrlConfirmMenu = &tb.ReplyMarkup{}
	cancelLnUrl      = lnUrlConfirmMenu.Data("🚫 Cancel", "cancel_lnurl")
	confirmLnUrl     = lnUrlConfirmMenu.Data("✅ Pay", "confirm_lnurl")
)

func (bot TipBot) lnurlPayHandler(m *tb.Message) {
	_, params, err := HandleLNURL(m.Text)
	if err != nil {
		bot.telegram.Send(m.Sender, "invalid lnurl")
		log.Println(err)
		return
	}
	var payParams LnurlStateResponse
	switch params.(type) {
	case lnurl.LNURLPayResponse1:
		payParams = LnurlStateResponse{LNURLPayResponse1: params.(lnurl.LNURLPayResponse1)}
		fmt.Println(payParams.Callback)
	default:
		err := fmt.Errorf("invalid lnurl type")
		log.Println(err)
		bot.telegram.Send(m.Sender, err.Error())
		return
	}
	user, err := GetUser(m.Sender, bot)
	if err != nil {
		log.Println(err)
		bot.telegram.Send(m.Sender, err.Error())
		return
	}
	paramsJson, err := json.Marshal(payParams)
	if err != nil {
		log.Println(err)
		bot.telegram.Send(m.Sender, err.Error())
		return
	}

	user.StateData = string(paramsJson)
	user.StateKey = lnbits.UserStateLNURLEnterAmount
	err = UpdateUserRecord(user, bot)
	if err != nil {
		log.Println(err)
		bot.telegram.Send(m.Sender, err.Error())
		return
	}
	bot.telegram.Send(m.Sender, fmt.Sprintf("reply with amount"), tb.ForceReply)

}

func (bot TipBot) confirmLnurlPayHandler(m *tb.Message) {
	user, err := GetUser(m.Sender, bot)
	if err != nil {
		log.Println(err)
		bot.telegram.Send(m.Sender, err.Error())
		return
	}
	if user.StateKey == lnbits.UserStateLNURLEnterAmount {
		a, err := strconv.Atoi(m.Text)
		if err != nil {
			log.Println(err)
			bot.telegram.Send(m.Sender, err.Error())
			return
		}
		amount := int64(a)
		var stateResponse LnurlStateResponse
		err = json.Unmarshal([]byte(user.StateData), &stateResponse)
		if err != nil {
			log.Println(err)
			bot.telegram.Send(m.Sender, err.Error())
			return
		}
		if amount > (stateResponse.MaxSendable/1000) || amount < (stateResponse.MinSendable/1000) {
			err = fmt.Errorf("amount not in range")
			log.Println(err)
			bot.telegram.Send(m.Sender, err.Error())
			return
		}
		stateResponse.Amount = a
		user.StateKey = lnbits.UserStateConfirmLNURLPay
		state, err := json.Marshal(stateResponse)
		if err != nil {
			log.Println(err)
			bot.telegram.Send(m.Sender, err.Error())
			return
		}
		user.StateData = string(state)
		err = UpdateUserRecord(user, bot)
		if err != nil {
			log.Println(err)
			bot.telegram.Send(m.Sender, err.Error())
			return
		}
		lnUrlConfirmMenu.Inline(lnUrlConfirmMenu.Row(confirmLnUrl, cancelLnUrl))

		bot.telegram.Send(m.Sender, "plz confirm", lnUrlConfirmMenu)
	}
}

type LnurlStateResponse struct {
	lnurl.LNURLPayResponse1
	Amount int `json:"amount"`
}

func (bot TipBot) payLnUrlHandler(c *tb.Callback) {
	user, err := GetUser(c.Sender, bot)
	if err != nil {
		log.Println(err)
		bot.telegram.Send(c.Sender, err.Error())
		return
	}
	if user.StateKey == lnbits.UserStateConfirmLNURLPay {
		client, err := getHttpClient()
		if err != nil {
			log.Println(err)
			bot.telegram.Send(c.Sender, err.Error())
			return
		}
		var stateResponse LnurlStateResponse
		err = json.Unmarshal([]byte(user.StateData), &stateResponse)
		if err != nil {
			log.Println(err)
			bot.telegram.Send(c.Sender, err.Error())
			return
		}
		callbackUrl, err := url.Parse(stateResponse.Callback)
		if err != nil {
			log.Println(err)
			bot.telegram.Send(c.Sender, err.Error())
			return
		}
		qs := callbackUrl.Query()
		qs.Set("amount", strconv.Itoa(stateResponse.Amount*1000))
		callbackUrl.RawQuery = qs.Encode()

		res, err := client.Get(callbackUrl.String())
		if err != nil {
			log.Println(err)
			bot.telegram.Send(c.Sender, err.Error())
			return
		}
		var response2 lnurl.LNURLPayResponse2
		body, err := ioutil.ReadAll(res.Body)
		if err != nil {
			log.Println(err)
			bot.telegram.Send(c.Sender, err.Error())
			return
		}
		json.Unmarshal(body, &response2)

		bot.telegram.Send(c.Sender, response2.PR)
	}
}

func getHttpClient() (*http.Client, error) {
	client := http.Client{}
	if Configuration.HttpProxy != "" {
		proxyUrl, err := url.Parse(Configuration.HttpProxy)
		if err != nil {
			log.Println(err)
			return nil, err
		}
		client.Transport = &http.Transport{Proxy: http.ProxyURL(proxyUrl)}
	}
	return &client, nil
}
func (bot TipBot) cancelLnUrlHandler(c *tb.Callback) {

}

// from https://github.com/fiatjaf/go-lnurl
func HandleLNURL(rawlnurl string) (string, lnurl.LNURLParams, error) {
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
	client, err := getHttpClient()
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
	case "withdrawRequest":
		value, err := lnurl.HandleWithdraw(j)
		return rawurl, value, err
	case "payRequest":
		value, err := lnurl.HandlePay(j)
		return rawurl, value, err
	case "channelRequest":
		value, err := lnurl.HandleChannel(j)
		return rawurl, value, err
	default:
		return rawurl, nil, errors.New("unknown response tag " + j.String())
	}
}

func (bot *TipBot) sendToLightningAddress(m *tb.Message, address string, amount int) error {
	split := strings.Split(address, "@")
	if len(split) != 2 {
		return fmt.Errorf("lightning address format wrong")
	}
	host := strings.ToLower(split[1])
	name := strings.ToLower(split[0])

	// convert address scheme into LNURL Bech32 format
	callback := fmt.Sprintf("https://%s/.well-known/lnurlp/%s", host, name)

	log.Infof("[sendToLightningAddress] %s: callback: %s", GetUserStr(m.Sender), callback)

	lnurl, err := lnurl.LNURLEncode(callback)
	if err != nil {
		return err
	}
	m.Text = fmt.Sprintf("/lnurl %s", lnurl)
	bot.lnurlPayHandler(m)
	return nil
}
