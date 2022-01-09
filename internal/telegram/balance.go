package telegram

import (
	"context"
	"fmt"
	"github.com/LightningTipBot/LightningTipBot/internal/errors"

	log "github.com/sirupsen/logrus"

	tb "gopkg.in/lightningtipbot/telebot.v2"
)

func (bot *TipBot) balanceHandler(ctx context.Context, m *tb.Message) (context.Context, error) {
	// check and print all commands
	if len(m.Text) > 0 {
		bot.anyTextHandler(ctx, m)
	}

	// reply only in private message
	if m.Chat.Type != tb.ChatPrivate {
		// delete message
		bot.tryDeleteMessage(m)
	}
	// first check whether the user is initialized
	user := LoadUser(ctx)
	if user.Wallet == nil {
		return ctx, errors.Create(errors.UserNoWalletError)
	}

	if !user.Initialized {
		return bot.startHandler(ctx, m)
	}

	usrStr := GetUserStr(m.Sender)
	balance, err := bot.GetUserBalance(user)
	if err != nil {
		log.Errorf("[/balance] Error fetching %s's balance: %s", usrStr, err)
		bot.trySendMessage(m.Sender, Translate(ctx, "balanceErrorMessage"))
		return ctx, err
	}

	log.Infof("[/balance] %s's balance: %d sat\n", usrStr, balance)
	bot.trySendMessage(m.Sender, fmt.Sprintf(Translate(ctx, "balanceMessage"), balance))
	return ctx, nil
}
