package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/LightningTipBot/LightningTipBot/internal/lnbits"
	lnurl2 "github.com/fiatjaf/go-lnurl"
	log "github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
	tb "gopkg.in/tucnak/telebot.v2"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
)

func (bot TipBot) lnurlPayHandler(m *tb.Message) {
	_, params, err := HandleLNURL(m.Text)
	if err != nil {
		return
	}
	var payParams lnurl2.LNURLPayResponse1
	switch params.(type) {
	case lnurl2.LNURLPayResponse1:
		payParams = params.(lnurl2.LNURLPayResponse1)
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
	user.StateKey = lnbits.UserStateConfirmPayment

	bot.telegram.Send(m.Sender, fmt.Sprintf("reply with amount"), tb.ForceReply)

}

func HandleLNURL(rawlnurl string) (string, lnurl2.LNURLParams, error) {
	var err error
	var rawurl string

	if name, domain, ok := lnurl2.ParseInternetIdentifier(rawlnurl); ok {
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
		lnurl, ok := lnurl2.FindLNURLInText(rawlnurl)
		if !ok {
			return "", nil,
				errors.New("invalid bech32-encoded lnurl: " + rawlnurl)
		}
		rawurl, err = lnurl2.LNURLDecode(lnurl)
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
		value, err := lnurl2.HandleAuth(rawurl, parsed, query)
		return rawurl, value, err
	case "withdrawRequest":
		if value, ok := lnurl2.HandleFastWithdraw(query); ok {
			return rawurl, value, nil
		}
	}
	proxyUrl, err := url.Parse("proxy:port")
	if err != nil {
		return "", nil, err
	}
	client := http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyUrl)}}
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
		return rawurl, nil, lnurl2.LNURLErrorResponse{
			URL:    parsed,
			Reason: j.Get("reason").String(),
			Status: "ERROR",
		}
	}

	switch j.Get("tag").String() {
	case "withdrawRequest":
		value, err := lnurl2.HandleWithdraw(j)
		return rawurl, value, err
	case "payRequest":
		value, err := lnurl2.HandlePay(j)
		return rawurl, value, err
	case "channelRequest":
		value, err := lnurl2.HandleChannel(j)
		return rawurl, value, err
	default:
		return rawurl, nil, errors.New("unknown response tag " + j.String())
	}
}
