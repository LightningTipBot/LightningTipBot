package telegram

import (
	"context"

	"github.com/LightningTipBot/LightningTipBot/internal/lnbits"
	tb "gopkg.in/tucnak/telebot.v2"
)

type StateCallbackMessage map[lnbits.UserStateKey]func(ctx context.Context, m *tb.Message)

var stateCallbackMessage StateCallbackMessage

func initializeStateCallbackMessage(bot *TipBot) {
	stateCallbackMessage = StateCallbackMessage{
		lnbits.UserStateLNURLEnterAmount:  bot.enterAmountHandler,
		lnbits.UserEnterAmount:            bot.enterAmountHandler,
		lnbits.UserEnterUser:              bot.enterUserHandler,
		lnbits.UserEnterShopTitle:         bot.enterShopTitleHandler,
		lnbits.UserStateShopItemSendPhoto: bot.addShopItemPhoto,
	}
}
