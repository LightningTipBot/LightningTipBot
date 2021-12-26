package main

import (
	"net/http"
	"runtime/debug"

	"github.com/LightningTipBot/LightningTipBot/internal/runtime/mutex"
	"github.com/gorilla/mux"

	_ "net/http/pprof"

	"github.com/LightningTipBot/LightningTipBot/internal/lnbits/webhook"
	"github.com/LightningTipBot/LightningTipBot/internal/lnurl"
	"github.com/LightningTipBot/LightningTipBot/internal/price"
	"github.com/LightningTipBot/LightningTipBot/internal/telegram"
	log "github.com/sirupsen/logrus"
)

// setLogger will initialize the log format
func setLogger() {
	log.SetLevel(log.InfoLevel)
	customFormatter := new(log.TextFormatter)
	customFormatter.TimestampFormat = "2006-01-02 15:04:05"
	customFormatter.FullTimestamp = true
	log.SetFormatter(customFormatter)
}

func main() {
	// set logger
	setLogger()
	router := mux.NewRouter()
	router.PathPrefix("/debug/pprof/").Handler(http.DefaultServeMux)
	router.Handle("/mutex", http.HandlerFunc(mutex.ServeHTTP))
	router.Handle("/mutex/unlock/{id}", http.HandlerFunc(mutex.UnlockHTTP))
	go http.ListenAndServe("0.0.0.0:6060", router)
	defer withRecovery()
	bot := telegram.NewBot()
	webhook.NewServer(&bot)
	lnurl.NewServer(&bot)
	price.NewPriceWatcher().Start()
	bot.Start()
}

func withRecovery() {
	if r := recover(); r != nil {
		log.Errorln("Recovered panic: ", r)
		debug.PrintStack()
	}
}
