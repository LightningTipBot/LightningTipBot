package telegram

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/LightningTipBot/LightningTipBot/internal"
	"github.com/LightningTipBot/LightningTipBot/internal/database"
	"github.com/LightningTipBot/LightningTipBot/internal/storage"
	"github.com/tidwall/buntdb"

	log "github.com/sirupsen/logrus"

	"github.com/LightningTipBot/LightningTipBot/internal/lnbits"
	tb "gopkg.in/tucnak/telebot.v2"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

const (
	MessageOrderedByReplyToFrom = "message.reply_to_message.from.id"
	TipTooltipKeyPattern        = "tip-tool-tip:*"
)

func createBunt() *storage.DB {
	// create bunt database
	bunt := storage.NewBunt(internal.Configuration.Database.BuntDbPath)
	// create bunt database index for ascending (searching) TipTooltips
	err := bunt.CreateIndex(MessageOrderedByReplyToFrom, TipTooltipKeyPattern, buntdb.IndexJSON(MessageOrderedByReplyToFrom))
	if err != nil {
		panic(err)
	}
	return bunt
}

func UserDbMigrations(db *gorm.DB) {
	// db.Migrator().DropColumn(&lnbits.User{}, "anon_id")
	if !db.Migrator().HasColumn(&lnbits.User{}, "anon_id") {
		log.Info("Running user database migrations ...")
		database.MigrateUSerDBHash(db)
		log.Info("User database migrations complete.")
	}
}

func AutoMigration() (db *gorm.DB, txLogger *gorm.DB) {
	orm, err := gorm.Open(sqlite.Open(internal.Configuration.Database.DbPath), &gorm.Config{DisableForeignKeyConstraintWhenMigrating: true, FullSaveAssociations: true})
	if err != nil {
		panic("Initialize orm failed.")
	}
	err = orm.AutoMigrate(&lnbits.User{})
	if err != nil {
		panic(err)
	}
	// db migrations that are due to updates to the bot
	UserDbMigrations(orm)

	txLogger, err = gorm.Open(sqlite.Open(internal.Configuration.Database.TransactionsPath), &gorm.Config{DisableForeignKeyConstraintWhenMigrating: true, FullSaveAssociations: true})
	if err != nil {
		panic("Initialize orm failed.")
	}
	err = txLogger.AutoMigrate(&Transaction{})
	if err != nil {
		panic(err)
	}
	return orm, txLogger
}

func GetUserByTelegramUsername(toUserStrWithoutAt string, bot TipBot) (*lnbits.User, error) {
	toUserDb := &lnbits.User{}
	tx := bot.Database.Where("telegram_username = ?", strings.ToLower(toUserStrWithoutAt)).First(toUserDb)
	if tx.Error != nil || toUserDb.Wallet == nil {
		err := tx.Error
		if toUserDb.Wallet == nil {
			err = fmt.Errorf("%s | user @%s has no wallet", tx.Error, toUserStrWithoutAt)
		}
		return nil, err
	}
	return toUserDb, nil
}

// GetLnbitsUser will not update the user in Database.
// this is required, because fetching lnbits.User from a incomplete tb.User
// will update the incomplete (partial) user in storage.
// this function will accept users like this:
// &tb.User{ID: toId, Username: username}
// without updating the user in storage.
func GetLnbitsUser(u *tb.User, bot TipBot) (*lnbits.User, error) {
	user := &lnbits.User{Name: strconv.Itoa(u.ID)}
	tx := bot.Database.First(user)
	if tx.Error != nil {
		errmsg := fmt.Sprintf("[GetUser] Couldn't fetch %s from Database: %s", GetUserStr(u), tx.Error.Error())
		log.Warnln(errmsg)
		user.Telegram = u
		return user, tx.Error
	}
	return user, nil
}

// GetUser from Telegram user. Update the user if user information changed.
func GetUser(u *tb.User, bot TipBot) (*lnbits.User, error) {
	user, err := GetLnbitsUser(u, bot)
	if err != nil {
		return user, err
	}
	go func() {
		userCopy := bot.CopyLowercaseUser(u)
		if !reflect.DeepEqual(userCopy, user.Telegram) {
			// update possibly changed user details in Database
			user.Telegram = userCopy
			err = UpdateUserRecord(user, bot)
			if err != nil {
				log.Warnln(fmt.Sprintf("[UpdateUserRecord] %s", err.Error()))
			}
		}
	}()
	return user, err
}

func UpdateUserRecord(user *lnbits.User, bot TipBot) error {
	user.Telegram = bot.CopyLowercaseUser(user.Telegram)
	user.UpdatedAt = time.Now()
	tx := bot.Database.Save(user)
	if tx.Error != nil {
		errmsg := fmt.Sprintf("[UpdateUserRecord] Error: Couldn't update %s's info in Database.", GetUserStr(user.Telegram))
		log.Errorln(errmsg)
		return tx.Error
	}
	log.Debugf("[UpdateUserRecord] Records of user %s updated.", GetUserStr(user.Telegram))
	return nil
}
