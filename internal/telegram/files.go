package telegram

import (
	"context"
	"github.com/LightningTipBot/LightningTipBot/internal/runtime"
	tb "gopkg.in/lightningtipbot/telebot.v2"
)

func (bot *TipBot) fileHandler(ctx context.Context, m *tb.Message) {
	if m.Chat.Type != tb.ChatPrivate {
		return
	}
	user := LoadUser(ctx)
	if c := stateCallbackMessage[user.StateKey]; c != nil {
		// found handler for this state
		// now looking for user state reset ticker
		ticker := runtime.GetTicker(user.ID)
		if !ticker.Started {
			ticker.Do(func() {
				ResetUserState(user, bot)
				// removing ticker asap done
				runtime.RemoveTicker(user.ID)
			})
		} else {
			ticker.ResetChan <- struct{}{}
		}

		c(ctx, m)
		return
	}
}
