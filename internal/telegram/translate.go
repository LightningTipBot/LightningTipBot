package telegram

import (
	"context"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	log "github.com/sirupsen/logrus"
)

func Translate(ctx context.Context, MessgeID string) string {
	str, err := LoadPublicLocalizer(ctx).Localize(&i18n.LocalizeConfig{MessageID: MessgeID})
	if err != nil {
		log.Warnf("Error translating message %s: %s", MessgeID, err)
	}
	return str
}

func TranslateUser(ctx context.Context, MessgeID string) string {
	str, err := LoadUserLocalizer(ctx).Localize(&i18n.LocalizeConfig{MessageID: MessgeID})
	if err != nil {
		log.Warnf("Error translating message %s: %s", MessgeID, err)
	}
	return str
}
