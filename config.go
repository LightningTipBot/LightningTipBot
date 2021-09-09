package main

import (
	"fmt"
	"github.com/jinzhu/configor"
	log "github.com/sirupsen/logrus"
	"net/url"
	"strings"
)

var Configuration = struct {
	Bot      BotConfiguration      `yaml:"bot"`
	Telegram TelegramConfiguration `yaml:"telegram"`
	Database DatabaseConfiguration `yaml:"database"`
	Lnbits   LnbitsConfiguration   `yaml:"lnbits"`
}{}

type BotConfiguration struct {
	HttpProxy      string   `yaml:"http_proxy"`
	LNURLServer    string   `yaml:"lnurl_server"`
	LNURLServerUrl *url.URL `yaml:"-"`
	LNURLHostName  string   `yaml:"lnurl_public_host_name"`
	LNURLHostUrl   *url.URL `yaml:"-"`
}

type TelegramConfiguration struct {
	MessageDisposeDuration int64  `yaml:"message_dispose_duration"`
	ApiKey                 string `yaml:"api_key"`
}
type DatabaseConfiguration struct {
	DbPath           string `yaml:"db_path"`
	BuntDbPath       string `yaml:"buntdb_path"`
	TransactionsPath string `yaml:"transactions_path"`
}

type LnbitsConfiguration struct {
	AdminId          string   `yaml:"admin_id"`
	AdminKey         string   `yaml:"admin_key"`
	Url              string   `yaml:"url"`
	LnbitsPublicUrl  string   `yaml:"lnbits_public_url"`
	WebhookServer    string   `yaml:"webhook_server"`
	WebhookServerUrl *url.URL `yaml:"-"`
}

func init() {
	err := configor.Load(&Configuration, "config.yaml")
	if err != nil {
		panic(err)
	}
	webhookUrl, err := url.Parse(Configuration.Lnbits.WebhookServer)
	if err != nil {
		panic(err)
	}
	Configuration.Lnbits.WebhookServerUrl = webhookUrl

	lnUrl, err := url.Parse(Configuration.Bot.LNURLServer)
	if err != nil {
		panic(err)
	}
	Configuration.Bot.LNURLServerUrl = lnUrl
	hostname, err := url.Parse(Configuration.Bot.LNURLHostName)
	if err != nil {
		panic(err)
	}
	Configuration.Bot.LNURLHostUrl = hostname
	checkLnbitsConfiguration()
}

func checkLnbitsConfiguration() {
	if Configuration.Lnbits.Url == "" {
		panic(fmt.Errorf("please configure a lnbits url"))
	}
	if Configuration.Lnbits.LnbitsPublicUrl == "" {
		log.Warnf("Please specify a lnbits public url otherwise users won't be able to")
	} else {
		if !strings.HasSuffix(Configuration.Lnbits.LnbitsPublicUrl, "/") {
			Configuration.Lnbits.LnbitsPublicUrl = Configuration.Lnbits.LnbitsPublicUrl + "/"
		}
	}
}
