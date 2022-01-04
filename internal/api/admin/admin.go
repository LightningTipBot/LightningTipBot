package admin

import (
	"github.com/LightningTipBot/LightningTipBot/internal/telegram"
	"gorm.io/gorm"
)

type Service struct {
	db *gorm.DB
}

func New(b *telegram.TipBot) Service {
	return Service{
		db: b.Database,
	}
}
