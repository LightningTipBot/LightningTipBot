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
	"github.com/LightningTipBot/LightningTipBot/internal/runtime/mutex"
	_ "net/http/pprof"

	tb "gopkg.in/lightningtipbot/telebot.v3"

	"github.com/LightningTipBot/LightningTipBot/internal/lnbits/webhook"
	"github.com/LightningTipBot/LightningTipBot/internal/price"
	"github.com/LightningTipBot/LightningTipBot/internal/telegram"
	log "github.com/sirupsen/logrus"
)

// setLogger will initialize the log format

func main() {

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

	// append lndhub ctx functions
	hub := lndhub.New(bot)
	s.AppendRoute(`/lndhub/ext/{.*}`, hub.Handle)
	s.AppendRoute(`/lndhub/ext`, hub.Handle)
	//s.AppendAuthorizedRoute(`/lndhub/ext/{.*}`, api.AuthTypeBearer, bot.DB.Users, hub.Handle)
	//s.AppendAuthorizedRoute(`/lndhub/ext`, api.AuthTypeBearer, bot.DB.Users, hub.Handle)

	// starting api service
	apiService := api.Service{Bot: bot}
	s.AppendAuthorizedRoute(`/api/v1/paymentstatus/{payment_hash}`, api.AuthTypeBasic, bot.DB.Users, apiService.PaymentStatus, http.MethodPost)
	s.AppendAuthorizedRoute(`/api/v1/invoicestatus/{payment_hash}`, api.AuthTypeBasic, bot.DB.Users, apiService.InvoiceStatus, http.MethodPost)
	s.AppendAuthorizedRoute(`/api/v1/payinvoice`, api.AuthTypeBasic, bot.DB.Users, apiService.PayInvoice, http.MethodPost)
	s.AppendAuthorizedRoute(`/api/v1/createinvoice`, api.AuthTypeBasic, bot.DB.Users, apiService.CreateInvoice, http.MethodPost)
	s.AppendAuthorizedRoute(`/api/v1/balance`, api.AuthTypeBasic, bot.DB.Users, apiService.Balance, http.MethodGet)

	// start internal admin server
	adminService := admin.New(bot)
	internalAdminServer := api.NewServer("0.0.0.0:6060")
	internalAdminServer.AppendRoute("/mutex", mutex.ServeHTTP)
	internalAdminServer.AppendRoute("/mutex/unlock/{id}", mutex.UnlockHTTP)
	internalAdminServer.AppendRoute("/admin/ban/{id}", adminService.BanUser)
	internalAdminServer.AppendRoute("/admin/unban/{id}", adminService.UnbanUser)
	internalAdminServer.PathPrefix("/debug/pprof/", http.DefaultServeMux)

}

func withMiddleware(mw func(w http.ResponseWriter, r *http.Request)) http.HandlerFunc {
	return mw
}
func withRecovery() {
	if r := recover(); r != nil {
		log.Errorln("Recovered panic: ", r)
		debug.PrintStack()
	}
}
