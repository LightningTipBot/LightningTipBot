package main

import (
	"errors"
	"fmt"
	"gorm.io/gorm"
	"strings"

	"github.com/LightningTipBot/LightningTipBot/internal/lnbits"
	log "github.com/sirupsen/logrus"

	tb "gopkg.in/tucnak/telebot.v2"
)

var markdownV2Escapes = []string{"_", "[", "]", "(", ")", "~", "`", ">", "#", "+", "-", "=", "|", "{", "}", ".", "!"}
var markdownEscapes = []string{"_"}

func MarkdownV2Escape(s string) string {
	for _, esc := range markdownV2Escapes {
		if strings.Contains(s, esc) {
			s = strings.Replace(s, esc, fmt.Sprintf("\\%s", esc), -1)
		}
	}
	return s
}

func MarkdownEscape(s string) string {
	for _, esc := range markdownEscapes {
		if strings.Contains(s, esc) {
			s = strings.Replace(s, esc, fmt.Sprintf("\\%s", esc), -1)
		}
	}
	return s
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
	} else if len(userStr) < 2 {
		userStr = fmt.Sprintf("[%d](tg://user?id=%d)", user.ID, user.ID)
	}
	return MarkdownEscape(userStr)
}

func appendUinqueUsersToSlice(slice []*tb.User, i *tb.User) []*tb.User {
	for _, ele := range slice {
		if ele.ID == i.ID {
			return slice
		}
	}
	return append(slice, i)
}

func (bot *TipBot) GetUserBalance(fromUser *lnbits.User) (amount int, err error) {
	// get user
	wallet, err := fromUser.Wallet.Info(*fromUser.Wallet)
	if err != nil {
		errmsg := fmt.Sprintf("[GetUserBalance] Error: Couldn't fetch user %s's info from LNbits: %s", GetUserStr(fromUser.Telegram), err)
		log.Errorln(errmsg)
		return
	}
	fromUser.Wallet.Balance = wallet.Balance
	err = UpdateUserRecord(fromUser, *bot)
	if err != nil {
		return
	}
	// msat to sat
	amount = int(wallet.Balance) / 1000
	log.Infof("[GetUserBalance] %s's balance: %d sat\n", GetUserStr(fromUser.Telegram), amount)
	return
}


func (bot *TipBot) copyLowercaseUser(u *tb.User) *tb.User {
	userCopy := *u
	userCopy.Username = strings.ToLower(u.Username)
	return &userCopy
}

func (bot *TipBot) CreateWalletForTelegramUser(tbUser *tb.User) (*lnbits.User, error) {
	userCopy := bot.copyLowercaseUser(tbUser)
	user := &lnbits.User{Telegram: userCopy}
	userStr := GetUserStr(tbUser)
	log.Printf("[CreateWalletForTelegramUser] Creating wallet for user %s ... ", userStr)
	err := bot.createWallet(user)
	if err != nil {
		errmsg := fmt.Sprintf("[CreateWalletForTelegramUser] Error: Could not create wallet for user %s", userStr)
		log.Errorln(errmsg)
		return nil, err
	}
	tx := bot.database.Save(user)
	if tx.Error != nil {
		return nil, tx.Error
	}
	log.Printf("[CreateWalletForTelegramUser] Wallet created for user %s. ", userStr)
	return user, nil
}

func (bot *TipBot) UserExists(user *tb.User) (*lnbits.User, bool) {
	lnbitUser, err := GetUser(user, *bot)
	if err != nil && errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false
	}
	return lnbitUser, true
}
