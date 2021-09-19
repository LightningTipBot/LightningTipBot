package main

import (
	"context"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/LightningTipBot/LightningTipBot/internal/lnbits"
	"github.com/LightningTipBot/LightningTipBot/pkg/lightning"
	tb "gopkg.in/tucnak/telebot.v2"
)

const (
	initWalletMessage = "You don't have a wallet yet. Enter */start*"
)

func (bot TipBot) anyTextHandler(ctx context.Context, m *tb.Message) {
	log.Infof("[%s:%d %s:%d] %s", m.Chat.Title, m.Chat.ID, GetUserStr(m.Sender), m.Sender.ID, m.Text)
	if m.Chat.Type != tb.ChatPrivate {
		return
	}

	// check if user is in database, if not, initialize wallet
	user := LoadUser(ctx)
	if user.Wallet == nil || !user.Initialized {
		bot.startHandler(m)
		return
	}

	// could be an invoice
	anyText := strings.ToLower(m.Text)
	if lightning.IsInvoice(anyText) {
		m.Text = "/pay " + anyText
		bot.confirmPaymentHandler(ctx, m)
		return
	}
	if lightning.IsLnurl(anyText) {
		m.Text = "/lnurl " + anyText
		bot.lnurlHandler(ctx, m)
		return
	}

	// could be a LNURL
	// var lnurlregex = regexp.MustCompile(`.*?((lnurl)([0-9]{1,}[a-z0-9]+){1})`)
	if user.StateKey == lnbits.UserStateLNURLEnterAmount {
		bot.lnurlEnterAmountHandler(ctx, m)
	}

}
