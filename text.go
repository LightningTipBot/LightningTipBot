package main

import (
	"strings"

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
	if !bot.UserHasWallet(m.Sender) {
		bot.startHandler(m)
		return
	}

	// could be an invoice
	invoiceString := strings.ToLower(m.Text)
	if lightning.IsInvoice(invoiceString) {
		m.Text = "/pay " + invoiceString
		bot.confirmPaymentHandler(m)
		return
	}
}
