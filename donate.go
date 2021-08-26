package main

import (
	"fmt"
	"github.com/LightningTipBot/LightningTipBot/internal/lnbits"
	log "github.com/sirupsen/logrus"
	tb "gopkg.in/tucnak/telebot.v2"
	"io/ioutil"
	"net/http"
)

func (bot TipBot) donationHandler(m *tb.Message) {
	amount, err := decodeAmountFromCommand(m.Text)
	if err != nil {
		return
	}
	if amount <= 0 {
		return
	}
	resp, err := http.Get(fmt.Sprintf("https://relay.lnscan.com/donate/%d", amount))
	if err != nil {
		log.Fatalln(err)
	}
	//We Read the response body on the line below.
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatalln(err)
	}
	user, err := GetUser(m.Sender, bot)
	if err != nil {
		return
	}
	user.Wallet.Pay(lnbits.PaymentParams{Out: true, Bolt11: string(body)}, *user.Wallet)

}
