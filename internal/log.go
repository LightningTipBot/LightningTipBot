package internal

import (
	log "github.com/sirupsen/logrus"
	"go.elastic.co/ecslogrus"
	"gopkg.in/natefinch/lumberjack.v2"
	"io"
)

func init() {
	stdoutLogger := log.New()
	customFormatter := new(log.TextFormatter)
	customFormatter.FullTimestamp = true
	stdoutLogger.SetFormatter(customFormatter)

	log.SetLevel(log.DebugLevel)
	log.SetFormatter(&ecslogrus.Formatter{})

	log.SetOutput(io.MultiWriter(stdoutLogger.Out, &lumberjack.Logger{
		Filename:   "out.log",
		MaxSize:    1, // megabytes
		MaxBackups: 3,
		MaxAge:     28,
		Compress:   true}))

}
