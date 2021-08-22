package main

import (
	"fmt"

	log "github.com/sirupsen/logrus"

	tb "gopkg.in/tucnak/telebot.v2"
)

func (bot TipBot) balanceHandler(m *tb.Message) {
	log.Infof("[%s:%d %s:%d] %s", m.Chat.Title, m.Chat.ID, GetUserStr(m.Sender), m.Sender.ID, m.Text)
	// reply only in private message
	if m.Chat.Type != tb.ChatPrivate {
		// delete message
		NewMessage(m).Dispose(0, bot.telegram)
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
		bot.telegram.Send(m.Sender, "🚫 Error fetching your balance. Please try again later.")
		return
	}

	log.Infof("[/balance] %s's balance: %d sat\n", usrStr, balance)
	bot.telegram.Send(m.Sender, fmt.Sprintf("👑 *Your balance:* %d sat", balance))
	return
}
