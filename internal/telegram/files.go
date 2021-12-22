package telegram

import (
	"context"
	"github.com/orcaman/concurrent-map"
	"time"

	tb "gopkg.in/lightningtipbot/telebot.v2"
)

func init() {
	fileStateResetTicker = cmap.New()
}

var fileStateResetTicker cmap.ConcurrentMap

func (bot *TipBot) fileHandler(ctx context.Context, m *tb.Message) {
	if m.Chat.Type != tb.ChatPrivate {
		return
	}
	user := LoadUser(ctx)
	if c := stateCallbackMessage[user.StateKey]; c != nil {
		if t, ok := fileStateResetTicker.Get(user.ID); ok {
			t.(UserStateTicker).ticketResetChan <- struct{}{}
		} else {
			ticker := UserStateTicker{
				ticketResetChan: make(chan struct{}, 1),
				user:            user,
				bot:             bot,
				ticker:          time.NewTicker(tickerCoolDown)}
			fileStateResetTicker.Set(user.ID, ticker)
			go func() {
				ticker.Do()
				fileStateResetTicker.Remove(ticker.user.ID)
			}()
		}
		c(ctx, m)
		return
	}
}
