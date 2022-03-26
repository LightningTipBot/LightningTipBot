package telegram

import (
	"github.com/LightningTipBot/LightningTipBot/internal/errors"
	"github.com/LightningTipBot/LightningTipBot/internal/runtime"
	"github.com/LightningTipBot/LightningTipBot/internal/telegram/intercept"
	tb "gopkg.in/telebot.v3"
)

func (bot *TipBot) fileHandler(ctx intercept.Context) (intercept.Context, error) {
	m := ctx.Message()
	if m.Chat.Type != tb.ChatPrivate {
		return ctx, errors.Create(errors.NoPrivateChatError)
	}
	user := LoadUser(ctx)
	if c := stateCallbackMessage[user.StateKey]; c != nil {
		// found ctx for this state
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

		return c(ctx)
	}
	return ctx, errors.Create(errors.NoFileFoundError)
}
