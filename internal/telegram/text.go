package telegram

import (
	"context"
	"strings"

	"github.com/LightningTipBot/LightningTipBot/internal/lnbits"
	"github.com/LightningTipBot/LightningTipBot/pkg/lightning"
	tb "gopkg.in/tucnak/telebot.v2"
)

func (bot TipBot) anyTextHandler(ctx context.Context, m *tb.Message) {
	if m.Chat.Type != tb.ChatPrivate {
		return
	}

	// check if user is in Database, if not, initialize wallet
	user := LoadUser(ctx)
	if user.Wallet == nil || !user.Initialized {
		bot.startHandler(ctx, m)
		return
	}

	// could be an invoice
	anyText := strings.ToLower(m.Text)
	if lightning.IsInvoice(anyText) {
		m.Text = "/pay " + anyText
		bot.payHandler(ctx, m)
		return
	}
	if lightning.IsLnurl(anyText) {
		m.Text = "/lnurl " + anyText
		bot.lnurlHandler(ctx, m)
		return
	}

	// could be a LNURL
	// var lnurlregex = regexp.MustCompile(`.*?((lnurl)([0-9]{1,}[a-z0-9]+){1})`)
	if user.StateKey == lnbits.UserStateLNURLEnterAmount || user.StateKey == lnbits.UserEnterAmount {
		bot.enterAmountHandler(ctx, m)
	}

}
