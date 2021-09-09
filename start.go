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

const (
	startSettingWalletMessage = "🧮 Setting up your wallet..."
	startWalletCreatedMessage = "🧮 Wallet created."
	startWalletReadyMessage   = "✅ *Your wallet is ready.*"
	startWalletErrorMessage   = "🚫 Error initializing your wallet. Try again later."
	startNoUsernameMessage    = "☝️ It looks like you don't have a Telegram @username yet. That's ok, you don't need one to use this bot. However, to make better use of your wallet, set up a username in the Telegram settings. Then, enter /balance so the bot can update its record of you."
)

func (bot TipBot) startHandler(m *tb.Message) {
	if !m.Private() {
		return
	}
	// ATTENTION: DO NOT CALL ANY HANDLER BEFORE THE WALLET IS CREATED
	// WILL RESULT IN AN ENDLESS LOOP OTHERWISE
	// bot.helpHandler(m)
	log.Printf("[/start] User: %s (%d)\n", m.Sender.Username, m.Sender.ID)
	walletCreationMsg, err := bot.telegram.Send(m.Sender, startSettingWalletMessage)
	user, err := bot.initWallet(m.Sender)
	if err != nil {
		log.Errorln(fmt.Sprintf("[startHandler] Error with initWallet: %s", err.Error()))
		bot.tryEditMessage(walletCreationMsg, startWalletErrorMessage)
		return
	}
	bot.tryDeleteMessage(walletCreationMsg)

	bot.helpHandler(m)
	bot.trySendMessage(m.Sender, startWalletReadyMessage)
	bot.balanceHandler(context.WithValue(context.Background(), "user", user), m)

	// send the user a warning about the fact that they need to set a username
	if len(m.Sender.Username) == 0 {
		bot.trySendMessage(m.Sender, startNoUsernameMessage, tb.NoPreview)
	}
	return
}

func (bot TipBot) initWallet(tguser *tb.User) (*lnbits.User, error) {
	user, err := GetUser(tguser, bot)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		u := &lnbits.User{Telegram: tguser}
		err = bot.createWallet(u)
		if err != nil {
			return u, err
		}
		u.Initialized = true
		err = UpdateUserRecord(u, bot)
		if err != nil {
			log.Errorln(fmt.Sprintf("[initWallet] error updating user: %s", err.Error()))
			return u, err
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
		return user,nil
	} else {
		err = fmt.Errorf("could not initialize wallet")
		return user,err
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
	user.Wallet = &lnbits.Wallet{Client: bot.client}
	user.ID = u.ID
	user.Name = u.Name
	wallet, err := user.Wallet.Wallets(*user)
	if err != nil {
		errormsg := fmt.Sprintf("[createWallet] Get wallet error: %s", err)
		log.Errorln(errormsg)
		return err
	}
	user.Wallet = &wallet[0]
	user.Wallet.Client = bot.client
	user.Initialized = false
	err = UpdateUserRecord(user, bot)
	if err != nil {
		errormsg := fmt.Sprintf("[createWallet] Update user record error: %s", err)
		log.Errorln(errormsg)
		return err
	}
	return nil
}
