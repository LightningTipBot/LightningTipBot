package main

import (
	log "github.com/sirupsen/logrus"
	tb "gopkg.in/tucnak/telebot.v2"
)

func (bot TipBot) tryForwardMessage(to tb.Recipient, what tb.Editable, options ...interface{}) (msg *tb.Message) {
	msg, err := bot.telegram.Forward(to, what, options...)
	if err != nil {
		log.Errorln(err.Error())
	}
	return
}
func (bot TipBot) trySendMessage(to tb.Recipient, what interface{}, options ...interface{}) (msg *tb.Message) {
	msg, err := bot.telegram.Send(to, what, options...)
	if err != nil {
		log.Errorln(err.Error())
	}
	return
}

func (bot TipBot) tryReplyMessage(to *tb.Message, what interface{}, options ...interface{}) (msg *tb.Message) {
	msg, err := bot.telegram.Reply(to, what, options...)
	if err != nil {
		log.Errorln(err.Error())
	}
	return
}

func (bot TipBot) tryEditMessage(to tb.Editable, what interface{}, options ...interface{}) (msg *tb.Message) {
	msg, err := bot.telegram.Edit(to, what, options...)
	if err != nil {
		log.Errorln(err.Error())
	}
	return
}

func (bot TipBot) tryDeleteMessage(msg tb.Editable) {
	err := bot.telegram.Delete(msg)
	if err != nil {
		log.Errorln(err.Error())
	}
}
