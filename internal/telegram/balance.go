package telegram

import (
	"context"
	"fmt"

	log "github.com/sirupsen/logrus"

	tb "gopkg.in/lightningtipbot/telebot.v2"
)

func (bot TipBot) balanceHandler(ctx context.Context, m *tb.Message) {
	// check and print all commands
	bot.anyTextHandler(ctx, m)
	// reply only in private message
	if m.Chat.Type != tb.ChatPrivate {
		// delete message
		bot.tryDeleteMessage(m)
	}
	// first check whether the user is initialized
	user := LoadUser(ctx)
	if user.Wallet == nil {
		return
	}

	if !user.Initialized {
		bot.startHandler(ctx, m)
		return
	}

	usrStr := GetUserStr(m.Sender)
	balance, err := bot.GetUserBalance(user)
	if err != nil {
		log.Errorf("[/balance] Error fetching %s's balance: %s", usrStr, err)
		bot.trySendMessage(m.Sender, Translate(ctx, "balanceErrorMessage"))
		return
	}

	log.Infof("[/balance] %s's balance: %d sat\n", usrStr, balance)
	bot.trySendMessage(m.Sender, fmt.Sprintf(Translate(ctx, "balanceMessage"), balance))
	return
}
