package telegram

import (
	"context"
	"github.com/LightningTipBot/LightningTipBot/internal/runtime"
	"github.com/orcaman/concurrent-map"
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
		// found handler for this state
		// now looking for user state reset ticker
		if t, ok := fileStateResetTicker.Get(user.ID); ok {
			// state reset ticker found. resetting ticker.
			t.(*runtime.ResettableFunctionTicker).ResetChan <- struct{}{}
		} else {
			// state reset ticker not found. creating new one.
			ticker := runtime.NewResettableFunctionTicker()
			// storing reset ticker in mem
			fileStateResetTicker.Set(user.ID, ticker)
			go func() {
				// starting ticker
				ticker.Do(func() {
					ResetUserState(user, bot)
					// removing ticker asap done
					fileStateResetTicker.Remove(user.ID)
				})
			}()
		}
		c(ctx, m)
		return
	}
}
