package telegram

import (
	"context"
	stderrors "errors"
	"fmt"
	"github.com/LightningTipBot/LightningTipBot/internal"
	"github.com/LightningTipBot/LightningTipBot/internal/errors"
	"github.com/LightningTipBot/LightningTipBot/internal/lnbits"
	"github.com/LightningTipBot/LightningTipBot/internal/str"
	"github.com/LightningTipBot/LightningTipBot/internal/telegram/intercept"
	log "github.com/sirupsen/logrus"
	tb "gopkg.in/lightningtipbot/telebot.v3"
	"gorm.io/gorm"
	"strconv"
	"time"
)

func (bot TipBot) startHandler(ctx intercept.Context) (intercept.Context, error) {
	if !ctx.Message().Private() {
		return ctx, errors.Create(errors.NoPrivateChatError)
	}
	// ATTENTION: DO NOT CALL ANY HANDLER BEFORE THE WALLET IS CREATED
	// WILL RESULT IN AN ENDLESS LOOP OTHERWISE
	// bot.helpHandler(m)
	log.Printf("[⭐️ /start] New user: %s (%d)\n", GetUserStr(ctx.Sender()), ctx.Sender().ID)
	walletCreationMsg := bot.trySendMessageEditable(ctx.Sender(), Translate(ctx, "startSettingWalletMessage"))
	user, err := bot.userWithInitWallet(ctx.Sender())
	if err != nil {
		log.Errorln(fmt.Sprintf("[startHandler] Error with initWallet: %s", err.Error()))
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

// userWithInitWallet will ensure that lnbits.User is initialized.
// Initialized users have already talked to the bot,
// therefor they are able to receive messages from the bot.
func (bot TipBot) userWithInitWallet(tguser *tb.User) (*lnbits.User, error) {
	user, err := GetUser(tguser, bot)
	if err != nil {
		if stderrors.Is(err, gorm.ErrRecordNotFound) {
			user, err = bot.createWallet(tguser)
			if err != nil {
				return user, err
			}
			user.Initialized = true
			err = UpdateUserRecord(user, bot)
			if err != nil {
				return user, fmt.Errorf("[initWallet] error updating user: %v", err)
			}
			return user, nil
		}
		return nil, err
	}
	if !user.Initialized {
		// update all tip tooltips (with the "initialize me" message) that this user might have received before
		tipTooltipInitializedHandler(user.Telegram, bot)
		user.Initialized = true
		err = UpdateUserRecord(user, bot)
		if err != nil {
			return user, fmt.Errorf("[initWallet] error updating user: %v", err)
		}
	}
	// wallet is already initialized
	return user, nil
}

// createWallet will create a wallet for any telegram user.
func (bot TipBot) createWallet(u *tb.User) (*lnbits.User, error) {
	userStr := GetUserStr(u)
	user, err := bot.Client.CreateUserWithWallet(strconv.FormatInt(u.ID, 10),
		fmt.Sprintf("%d (%s)", u.ID, userStr),
		internal.Configuration.Lnbits.AdminId,
		userStr)
	if err != nil {
		return nil, fmt.Errorf("[createWallet] Create wallet error: %v", err)
	}
	user.Telegram = u
	user.AnonID = fmt.Sprint(str.Int32Hash(user.ID))
	user.AnonIDSha256 = str.AnonIdSha256(&user)
	user.UUID = str.UUIDSha256(&user)

	user.CreatedAt = time.Now()
	err = UpdateUserRecord(&user, bot)
	if err != nil {
		return nil, fmt.Errorf("[createWallet] Update user record error: %v", err)
	}
	return &user, nil
}
