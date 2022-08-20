package telegram

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/LightningTipBot/LightningTipBot/internal/lnbits"
	"github.com/LightningTipBot/LightningTipBot/internal/str"
	"github.com/eko/gocache/store"
	log "github.com/sirupsen/logrus"
	tb "gopkg.in/lightningtipbot/telebot.v3"
	"gorm.io/gorm"
)

func SetUserState(user *lnbits.User, bot *TipBot, stateKey lnbits.UserStateKey, stateData string) {
	user.StateKey = stateKey
	user.StateData = stateData
	UpdateUserRecord(user, *bot)

}

func ResetUserState(user *lnbits.User, bot *TipBot) {
	user.ResetState()
	UpdateUserRecord(user, *bot)
}

func GetUserStr(user *tb.User) string {
	userStr := fmt.Sprintf("@%s", user.Username)
	// if user does not have a username
	if len(userStr) < 2 && user.FirstName != "" {
		userStr = fmt.Sprintf("%s", user.FirstName)
	} else if len(userStr) < 2 {
		userStr = fmt.Sprintf("%d", user.ID)
	}
	return userStr
}

func GetUserStrMd(user *tb.User) string {
	userStr := fmt.Sprintf("@%s", user.Username)
	// if user does not have a username
	if len(userStr) < 2 && user.FirstName != "" {
		userStr = fmt.Sprintf("[%s](tg://user?id=%d)", user.FirstName, user.ID)
		return userStr
	} else if len(userStr) < 2 {
		userStr = fmt.Sprintf("[%d](tg://user?id=%d)", user.ID, user.ID)
		return userStr
	} else {
		// escape only if user has a username
		return str.MarkdownEscape(userStr)
	}
}

func appendUinqueUsersToSlice(slice []*tb.User, i *tb.User) []*tb.User {
	for _, ele := range slice {
		if ele.ID == i.ID {
			return slice
		}
	}
	return append(slice, i)
}

func (bot *TipBot) GetUserBalanceCached(user *lnbits.User) (amount int64, err error) {
	u, err := bot.Cache.Get(fmt.Sprintf("%s_balance", user.Name))
	if err != nil {
		return bot.GetUserBalance(user)
	}
	cachedBalance := u.(int64)
	return cachedBalance, nil
}

func (bot *TipBot) GetUserBalance(user *lnbits.User) (amount int64, err error) {
	if user.Wallet == nil {
		return 0, errors.New("User has no wallet")
	}

	wallet, err := bot.Client.Info(*user.Wallet)
	if err != nil {
		errmsg := fmt.Sprintf("[GetUserBalance] Error: Couldn't fetch user %s's info from LNbits: %s", GetUserStr(user.Telegram), err.Error())
		log.Errorln(errmsg)
		return
	}
	user.Wallet.Balance = wallet.Balance
	err = UpdateUserRecord(user, *bot)
	if err != nil {
		return
	}
	// msat to sat
	amount = int64(wallet.Balance) / 1000
	log.WithFields(log.Fields{
		"module":      "telegram",
		"func":        "GetUserBalance",
		"user":        GetUserStr(user.Telegram),
		"user_id":     user.ID,
		"wallet_id":   user.Wallet.ID,
		"telegram_id": user.Telegram.ID,
		"amount":      amount},
	).Debugf("updated user balance")

	// update user balance in cache
	bot.Cache.Set(
		fmt.Sprintf("%s_balance", user.Name),
		amount,
		&store.Options{Expiration: 1 * time.Hour},
	)
	return
}

func (bot *TipBot) CreateWalletForTelegramUser(tbUser *tb.User) (*lnbits.User, error) {
	// failsafe: do not create wallet for existing user
	if _, exists := bot.UserExists(tbUser); exists {
		return nil, fmt.Errorf("user already exists")
	}
	user := &lnbits.User{Telegram: tbUser}
	userStr := GetUserStr(tbUser)
	log.Printf("[CreateWalletForTelegramUser] Creating wallet for user %s ... ", userStr)
	err := bot.createWallet(user)
	if err != nil {
		return user, err
	}
	// todo: remove this. we're doing this already in bot.createWallet().
	err = UpdateUserRecord(user, *bot)
	if err != nil {
		return nil, err
	}
	log.WithFields(log.Fields{
		"module":    "telegram",
		"func":      "CreateWalletForTelegramUser",
		"user":      GetUserStr(user.Telegram),
		"user_id":   user.ID,
		"wallet_id": user.Wallet.ID,
		"error":     err.Error()},
	).Printf("Wallet created")
	return user, nil
}

func (bot *TipBot) UserExists(user *tb.User) (*lnbits.User, bool) {
	lnbitUser, err := GetUser(user, *bot)
	if err != nil || errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false
	}
	return lnbitUser, true
}

func (bot *TipBot) UserIsBanned(user *lnbits.User) bool {
	// do not respond to banned users
	if user.Wallet == nil {
		log.Tracef("[UserIsBanned] User %s has no wallet.\n", GetUserStr(user.Telegram))
		return false
	}
	if strings.HasPrefix(user.Wallet.Adminkey, "banned") || strings.Contains(user.Wallet.Adminkey, "_") {
		log.Debugf("[UserIsBanned] User %s is banned. Not responding.", GetUserStr(user.Telegram))
		return true
	}
	return false
}
