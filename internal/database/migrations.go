package database

import (
	"fmt"

	"github.com/LightningTipBot/LightningTipBot/internal/lnbits"
	"github.com/LightningTipBot/LightningTipBot/internal/str"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

func MigrateAnonIdHash(db *gorm.DB) error {
	users := []lnbits.User{}
	_ = db.Find(&users)
	for _, u := range users {
		log.Info(u.ID, str.Int32Hash(u.ID))
		u.AnonID = fmt.Sprint(str.Int32Hash(u.ID))
		tx := db.Save(u)
		if tx.Error != nil {
			errmsg := fmt.Sprintf("[MigrateAnonIdHash] Error: Couldn't migrate user %s (%d)", u.Telegram.Username, u.Telegram.ID)
			log.Errorln(errmsg)
			return tx.Error
		}
	}
	return nil
}
