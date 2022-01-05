package admin

import (
	"github.com/LightningTipBot/LightningTipBot/internal/telegram"
)

type Service struct {
	bot *telegram.TipBot
}

func New(b *telegram.TipBot) Service {
	return Service{
		bot: b,
	}
}
