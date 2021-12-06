module github.com/LightningTipBot/LightningTipBot

go 1.15

require (
	github.com/BurntSushi/toml v0.3.1
	github.com/eko/gocache v1.2.0
	github.com/fiatjaf/go-lnurl v1.8.4
	github.com/fiatjaf/ln-decodepay v1.1.0
	github.com/gorilla/mux v1.8.0
	github.com/imroc/req v0.3.0
	github.com/jinzhu/configor v1.2.1
	github.com/makiuchi-d/gozxing v0.0.2
	github.com/nicksnyder/go-i18n/v2 v2.1.2
	github.com/patrickmn/go-cache v2.1.0+incompatible
	github.com/sethvargo/go-limiter v0.7.2
	github.com/sirupsen/logrus v1.6.0
	github.com/skip2/go-qrcode v0.0.0-20200617195104-da1b6568686e
	github.com/tidwall/buntdb v1.2.7
	github.com/tidwall/gjson v1.10.2
	golang.org/x/text v0.3.5
	golang.org/x/time v0.0.0-20191024005414-555d28b269f0
	gopkg.in/lightningtipbot/telebot.v2 v2.4.2-0.20211201074627-babf9f2cc955
	gopkg.in/tucnak/telebot.v2 v2.4.1
	gorm.io/driver/sqlite v1.1.4
	gorm.io/gorm v1.21.12
)

// replace gopkg.in/lightningtipbot/telebot.v2 => ../telebot
