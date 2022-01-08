package telegram

import (
	"context"
	"github.com/LightningTipBot/LightningTipBot/internal/errors"
	"github.com/LightningTipBot/LightningTipBot/internal/runtime"
	tb "gopkg.in/lightningtipbot/telebot.v2"
)

func (bot *TipBot) fileHandler(ctx context.Context, m *tb.Message) (context.Context, error) {
	if m.Chat.Type != tb.ChatPrivate {
		return ctx, errors.Create(errors.NoPrivateChatError)
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
				bot.shopViewDeleteAllStatusMsgs(ctx, user)
				runtime.RemoveTicker(user.ID)
			})
		} else {
			ticker.ResetChan <- struct{}{}
		}

		return c(ctx, m)
	}
	return ctx, errors.Create(errors.NoFileFoundError)
}
