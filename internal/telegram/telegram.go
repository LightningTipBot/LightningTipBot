package telegram

import (
	log "github.com/sirupsen/logrus"
	tb "gopkg.in/tucnak/telebot.v2"
)

// limitMessageLength limits the length of a message to 500 characters
func limitMessageLength(what interface{}) interface{} {
	// if what is a string, limit it to 500 characters
	if s, ok := what.(string); ok {
		if len(s) > 500 {
			what = s[:500]
		}
	}
	return what
}

func (bot TipBot) tryForwardMessage(to tb.Recipient, what tb.Editable, options ...interface{}) (msg *tb.Message) {
	msg, err := bot.Telegram.Forward(to, what, options...)
	if err != nil {
		log.Warnln(err.Error())
	}
	return
}
func (bot TipBot) trySendMessage(to tb.Recipient, what interface{}, options ...interface{}) (msg *tb.Message) {
	what = limitMessageLength(what)
	msg, err := bot.Telegram.Send(to, what, options...)
	if err != nil {
		log.Warnln(err.Error())
	}
	return
}

func (bot TipBot) tryReplyMessage(to *tb.Message, what interface{}, options ...interface{}) (msg *tb.Message) {
	what = limitMessageLength(what)
	msg, err := bot.Telegram.Reply(to, what, options...)
	if err != nil {
		log.Warnln(err.Error())
	}
	return
}

func (bot TipBot) tryEditMessage(to tb.Editable, what interface{}, options ...interface{}) (msg *tb.Message) {
	what = limitMessageLength(what)
	msg, err := bot.Telegram.Edit(to, what, options...)
	if err != nil {
		log.Warnln(err.Error())
	}
	return
}

func (bot TipBot) tryDeleteMessage(msg tb.Editable) {
	err := bot.Telegram.Delete(msg)
	if err != nil {
		log.Warnln(err.Error())
	}
}
