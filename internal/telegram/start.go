package telegram

import (
	"context"
	stderrors "errors"
	"fmt"
	"github.com/LightningTipBot/LightningTipBot/internal/telegram/intercept"
	"strconv"
	"time"

	"github.com/LightningTipBot/LightningTipBot/internal/errors"

	"github.com/LightningTipBot/LightningTipBot/internal"

	log "github.com/sirupsen/logrus"

	"github.com/LightningTipBot/LightningTipBot/internal/lnbits"
	"github.com/LightningTipBot/LightningTipBot/internal/str"
	tb "gopkg.in/lightningtipbot/telebot.v3"
	"gorm.io/gorm"
)

func (bot TipBot) startHandler(ctx intercept.Context) (intercept.Context, error) {
	if !ctx.Message().Private() {
		return ctx, errors.Create(errors.NoPrivateChatError)
	}
	// ATTENTION: DO NOT CALL ANY HANDLER BEFORE THE WALLET IS CREATED
	// WILL RESULT IN AN ENDLESS LOOP OTHERWISE
	// bot.helpHandler(m)
	log.WithFields(log.Fields{
		"module":  "telegram",
		"func":    "startHandler",
		"path":    "/start",
		"user":    GetUserStr(ctx.Sender()),
		"user_id": ctx.Sender().ID,
	}).Printf(" New user")
	walletCreationMsg := bot.trySendMessageEditable(ctx.Sender(), Translate(ctx, "startSettingWalletMessage"))
	user, err := bot.initWallet(ctx.Sender())
	if err != nil {
		log.WithFields(log.Fields{
			"module":  "telegram",
			"func":    "startHandler",
			"path":    "/start",
			"user":    GetUserStr(ctx.Sender()),
			"user_id": ctx.Sender().ID,
		}).Errorln(fmt.Sprintf("Error with initWallet: %s", err.Error()))
		bot.tryEditMessage(walletCreationMsg, Translate(ctx, "startWalletErrorMessage"))
		return ctx, err
	}
	bot.tryDeleteMessage(walletCreationMsg)
	ctx.Context = context.WithValue(ctx, "user", user)
	bot.helpHandler(ctx)
	bot.trySendMessage(ctx.Sender(), Translate(ctx, "startWalletReadyMessage"))
	bot.balanceHandler(ctx)

	// send the user a warning about the fact that they need to set a username
	if len(ctx.Sender().Username) == 0 {
		bot.trySendMessage(ctx.Sender(), Translate(ctx, "startNoUsernameMessage"), tb.NoPreview)
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
			return user, err
		}
	} else if !user.Initialized {
		// update all tip tooltips (with the "initialize me" message) that this user might have received before
		tipTooltipInitializedHandler(user.Telegram, bot)
		user.Initialized = true
		err = UpdateUserRecord(user, bot)
		if err != nil {
			return user, err
		}
	} else if user.Initialized {
		// wallet is already initialized
		return user, nil
	} else {
		return user, fmt.Errorf("could not initialize wallet")
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
		return err
	}
	user.Wallet = &lnbits.Wallet{}
	user.ID = u.ID
	user.Name = u.Name
	wallet, err := bot.Client.Wallets(*user)
	if err != nil {
		return err
	}
	user.Wallet = &wallet[0]

	user.AnonID = fmt.Sprint(str.Int32Hash(user.ID))
	user.AnonIDSha256 = str.AnonIdSha256(user)
	user.UUID = str.UUIDSha256(user)

	user.Initialized = false
	user.CreatedAt = time.Now()
	err = UpdateUserRecord(user, bot)
	if err != nil {
		return err
	}
	return nil
}
