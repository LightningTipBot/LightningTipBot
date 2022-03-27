package main

import (
	"net/http"
	"runtime/debug"

	"github.com/LightningTipBot/LightningTipBot/internal"
	"github.com/LightningTipBot/LightningTipBot/internal/api"
	"github.com/LightningTipBot/LightningTipBot/internal/api/admin"
	"github.com/LightningTipBot/LightningTipBot/internal/lndhub"
	"github.com/LightningTipBot/LightningTipBot/internal/lnurl"
	"github.com/LightningTipBot/LightningTipBot/internal/runtime/mutex"

	_ "net/http/pprof"

	tb "gopkg.in/lightningtipbot/telebot.v3"

	"github.com/LightningTipBot/LightningTipBot/internal/lnbits/webhook"
	"github.com/LightningTipBot/LightningTipBot/internal/price"
	"github.com/LightningTipBot/LightningTipBot/internal/telegram"
	log "github.com/sirupsen/logrus"
)

// setLogger will initialize the log format
func setLogger() {
	log.SetLevel(log.DebugLevel)
	customFormatter := new(log.TextFormatter)
	customFormatter.TimestampFormat = "2006-01-02 15:04:05"
	customFormatter.FullTimestamp = true
	log.SetFormatter(customFormatter)
}

func main() {
	// set logger
	setLogger()

	defer withRecovery()
	price.NewPriceWatcher().Start()
	bot := telegram.NewBot()
	startApiServer(&bot)
	bot.Start()
}
func startApiServer(bot *telegram.TipBot) {
	// log errors from interceptors
	bot.Telegram.OnError = func(err error, ctx tb.Context) {
		// we already log in the interceptors
	}
	// start internal webhook server
	webhook.NewServer(bot)
	// start external api server
	s := api.NewServer(internal.Configuration.Bot.LNURLServerUrl.Host)

	// append lnurl ctx functions
	lnUrl := lnurl.New(bot)
	s.AppendRoute("/.well-known/lnurlp/{username}", lnUrl.Handle, http.MethodGet)
	s.AppendRoute("/@{username}", lnUrl.Handle, http.MethodGet)

	// append lndhub ctx functions
	hub := lndhub.New(bot)
	s.AppendRoute(`/lndhub/ext/{.*}`, hub.Handle)
	s.AppendRoute(`/lndhub/ext`, hub.Handle)

	// start internal admin server
	adminService := admin.New(bot)
	internalAdminServer := api.NewServer("0.0.0.0:6060")
	internalAdminServer.AppendRoute("/mutex", mutex.ServeHTTP)
	internalAdminServer.AppendRoute("/mutex/unlock/{id}", mutex.UnlockHTTP)
	internalAdminServer.AppendRoute("/admin/ban/{id}", adminService.BanUser)
	internalAdminServer.AppendRoute("/admin/unban/{id}", adminService.UnbanUser)
	internalAdminServer.PathPrefix("/debug/pprof/", http.DefaultServeMux)

}

func withRecovery() {
	if r := recover(); r != nil {
		log.Errorln("Recovered panic: ", r)
		debug.PrintStack()
	}
}
