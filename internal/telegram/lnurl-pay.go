package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"
	"strconv"

	"github.com/LightningTipBot/LightningTipBot/internal/lnbits"
	"github.com/LightningTipBot/LightningTipBot/internal/storage/transaction"
	lnurl "github.com/fiatjaf/go-lnurl"
	log "github.com/sirupsen/logrus"
	tb "gopkg.in/tucnak/telebot.v2"
)

// LnurlPayState saves the state of the user for an LNURL payment
type LnurlPayState struct {
	*transaction.Base
	From              *lnbits.User            `json:"from"`
	LNURLPayResponse1 lnurl.LNURLPayResponse1 `json:"LNURLPayResponse1"`
	LNURLPayResponse2 lnurl.LNURLPayResponse2 `json:"LNURLPayResponse2"`
	Amount            int                     `json:"amount"`
	Comment           string                  `json:"comment"`
	LanguageCode      string                  `json:"languagecode"`
}

// lnurlPayHandler is invoked when the user has delivered an amount and is ready to pay
func (bot TipBot) lnurlPayHandler(ctx context.Context, c *tb.Message) {
	msg := bot.trySendMessage(c.Sender, Translate(ctx, "lnurlGettingUserMessage"))

	user := LoadUser(ctx)
	if user.Wallet == nil {
		return
	}

	if user.StateKey == lnbits.UserStateConfirmLNURLPay {
		client, err := getHttpClient()
		if err != nil {
			log.Errorln(err)
			// bot.trySendMessage(c.Sender, err.Error())
			bot.tryEditMessage(msg, fmt.Sprintf(Translate(ctx, "lnurlPaymentFailed"), err))
			return
		}
		var stateResponse LnurlPayState
		err = json.Unmarshal([]byte(user.StateData), &stateResponse)
		if err != nil {
			log.Errorln(err)
			// bot.trySendMessage(c.Sender, err.Error())
			bot.tryEditMessage(msg, fmt.Sprintf(Translate(ctx, "lnurlPaymentFailed"), err))
			return
		}
		callbackUrl, err := url.Parse(stateResponse.LNURLPayResponse1.Callback)
		if err != nil {
			log.Errorln(err)
			// bot.trySendMessage(c.Sender, err.Error())
			bot.tryEditMessage(msg, fmt.Sprintf(Translate(ctx, "lnurlPaymentFailed"), err))
			return
		}
		qs := callbackUrl.Query()
		// add amount to query string
		qs.Set("amount", strconv.Itoa(stateResponse.Amount*1000))
		// add comment to query string
		if len(stateResponse.Comment) > 0 {
			qs.Set("comment", stateResponse.Comment)
		}

		callbackUrl.RawQuery = qs.Encode()

		res, err := client.Get(callbackUrl.String())
		if err != nil {
			log.Errorln(err)
			// bot.trySendMessage(c.Sender, err.Error())
			bot.tryEditMessage(msg, fmt.Sprintf(Translate(ctx, "lnurlPaymentFailed"), err))
			return
		}
		var response2 lnurl.LNURLPayResponse2
		body, err := ioutil.ReadAll(res.Body)
		if err != nil {
			log.Errorln(err)
			// bot.trySendMessage(c.Sender, err.Error())
			bot.tryEditMessage(msg, fmt.Sprintf(Translate(ctx, "lnurlPaymentFailed"), err))
			return
		}
		json.Unmarshal(body, &response2)

		if len(response2.PR) < 1 {
			bot.tryEditMessage(msg, fmt.Sprintf(Translate(ctx, "lnurlPaymentFailed"), "could not receive invoice (wrong address?)."))
			return
		}
		bot.Telegram.Delete(msg)
		c.Text = fmt.Sprintf("/pay %s", response2.PR)
		bot.payHandler(ctx, c)
	}
}
