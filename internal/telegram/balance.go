package telegram

import (
	"fmt"
	"github.com/LightningTipBot/LightningTipBot/internal/errors"
	"github.com/LightningTipBot/LightningTipBot/internal/telegram/intercept"

	log "github.com/sirupsen/logrus"

	tb "gopkg.in/lightningtipbot/telebot.v3"
)

func (bot *TipBot) balanceHandler(ctx intercept.Context) (intercept.Context, error) {
	m := ctx.Message()
	// check and print all commands
	if len(m.Text) > 0 {
		bot.anyTextHandler(ctx)
	}

	// reply only in private message
	if m.Chat.Type != tb.ChatPrivate {
		// delete message
		bot.tryDeleteMessage(m)
	}
	// first check whether the user is initialized
	user := LoadUser(ctx)
	if user.Wallet == nil {
		return ctx, errors.Create(errors.UserNoWalletError)
	}

	if !user.Initialized {
		return bot.startHandler(ctx)
	}

	usrStr := GetUserStr(ctx.Sender())
	balance, err := bot.GetUserBalance(user)
	if err != nil {
		log.WithFields(log.Fields{
			"module":      "telegram",
			"func":        "balanceHandler",
			"path":        "/balance",
			"user":        usrStr,
			"user_id":     user.ID,
			"wallet_id":   user.Wallet.ID,
			"telegram_id": user.Telegram.ID}).Errorf("Error fetching balance: %s", err)
		bot.trySendMessage(ctx.Sender(), Translate(ctx, "balanceErrorMessage"))
		return ctx, err
	}

	log.WithFields(log.Fields{
		"module":      "telegram",
		"func":        "balanceHandler",
		"path":        "/balance",
		"amount":      balance,
		"user":        usrStr,
		"user_id":     user.ID,
		"wallet_id":   user.Wallet.ID,
		"telegram_id": user.Telegram.ID}).Infof("sending balance to user")
	bot.trySendMessage(ctx.Sender(), fmt.Sprintf(Translate(ctx, "balanceMessage"), balance))
	return ctx, nil
}
