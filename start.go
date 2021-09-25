package main

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	log "github.com/sirupsen/logrus"

	"github.com/LightningTipBot/LightningTipBot/internal/lnbits"
	tb "gopkg.in/tucnak/telebot.v2"
	"gorm.io/gorm"
)

func (bot TipBot) startHandler(ctx context.Context, m *tb.Message) {
	if !m.Private() {
		return
	}
	// ATTENTION: DO NOT CALL ANY HANDLER BEFORE THE WALLET IS CREATED
	// WILL RESULT IN AN ENDLESS LOOP OTHERWISE
	// bot.helpHandler(m)
	log.Printf("[/start] User: %s (%d)\n", m.Sender.Username, m.Sender.ID)
	walletCreationMsg, err := bot.telegram.Send(m.Sender, Translate(ctx, "startSettingWalletMessage"))
	user, err := bot.initWallet(m.Sender)
	if err != nil {
		log.Errorln(fmt.Sprintf("[startHandler] Error with initWallet: %s", err.Error()))
		bot.tryEditMessage(walletCreationMsg, Translate(ctx, "startWalletErrorMessage"))
		return
	}
	bot.tryDeleteMessage(walletCreationMsg)
	userContext := context.WithValue(context.Background(), "user", user)
	bot.helpHandler(userContext, m)
	bot.trySendMessage(m.Sender, Translate(ctx, "startWalletReadyMessage"))
	bot.balanceHandler(userContext, m)

	// send the user a warning about the fact that they need to set a username
	if len(m.Sender.Username) == 0 {
		bot.trySendMessage(m.Sender, Translate(ctx, "startNoUsernameMessage"), tb.NoPreview)
	}
	return
}

func (bot TipBot) initWallet(tguser *tb.User) (*lnbits.User, error) {
	user, err := GetUser(tguser, bot)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		user = &lnbits.User{Telegram: tguser}
		err = bot.createWallet(user)
		if err != nil {
			return user, err
		}
		user.Initialized = true
		err = UpdateUserRecord(user, bot)
		if err != nil {
			log.Errorln(fmt.Sprintf("[initWallet] error updating user: %s", err.Error()))
			return user, err
		}
	} else if !user.Initialized {
		// update all tip tooltips (with the "initialize me" message) that this user might have received before
		tipTooltipInitializedHandler(user.Telegram, bot)
		user.Initialized = true
		err = UpdateUserRecord(user, bot)
		if err != nil {
			log.Errorln(fmt.Sprintf("[initWallet] error updating user: %s", err.Error()))
			return user, err
		}
	} else if user.Initialized {
		// wallet is already initialized
		return user, nil
	} else {
		err = fmt.Errorf("could not initialize wallet")
		return user, err
	}
	return user, nil
}

func (bot TipBot) createWallet(user *lnbits.User) error {
	UserStr := GetUserStr(user.Telegram)
	u, err := bot.client.CreateUserWithInitialWallet(strconv.Itoa(user.Telegram.ID),
		fmt.Sprintf("%d (%s)", user.Telegram.ID, UserStr),
		Configuration.Lnbits.AdminId,
		UserStr)
	if err != nil {
		errormsg := fmt.Sprintf("[createWallet] Create wallet error: %s", err)
		log.Errorln(errormsg)
		return err
	}
	user.Wallet = &lnbits.Wallet{}
	user.ID = u.ID
	user.Name = u.Name
	wallet, err := bot.client.Wallets(*user)
	if err != nil {
		errormsg := fmt.Sprintf("[createWallet] Get wallet error: %s", err)
		log.Errorln(errormsg)
		return err
	}
	user.Wallet = &wallet[0]
	user.Initialized = false
	err = UpdateUserRecord(user, bot)
	if err != nil {
		errormsg := fmt.Sprintf("[createWallet] Update user record error: %s", err)
		log.Errorln(errormsg)
		return err
	}
	return nil
}
