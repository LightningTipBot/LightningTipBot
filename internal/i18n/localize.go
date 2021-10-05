package i18n

import (
	"github.com/BurntSushi/toml"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

func init() {

}

func RegisterLanguages() *i18n.Bundle {

	bundle := i18n.NewBundle(language.English)
	bundle.RegisterUnmarshalFunc("toml", toml.Unmarshal)
	bundle.MustLoadMessageFile("translations/en.toml")
	bundle.LoadMessageFile("translations/de.toml")
	bundle.LoadMessageFile("translations/it.toml")
	bundle.LoadMessageFile("translations/es.toml")
	bundle.LoadMessageFile("translations/nl.toml")
	return bundle
}
