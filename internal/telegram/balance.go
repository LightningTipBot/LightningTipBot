package telegram

import (
	"fmt"
	"github.com/LightningTipBot/LightningTipBot/internal/errors"
	"github.com/LightningTipBot/LightningTipBot/internal/telegram/intercept"

	log "github.com/sirupsen/logrus"

	tb "gopkg.in/telebot.v3"
)

func (bot *TipBot) balanceHandler(handler intercept.Handler) (intercept.Handler, error) {
	m := handler.Message()
	// check and print all commands
	if len(m.Text) > 0 {
		bot.anyTextHandler(handler)
	}

	// reply only in private message
	if m.Chat.Type != tb.ChatPrivate {
		// delete message
		bot.tryDeleteMessage(m)
	}
	// first check whether the user is initialized
	user := LoadUser(handler.Ctx)
	if user.Wallet == nil {
		return handler, errors.Create(errors.UserNoWalletError)
	}

	if !user.Initialized {
		return bot.startHandler(handler)
	}

	usrStr := GetUserStr(handler.Sender())
	balance, err := bot.GetUserBalance(user)
	if err != nil {
		log.Errorf("[/balance] Error fetching %s's balance: %s", usrStr, err)
		bot.trySendMessage(handler.Sender(), Translate(handler.Ctx, "balanceErrorMessage"))
		return handler, err
	}

	log.Infof("[/balance] %s's balance: %d sat\n", usrStr, balance)
	bot.trySendMessage(handler.Sender(), fmt.Sprintf(Translate(handler.Ctx, "balanceMessage"), balance))
	return handler, nil
}
