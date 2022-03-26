package telegram

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"os"
	"reflect"
	"runtime/debug"
	"strconv"
	"time"

	"github.com/LightningTipBot/LightningTipBot/internal/str"

	"github.com/eko/gocache/store"

	"github.com/LightningTipBot/LightningTipBot/internal"
	"github.com/LightningTipBot/LightningTipBot/internal/database"
	"github.com/LightningTipBot/LightningTipBot/internal/storage"
	"github.com/tidwall/buntdb"

	log "github.com/sirupsen/logrus"

	"github.com/LightningTipBot/LightningTipBot/internal/lnbits"
	tb "gopkg.in/lightningtipbot/telebot.v3"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

const (
	MessageOrderedByReplyToFrom = "message.reply_to_message.from.id"
	TipTooltipKeyPattern        = "tip-tool-tip:*"
)

func createBunt(file string) *storage.DB {
	// create bunt database
	bunt := storage.NewBunt(file)
	// create bunt database index for ascending (searching) TipTooltips
	err := bunt.CreateIndex(MessageOrderedByReplyToFrom, TipTooltipKeyPattern, buntdb.IndexJSON(MessageOrderedByReplyToFrom))
	if err != nil {
		panic(err)
	}
	return bunt
}

func ColumnMigrationTasks(db *gorm.DB) error {
	var err error
	// anon_id migration (2021-11-01)
	if !db.Migrator().HasColumn(&lnbits.User{}, "anon_id") {
		// first we need to auto migrate the user. This will create anon_id column
		err = db.AutoMigrate(&lnbits.User{})
		if err != nil {
			panic(err)
		}
		log.Info("Running anon_id database migrations ...")
		// run the migration on anon_id
		err = database.MigrateAnonIdInt32Hash(db)
	}

	// anon_id_sha256 migration (2022-01-01)
	if !db.Migrator().HasColumn(&lnbits.User{}, "anon_id_sha256") {
		// first we need to auto migrate the user. This will create anon_id column
		err = db.AutoMigrate(&lnbits.User{})
		if err != nil {
			panic(err)
		}
		log.Info("Running anon_id_sha256 database migrations ...")
		// run the migration on anon_id
		err = database.MigrateAnonIdSha265Hash(db)
	}

	// uuid migration (2022-02-11)
	if !db.Migrator().HasColumn(&lnbits.User{}, "uuid") {
		// first we need to auto migrate the user. This will create uuid column
		err = db.AutoMigrate(&lnbits.User{})
		if err != nil {
			panic(err)
		}
		log.Info("Running UUID database migrations ...")
		// run the migration on uuid
		err = database.MigrateUUIDSha265Hash(db)
	}

	// todo -- add more database field migrations here in the future
	return err
}

func AutoMigration() (db *gorm.DB, txLogger *gorm.DB, groupsDb *gorm.DB) {
	orm, err := gorm.Open(sqlite.Open(internal.Configuration.Database.DbPath), &gorm.Config{DisableForeignKeyConstraintWhenMigrating: true, FullSaveAssociations: true})
	if err != nil {
		panic("Initialize orm failed.")
	}
	err = ColumnMigrationTasks(orm)
	if err != nil {
		panic(err)
	}
	err = orm.AutoMigrate(&lnbits.User{})
	if err != nil {
		panic(err)
	}

	txLogger, err = gorm.Open(sqlite.Open(internal.Configuration.Database.TransactionsPath), &gorm.Config{DisableForeignKeyConstraintWhenMigrating: true, FullSaveAssociations: true})
	if err != nil {
		panic("Initialize orm failed.")
	}
	err = txLogger.AutoMigrate(&Transaction{})
	if err != nil {
		panic(err)
	}

	groupsDb, err = gorm.Open(sqlite.Open(internal.Configuration.Database.GroupsDbPath), &gorm.Config{DisableForeignKeyConstraintWhenMigrating: true, FullSaveAssociations: true})
	if err != nil {
		panic("Initialize orm failed.")
	}
	err = groupsDb.AutoMigrate(&Group{})
	if err != nil {
		panic(err)
	}
	return orm, txLogger, groupsDb
}

func GetUserByTelegramUsername(toUserStrWithoutAt string, bot TipBot) (*lnbits.User, error) {
	toUserDb := &lnbits.User{}
	// return error if username is too long
	if len(toUserStrWithoutAt) > 100 {
		return nil, fmt.Errorf("[GetUserByTelegramUsername] Telegram username is too long: %s..", toUserStrWithoutAt[:100])
	}
	tx := bot.Database.Where("telegram_username = ? COLLATE NOCASE", toUserStrWithoutAt).First(toUserDb)
	if tx.Error != nil || toUserDb.Wallet == nil {
		err := tx.Error
		if toUserDb.Wallet == nil {
			err = fmt.Errorf("%s | user @%s has no wallet", tx.Error, toUserStrWithoutAt)
		}
		return nil, err
	}
	return toUserDb, nil
}
func getCachedUser(u *tb.User, bot TipBot) (*lnbits.User, error) {
	user := &lnbits.User{Name: strconv.FormatInt(u.ID, 10)}
	if us, err := bot.Cache.Get(user.Name); err == nil {
		return us.(*lnbits.User), nil
	}
	user.Telegram = u
	return user, gorm.ErrRecordNotFound
}

// GetLnbitsUser will not update the user in Database.
// this is required, because fetching lnbits.User from a incomplete tb.User
// will update the incomplete (partial) user in storage.
// this function will accept users like this:
// &tb.User{ID: toId, Username: username}
// without updating the user in storage.
func GetLnbitsUser(u *tb.User, bot TipBot) (*lnbits.User, error) {
	user := &lnbits.User{Name: strconv.FormatInt(u.ID, 10)}
	tx := bot.Database.First(user)
	if tx.Error != nil {
		errmsg := fmt.Sprintf("[GetUser] Couldn't fetch %s from Database: %s", GetUserStr(u), tx.Error.Error())
		log.Warnln(errmsg)
		user.Telegram = u
		return user, tx.Error
	}
	// todo -- unblock this !
	return user, nil
}

// GetUser from Telegram user. Update the user if user information changed.
func GetUser(u *tb.User, bot TipBot) (*lnbits.User, error) {
	var user *lnbits.User
	var err error
	if user, err = getCachedUser(u, bot); err != nil {
		user, err = GetLnbitsUser(u, bot)
		if err != nil {
			return user, err
		}
		updateCachedUser(user, bot)
	}
	if telegramUserChanged(u, user.Telegram) {
		// update possibly changed user details in Database
		user.Telegram = u
		err = UpdateUserRecord(user, bot)
		if err != nil {
			log.Warnln(fmt.Sprintf("[UpdateUserRecord] %s", err.Error()))
		}
	}
	return user, err
}

func updateCachedUser(apiUser *lnbits.User, bot TipBot) {
	bot.Cache.Set(apiUser.Name, apiUser, &store.Options{Expiration: 1 * time.Minute})
}

func telegramUserChanged(apiUser, stateUser *tb.User) bool {
	if reflect.DeepEqual(apiUser, stateUser) {
		return false
	}
	return true
}
func debugStack() {
	stack := debug.Stack()
	go func() {
		hasher := sha1.New()
		hasher.Write(stack)
		sha := base64.URLEncoding.EncodeToString(hasher.Sum(nil))
		fo, err := os.Create(fmt.Sprintf("trace_%s.txt", sha))
		log.Infof("[debugStack] ⚠️ Writing stack trace to %s", fmt.Sprintf("trace_%s.txt", sha))
		if err != nil {
			panic(err)
		}
		defer func() {
			if err := fo.Close(); err != nil {
				panic(err)
			}
		}()
		w := bufio.NewWriter(fo)
		if _, err := w.Write(stack); err != nil {
			panic(err)
		}

		if err = w.Flush(); err != nil {
			panic(err)
		}
	}()
}
func UpdateUserRecord(user *lnbits.User, bot TipBot) error {
	user.UpdatedAt = time.Now()

	// There is a weird bug that makes the AnonID vanish. This is a workaround.
	// TODO -- Remove this after empty anon id bug is identified
	if user.AnonIDSha256 == "" {
		debugStack()
		user.AnonIDSha256 = str.AnonIdSha256(user)
		log.Errorf("[UpdateUserRecord] AnonIDSha256 empty! Setting to: %s", user.AnonIDSha256)
	}
	// TODO -- Remove this after empty anon id bug is identified
	if user.AnonID == "" {
		debugStack()
		user.AnonID = fmt.Sprint(str.Int32Hash(user.ID))
		log.Errorf("[UpdateUserRecord] AnonID empty! Setting to: %s", user.AnonID)
	}
	// TODO -- Remove this after empty anon id bug is identified
	if user.UUID == "" {
		debugStack()
		user.UUID = str.UUIDSha256(user)
		log.Errorf("[UpdateUserRecord] UUID empty! Setting to: %s", user.UUID)
	}

	tx := bot.Database.Save(user)
	if tx.Error != nil {
		errmsg := fmt.Sprintf("[UpdateUserRecord] Error: Couldn't update %s's info in Database.", GetUserStr(user.Telegram))
		log.Errorln(errmsg)
		return tx.Error
	}
	log.Tracef("[UpdateUserRecord] Records of user %s updated.", GetUserStr(user.Telegram))
	if bot.Cache.GoCacheStore != nil {
		updateCachedUser(user, bot)
	}
	return nil
}
