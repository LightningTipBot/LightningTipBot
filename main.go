package main

import (
	"github.com/LightningTipBot/LightningTipBot/internal/lnbits/webhook"
	"github.com/LightningTipBot/LightningTipBot/internal/lnurl"
	"github.com/LightningTipBot/LightningTipBot/internal/telegram"
	log "github.com/sirupsen/logrus"
	"runtime/debug"
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
	defer withRecovery()
	bot := telegram.NewBot()
	webhook.NewServer(&bot)
	lnurl.NewServer(&bot)
	bot.Start()
}

func withRecovery() {
	if r := recover(); r != nil {
		log.Errorln("Recovered panic: ", r)
		debug.PrintStack()
	}
}
