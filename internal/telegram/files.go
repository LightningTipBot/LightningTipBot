package telegram

import (
	"context"
	"time"

	tb "gopkg.in/lightningtipbot/telebot.v2"
)

func (bot *TipBot) fileHandler(ctx context.Context, m *tb.Message) {
	if m.Chat.Type != tb.ChatPrivate {
		return
	}

	user := LoadUser(ctx)
	if c := stateCallbackMessage[user.StateKey]; c != nil {
		c(ctx, m)
		time.Sleep(time.Second * 10)
		ResetUserState(user, bot)
		return
	}
}
