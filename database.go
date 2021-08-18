package main

import (
	"fmt"
	"strconv"

	log "github.com/sirupsen/logrus"

	"github.com/LightningTipBot/LightningTipBot/internal/lnbits"
	tb "gopkg.in/tucnak/telebot.v2"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func migration() (db *gorm.DB, txLogger *gorm.DB) {
	txLogger, err := gorm.Open(sqlite.Open(Configuration.TransactionsPath), &gorm.Config{DisableForeignKeyConstraintWhenMigrating: true, FullSaveAssociations: true})
	if err != nil {
		panic("Initialize orm failed.")
	}

	orm, err := gorm.Open(sqlite.Open(Configuration.DbPath), &gorm.Config{DisableForeignKeyConstraintWhenMigrating: true, FullSaveAssociations: true})
	if err != nil {
		panic("Initialize orm failed.")
	}

	err = orm.AutoMigrate(&lnbits.User{})
	if err != nil {
		panic(err)
	}
	err = txLogger.AutoMigrate(&Transaction{})
	if err != nil {
		panic(err)
	}
	return orm, txLogger
}

// GetUser from telegram user
func GetUser(u *tb.User, bot TipBot) (*lnbits.User, error) {
	user := &lnbits.User{Name: strconv.Itoa(u.ID)}
	tx := bot.database.First(user)
	user.Telegram = u
	if tx.Error != nil {
		errmsg := fmt.Sprintf("[GetUser] Couldn't fetch %s's info from database.", GetUserStr(u))
		log.Warnln(errmsg)
		return user, tx.Error
	}
	user.Wallet.Client = bot.client

	// update possibly changed user details in database
	return user, UpdateUserRecord(user, bot)
}

func UpdateUserRecord(user *lnbits.User, bot TipBot) error {
	tx := bot.database.Save(user)
	if tx.Error != nil {
		errmsg := fmt.Sprintf("[UpdateUserRecord] Error: Couldn't update %s's info in database.", GetUserStr(user.Telegram))
		log.Errorln(errmsg)
		return tx.Error
	}
	log.Debugf("[UpdateUserRecord] Records of user %s updated.", GetUserStr(user.Telegram))
	return nil
}
