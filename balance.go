package main

import (
	"context"
	"fmt"
	"github.com/LightningTipBot/LightningTipBot/internal/lnbits"

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
	// try to load user form context
	fromUser := ctx.Value("user").(*lnbits.User)
	if fromUser == nil {
		log.Errorf("[/balance] Error: %s", fmt.Sprintf("user not found"))
		return
	}
	if !fromUser.Initialized {
		bot.startHandler(m)
		return
	}

	usrStr := GetUserStr(m.Sender)
	balance, err := bot.GetUserBalance(fromUser)
	if err != nil {
		log.Errorf("[/balance] Error fetching %s's balance: %s", usrStr, err)
		bot.trySendMessage(m.Sender, balanceErrorMessage)
		return
	}

	log.Infof("[/balance] %s's balance: %d sat\n", usrStr, balance)
	bot.trySendMessage(m.Sender, fmt.Sprintf(balanceMessage, balance))
	return
}
