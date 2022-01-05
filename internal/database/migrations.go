package database

import (
	"crypto/sha256"
	"fmt"

	"github.com/LightningTipBot/LightningTipBot/internal/lnbits"
	"github.com/LightningTipBot/LightningTipBot/internal/str"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

func MigrateAnonIdInt32Hash(db *gorm.DB) error {
	users := []lnbits.User{}
	_ = db.Find(&users)
	for _, u := range users {
		log.Infof("[MigrateAnonIdInt32Hash] %d -> %d", u.ID, str.Int32Hash(u.ID))
		u.AnonID = fmt.Sprint(str.Int32Hash(u.ID))
		tx := db.Save(u)
		if tx.Error != nil {
			errmsg := fmt.Sprintf("[MigrateAnonIdInt32Hash] Error: Couldn't migrate user %s (%d)", u.Telegram.Username, u.Telegram.ID)
			log.Errorln(errmsg)
			return tx.Error
		}
	}
	return nil
}

func MigrateAnonIdSha265Hash(db *gorm.DB) error {
	users := []lnbits.User{}
	_ = db.Find(&users)
	for _, u := range users {
		pw := u.Wallet.ID
		h := sha256.Sum256([]byte(u.Wallet.ID))
		hash := fmt.Sprintf("%x", h)
		anon_id := fmt.Sprintf("0x%s", hash[:16]) // starts with 0x because that can't be a valid telegram username
		log.Infof("[MigrateAnonIdSha265Hash] %s -> %s", pw, anon_id)
		u.AnonIDSha256 = anon_id
		tx := db.Save(u)
		if tx.Error != nil {
			errmsg := fmt.Sprintf("[MigrateAnonIdSha265Hash] Error: Couldn't migrate user %s (%s)", u.Telegram.Username, pw)
			log.Errorln(errmsg)
			return tx.Error
		}
	}
	return nil
}
