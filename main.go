package main

import (
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
	bot := NewBot()
	bot.Start()
}

func withRecovery() {
	if r := recover(); r != nil {
		log.Errorln("Recovered panic: ", r)
		debug.PrintStack()
	}
}
