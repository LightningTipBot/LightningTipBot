package main

import (
	"net/http"
	"runtime/debug"

	"github.com/LightningTipBot/LightningTipBot/internal"
	"github.com/LightningTipBot/LightningTipBot/internal/api"
	"github.com/LightningTipBot/LightningTipBot/internal/api/admin"
	"github.com/LightningTipBot/LightningTipBot/internal/api/userpage"
	"github.com/LightningTipBot/LightningTipBot/internal/lndhub"
	"github.com/LightningTipBot/LightningTipBot/internal/lnurl"
	"github.com/LightningTipBot/LightningTipBot/internal/nostr"
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
	// userpage server
	userpage := userpage.New(bot)
	s.AppendRoute("/@{username}", userpage.UserPageHandler, http.MethodGet)
	s.AppendRoute("/app/@{username}", userpage.UserWebAppHandler, http.MethodGet)

	// nostr nip05 identifier
	nostr := nostr.New(bot)
	s.AppendRoute("/.well-known/nostr.json", nostr.Handle, http.MethodGet)

	// append lndhub ctx functions
	hub := lndhub.New(bot)
	s.AppendAuthorizedRoute(`/lndhub/ext/auth`, api.AuthTypeNone, api.AccessKeyTypeNone, bot.DB.Users, hub.Handle)
	s.AppendAuthorizedRoute(`/lndhub/ext/{.*}`, api.AuthTypeBearerBase64, api.AccessKeyTypeAdmin, bot.DB.Users, hub.Handle)
	s.AppendAuthorizedRoute(`/lndhub/ext`, api.AuthTypeBearerBase64, api.AccessKeyTypeAdmin, bot.DB.Users, hub.Handle)

	// starting api service
	apiService := api.Service{Bot: bot}
	s.AppendAuthorizedRoute(`/api/v1/paymentstatus/{payment_hash}`, api.AuthTypeBasic, api.AccessKeyTypeInvoice, bot.DB.Users, apiService.PaymentStatus, http.MethodPost)
	s.AppendAuthorizedRoute(`/api/v1/invoicestatus/{payment_hash}`, api.AuthTypeBasic, api.AccessKeyTypeInvoice, bot.DB.Users, apiService.InvoiceStatus, http.MethodPost)
	s.AppendAuthorizedRoute(`/api/v1/payinvoice`, api.AuthTypeBasic, api.AccessKeyTypeAdmin, bot.DB.Users, apiService.PayInvoice, http.MethodPost)
	s.AppendAuthorizedRoute(`/api/v1/invoicestream`, api.AuthTypeBasic, api.AccessKeyTypeInvoice, bot.DB.Users, apiService.InvoiceStream, http.MethodGet)
	s.AppendAuthorizedRoute(`/api/v1/createinvoice`, api.AuthTypeBasic, api.AccessKeyTypeInvoice, bot.DB.Users, apiService.CreateInvoice, http.MethodPost)
	s.AppendAuthorizedRoute(`/api/v1/balance`, api.AuthTypeBasic, api.AccessKeyTypeInvoice, bot.DB.Users, apiService.Balance, http.MethodGet)

	// start internal admin server
	adminService := admin.New(bot)
	internalAdminServer := api.NewServer(internal.Configuration.Bot.AdminAPIHost)
	internalAdminServer.AppendRoute("/mutex", mutex.ServeHTTP)
	internalAdminServer.AppendRoute("/mutex/unlock/{id}", mutex.UnlockHTTP)
	internalAdminServer.AppendRoute("/admin/ban/{id}", adminService.BanUser)
	internalAdminServer.AppendRoute("/admin/unban/{id}", adminService.UnbanUser)
	internalAdminServer.AppendRoute("/admin/dalle/enable", adminService.EnableDalle)
	internalAdminServer.AppendRoute("/admin/dalle/disable", adminService.DisableDalle)
	internalAdminServer.PathPrefix("/debug/pprof/", http.DefaultServeMux)

}

func withRecovery() {
	if r := recover(); r != nil {
		log.Errorln("Recovered panic: ", r)
		debug.PrintStack()
	}
}
