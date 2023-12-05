package telegram

import (
	"fmt"
	"strings"

	"github.com/LightningTipBot/LightningTipBot/internal/telegram/intercept"
	log "github.com/sirupsen/logrus"
)

var (
	settingsHelpMessage = "📖 Change user settings\n\n`/set unit <BTC|USD|EUR|GBP>` 💶 Change your default currency."
)

func (bot *TipBot) settingHandler(ctx intercept.Context) (intercept.Context, error) {
	m := ctx.Message()
	splits := strings.Split(m.Text, " ")
	if len(splits) == 1 {
		bot.trySendMessage(m.Sender, settingsHelpMessage)
	} else if len(splits) > 1 {
		switch strings.ToLower(splits[1]) {
		case "unit":
			return bot.addFiatCurrency(ctx)
		case "help":
			return bot.nostrHelpHandler(ctx)
		}
	}
	return ctx, nil
}

func (bot *TipBot) addFiatCurrency(ctx intercept.Context) (intercept.Context, error) {
	m := ctx.Message()
	user, err := GetLnbitsUserWithSettings(m.Sender, *bot)
	if err != nil {
		return ctx, err
	}

	splits := strings.Split(m.Text, " ")
	splitlen := len(splits)
	if splitlen < 3 {
		// display users current fiat currency
		currentCurrency := strings.ToUpper(user.Settings.Display.DisplayCurrency)
		if currentCurrency == "" {
			currentCurrency = "BTC"
		}
		bot.trySendMessage(ctx.Message().Sender, fmt.Sprintf("🌍 Your current default currency is `%s`", currentCurrency))
		return ctx, nil
	}
	currencyInput := splits[2]
	// convert to lowercase and check if in [usd, eur, gbp, btc, sat]
	currencyInput = strings.ToLower(currencyInput)
	if currencyInput != "usd" && currencyInput != "eur" && currencyInput != "gbp" && currencyInput != "btc" && currencyInput != "sat" {
		bot.trySendMessage(ctx.Message().Sender, "🚫 Invalid currency. Please use one of the following: `BTC`, `USD`, `EUR`")
		return ctx, fmt.Errorf("invalid currency")
	}
	if currencyInput == "sat" {
		currencyInput = "BTC"
	}
	// save node in db
	user.Settings.Display.DisplayCurrency = currencyInput
	err = UpdateUserRecord(user, *bot)
	if err != nil {
		log.Errorf("[registerNodeHandler] could not update record of user %s: %v", GetUserStr(user.Telegram), err)
		return ctx, err
	}
	bot.trySendMessage(ctx.Message().Sender, "✅ Your default currency has been updated.")
	return ctx, nil
}
