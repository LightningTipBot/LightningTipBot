package main

import (
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/LightningTipBot/LightningTipBot/pkg/lightning"
	tb "gopkg.in/tucnak/telebot.v2"
)

func (bot TipBot) anyTextHandler(m *tb.Message) {
	log.Infof("[%s:%d %s:%d] %s", m.Chat.Title, m.Chat.ID, GetUserStr(m.Sender), m.Sender.ID, m.Text)
	if m.Chat.Type != tb.ChatPrivate {
		return
	}
	if strings.HasPrefix(m.Text, "/") {
		// check if user is in database, if not, initialize wallet
		if !bot.UserHasWallet(m.Sender) {
			log.Infof("User %s has no wallet, force-initializing", GetUserStr(m.Sender))
			bot.startHandler(m)
			return
		}
	}

	// could be an invoice
	invoiceString := strings.ToLower(m.Text)
	if lightning.IsInvoice(invoiceString) {
		m.Text = "/pay " + invoiceString
		bot.confirmPaymentHandler(m)
		return
	}
}
