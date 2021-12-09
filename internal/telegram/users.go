package telegram

import (
	"errors"
	"fmt"
	"time"

	"github.com/LightningTipBot/LightningTipBot/internal/lnbits"
	"github.com/LightningTipBot/LightningTipBot/internal/str"
	"github.com/eko/gocache/store"
	log "github.com/sirupsen/logrus"
	tb "gopkg.in/lightningtipbot/telebot.v2"
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
	log.Infof("[GetUserBalance] %s's balance: %d sat\n", GetUserStr(user.Telegram), amount)

	// update user balance in cache
	bot.Cache.Set(
		fmt.Sprintf("%s_balance", user.Name),
		amount,
		&store.Options{Expiration: 30 * time.Second},
	)
	return
}

func (bot *TipBot) CreateWalletForTelegramUser(tbUser *tb.User) (*lnbits.User, error) {
	user := &lnbits.User{Telegram: tbUser}
	userStr := GetUserStr(tbUser)
	log.Printf("[CreateWalletForTelegramUser] Creating wallet for user %s ... ", userStr)
	err := bot.createWallet(user)
	if err != nil {
		errmsg := fmt.Sprintf("[CreateWalletForTelegramUser] Error: Could not create wallet for user %s", userStr)
		log.Errorln(errmsg)
		return user, err
	}
	err = UpdateUserRecord(user, *bot)
	if err != nil {
		return nil, err
	}
	log.Printf("[CreateWalletForTelegramUser] Wallet created for user %s. ", userStr)
	return user, nil
}

func (bot *TipBot) UserExists(user *tb.User) (*lnbits.User, bool) {
	lnbitUser, err := GetUser(user, *bot)
	if err != nil || errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false
	}
	return lnbitUser, true
}
