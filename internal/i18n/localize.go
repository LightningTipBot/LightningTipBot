package i18n

import (
	"github.com/BurntSushi/toml"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	log "github.com/sirupsen/logrus"
	"golang.org/x/text/language"
)

var Bundle *i18n.Bundle

func init() {
	Bundle = RegisterLanguages()
}

func RegisterLanguages() *i18n.Bundle {
	bundle := i18n.NewBundle(language.English)
	bundle.RegisterUnmarshalFunc("toml", toml.Unmarshal)
	bundle.MustLoadMessageFile("translations/en.toml")
	bundle.LoadMessageFile("translations/de.toml")
	bundle.LoadMessageFile("translations/it.toml")
	bundle.LoadMessageFile("translations/es.toml")
	bundle.LoadMessageFile("translations/nl.toml")
	bundle.LoadMessageFile("translations/fr.toml")
	bundle.LoadMessageFile("translations/pt-br.toml")
	bundle.LoadMessageFile("translations/tr.toml")
	bundle.LoadMessageFile("translations/id.toml")
	return bundle
}
func Translate(languageCode string, MessgeID string) string {
	str, err := i18n.NewLocalizer(Bundle, languageCode).Localize(&i18n.LocalizeConfig{MessageID: MessgeID})
	if err != nil {
		log.Warnf("Error translating message %s: %s", MessgeID, err)
	}
	return str
}
