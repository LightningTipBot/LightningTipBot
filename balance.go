package main

import (
	"context"
	"fmt"

	log "github.com/sirupsen/logrus"

	tb "gopkg.in/tucnak/telebot.v2"
)

const (
	balanceMessage      = "👑 *Your balance:* %d sat"
	balanceErrorMessage = "🚫 Error fetching your balance. Please try again later."
)

func (bot TipBot) balanceHandler(ctx context.Context, m *tb.Message) {
	// check and print all commands
	bot.anyTextHandler(ctx, m)
	// reply only in private message
	if m.Chat.Type != tb.ChatPrivate {
		// delete message
		NewMessage(m, WithDuration(0, bot.telegram))
	}
	// first check whether the user is initialized
	user := LoadUser(ctx)
	if user.Wallet == nil {
		return
	}

	if !user.Initialized {
		bot.startHandler(m)
		return
	}

	usrStr := GetUserStr(m.Sender)
	balance, err := bot.GetUserBalance(user)
	if err != nil {
		log.Errorf("[/balance] Error fetching %s's balance: %s", usrStr, err)
		bot.trySendMessage(m.Sender, balanceErrorMessage)
		return
	}

	log.Infof("[/balance] %s's balance: %d sat\n", usrStr, balance)
	bot.trySendMessage(m.Sender, fmt.Sprintf(balanceMessage, balance))
	return
}
