package telegram

import (
	"github.com/LightningTipBot/LightningTipBot/internal/errors"
	"github.com/LightningTipBot/LightningTipBot/internal/runtime"
	"github.com/LightningTipBot/LightningTipBot/internal/telegram/intercept"
	tb "gopkg.in/telebot.v3"
)

func (bot *TipBot) fileHandler(handler intercept.Handler) (intercept.Handler, error) {
	m := handler.Message()
	if m.Chat.Type != tb.ChatPrivate {
		return handler, errors.Create(errors.NoPrivateChatError)
	}
	user := LoadUser(handler.Ctx)
	if c := stateCallbackMessage[user.StateKey]; c != nil {
		// found handler for this state
		// now looking for user state reset ticker
		ticker := runtime.GetTicker(user.ID)
		if !ticker.Started {
			ticker.Do(func() {
				ResetUserState(user, bot)
				// removing ticker asap done
				//bot.shopViewDeleteAllStatusMsgs(handler.Ctx, user)
				runtime.RemoveTicker(user.ID)
			})
		} else {
			ticker.ResetChan <- struct{}{}
		}

		return c(handler)
	}
	return handler, errors.Create(errors.NoFileFoundError)
}
