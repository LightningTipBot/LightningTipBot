package telegram

import (
	"context"
	"time"

	"github.com/LightningTipBot/LightningTipBot/internal/lnbits"
	tb "gopkg.in/lightningtipbot/telebot.v2"
)

type StateCallbackMessage map[lnbits.UserStateKey]func(ctx context.Context, m *tb.Message)

var stateCallbackMessage StateCallbackMessage

func initializeStateCallbackMessage(bot *TipBot) {
	stateCallbackMessage = StateCallbackMessage{
		lnbits.UserStateLNURLEnterAmount:     bot.enterAmountHandler,
		lnbits.UserEnterAmount:               bot.enterAmountHandler,
		lnbits.UserEnterUser:                 bot.enterUserHandler,
		lnbits.UserEnterShopTitle:            bot.enterShopTitleHandler,
		lnbits.UserStateShopItemSendPhoto:    bot.addShopItemPhoto,
		lnbits.UserStateShopItemSendPrice:    bot.enterShopItemPriceHandler,
		lnbits.UserStateShopItemSendTitle:    bot.enterShopItemTitleHandler,
		lnbits.UserStateShopItemSendItemFile: bot.addItemFileHandler,
		lnbits.UserEnterShopsDescription:     bot.enterShopsDescriptionHandler,
	}
}

var tickerCoolDown = time.Second * 10

type UserStateTicker struct {
	user            *lnbits.User
	ticker          *time.Ticker
	bot             *TipBot
	ticketResetChan chan struct{}
}

func (t UserStateTicker) Do() {
	for {
		select {
		case <-t.ticker.C:
			ResetUserState(t.user, t.bot)
			return
		case <-t.ticketResetChan:
			t.ticker = time.NewTicker(tickerCoolDown)
		}
	}
}
