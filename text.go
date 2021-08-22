package main

import (
	log "github.com/sirupsen/logrus"
	"strings"

	"github.com/LightningTipBot/LightningTipBot/pkg/lightning"
	tb "gopkg.in/tucnak/telebot.v2"
)

func (bot TipBot) anyTextHandler(m *tb.Message) {
	log.Infof("[%s:%d %s:%d] %s", m.Chat.Title, m.Chat.ID, GetUserStr(m.Sender), m.Sender.ID, m.Text)
	if m.Chat.Type != tb.ChatPrivate {
		return
	}
	// could be an invoice
	invoiceString := strings.ToLower(m.Text)
	if lightning.IsInvoice(invoiceString) {
		m.Text = "/pay " + invoiceString
		bot.confirmPaymentHandler(m)
	}
}
