package main

import (
	"fmt"

	log "github.com/sirupsen/logrus"

	tb "gopkg.in/tucnak/telebot.v2"
)

func (bot TipBot) balanceHandler(m *tb.Message) {
	// check and print all commands
	bot.anyTextHandler(m)
	// reply only in private message
	if m.Chat.Type != tb.ChatPrivate {
		// delete message
		NewMessage(m, WithDuration(0, bot.telegram))
	}
	// first check whether the user is initialized
	fromUser, err := GetUser(m.Sender, bot)
	if err != nil {
		log.Errorf("[/balance] Error: %s", err)
		return
	}
	if !fromUser.Initialized {
		bot.startHandler(m)
		return
	}

	usrStr := GetUserStr(m.Sender)
	balance, err := bot.GetUserBalance(m.Sender)
	if err != nil {
		log.Errorf("[/balance] Error fetching %s's balance: %s", usrStr, err)
		bot.trySendMessage(m.Sender, "🚫 Error fetching your balance. Please try again later.")
		return
	}

	log.Infof("[/balance] %s's balance: %d sat\n", usrStr, balance)
	bot.trySendMessage(m.Sender, fmt.Sprintf("👑 *Your balance:* %d sat", balance))
	return
}
