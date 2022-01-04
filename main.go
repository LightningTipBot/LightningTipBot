package main

import (
	"github.com/LightningTipBot/LightningTipBot/internal/api"
	"github.com/LightningTipBot/LightningTipBot/internal/lnurl"
	"github.com/LightningTipBot/LightningTipBot/internal/runtime/mutex"
	"github.com/gorilla/mux"
	"net/http"
	"runtime/debug"

	_ "net/http/pprof"

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
	// start internal webhook server
	webhook.NewServer(bot)
	// start external api server
	s := api.NewServer()
	lnUrl := lnurl.New(bot)
	s.AppendRoute("/.well-known/lnurlp/{username}", lnUrl.Handle, http.MethodGet)
	s.AppendRoute("/@{username}", lnUrl.Handle, http.MethodGet)

	// start internal admin server
	router := mux.NewRouter()
	router.PathPrefix("/debug/pprof/").Handler(http.DefaultServeMux)
	router.Handle("/mutex", http.HandlerFunc(mutex.ServeHTTP))
	router.Handle("/mutex/unlock/{id}", http.HandlerFunc(mutex.UnlockHTTP))
	go http.ListenAndServe("0.0.0.0:6060", router)
}

func withRecovery() {
	if r := recover(); r != nil {
		log.Errorln("Recovered panic: ", r)
		debug.PrintStack()
	}
}
