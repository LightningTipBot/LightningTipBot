package telegram

import (
	"context"
	stderrors "errors"
	"fmt"
	"strconv"
	"time"

	"github.com/LightningTipBot/LightningTipBot/internal/errors"

	"github.com/LightningTipBot/LightningTipBot/internal"

	log "github.com/sirupsen/logrus"

	"github.com/LightningTipBot/LightningTipBot/internal/lnbits"
	"github.com/LightningTipBot/LightningTipBot/internal/str"
	tb "gopkg.in/lightningtipbot/telebot.v2"
	"gorm.io/gorm"
)

func (bot TipBot) startHandler(ctx context.Context, m *tb.Message) (context.Context, error) {
	if !m.Private() {
		return ctx, errors.Create(errors.NoPrivateChatError)
	}
	// ATTENTION: DO NOT CALL ANY HANDLER BEFORE THE WALLET IS CREATED
	// WILL RESULT IN AN ENDLESS LOOP OTHERWISE
	// bot.helpHandler(m)
	log.Printf("[⭐️ /start] New user: %s (%d)\n", GetUserStr(m.Sender), m.Sender.ID)
	walletCreationMsg := bot.trySendMessage(m.Sender, Translate(ctx, "startSettingWalletMessage"))
	user, err := bot.initWallet(m.Sender)
	if err != nil {
		log.Errorln(fmt.Sprintf("[startHandler] Error with initWallet: %s", err.Error()))
		bot.tryEditMessage(walletCreationMsg, Translate(ctx, "startWalletErrorMessage"))
		return ctx, err
	}
	bot.tryDeleteMessage(walletCreationMsg)
	ctx = context.WithValue(ctx, "user", user)
	bot.helpHandler(ctx, m)
	bot.trySendMessage(m.Sender, Translate(ctx, "startWalletReadyMessage"))
	bot.balanceHandler(ctx, m)

	// send the user a warning about the fact that they need to set a username
	if len(m.Sender.Username) == 0 {
		bot.trySendMessage(m.Sender, Translate(ctx, "startNoUsernameMessage"), tb.NoPreview)
	}
	return ctx, nil
}

func (bot TipBot) initWallet(tguser *tb.User) (*lnbits.User, error) {
	user, err := GetUser(tguser, bot)
	if stderrors.Is(err, gorm.ErrRecordNotFound) {
		user = &lnbits.User{Telegram: tguser}
		err = bot.createWallet(user)
		if err != nil {
			return user, err
		}
		// set user initialized
		user, err := GetUser(tguser, bot)
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
	u, err := bot.Client.CreateUserWithInitialWallet(strconv.FormatInt(user.Telegram.ID, 10),
		fmt.Sprintf("%d (%s)", user.Telegram.ID, UserStr),
		internal.Configuration.Lnbits.AdminId,
		UserStr)
	if err != nil {
		errormsg := fmt.Sprintf("[createWallet] Create wallet error: %s", err.Error())
		log.Errorln(errormsg)
		return err
	}
	user.Wallet = &lnbits.Wallet{}
	user.ID = u.ID
	user.Name = u.Name
	wallet, err := bot.Client.Wallets(*user)
	if err != nil {
		errormsg := fmt.Sprintf("[createWallet] Get wallet error: %s", err.Error())
		log.Errorln(errormsg)
		return err
	}
	user.Wallet = &wallet[0]
	user.AnonID = fmt.Sprint(str.Int32Hash(user.ID))
	user.AnonIDSha256 = str.AnonIdSha256(user)
	user.Initialized = false
	user.CreatedAt = time.Now()
	err = UpdateUserRecord(user, bot)
	if err != nil {
		errormsg := fmt.Sprintf("[createWallet] Update user record error: %s", err.Error())
		log.Errorln(errormsg)
		return err
	}
	return nil
}
