package main

import (
	"strings"

	"github.com/LightningTipBot/LightningTipBot/internal/lnbits"

	log "github.com/sirupsen/logrus"

	"github.com/LightningTipBot/LightningTipBot/pkg/lightning"
	tb "gopkg.in/tucnak/telebot.v2"
)

const (
	initWalletMessage = "You don't have a wallet yet. Enter */start*"
)

func (bot TipBot) anyTextHandler(m *tb.Message) {
	log.Infof("[%s:%d %s:%d] %s", m.Chat.Title, m.Chat.ID, GetUserStr(m.Sender), m.Sender.ID, m.Text)
	if m.Chat.Type != tb.ChatPrivate {
		return
	}

	// check if user is in database, if not, initialize wallet
	user, exists := bot.UserExists(m.Sender)
	if !exists {
		bot.startHandler(m)
		return
	}

	// could be an invoice
	anyText := strings.ToLower(m.Text)
	if lightning.IsInvoice(anyText) {
		m.Text = "/pay " + anyText
		bot.confirmPaymentHandler(m)
		return
	}

	// could be a LNURL
	// var lnurlregex = regexp.MustCompile(`.*?((lnurl)([0-9]{1,}[a-z0-9]+){1})`)

	if user.StateKey == lnbits.UserStateLNURLEnterAmount {
		bot.confirmLnurlPayHandler(m)
	}

}
