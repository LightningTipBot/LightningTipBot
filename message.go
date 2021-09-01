package main

import (
	"strconv"
	"time"

	log "github.com/sirupsen/logrus"

	tb "gopkg.in/tucnak/telebot.v2"
)

type Message struct {
	Message  *tb.Message `json:"message"`
	duration time.Duration
}

const maxNamesInTipperMessage = 5

type MessageOption func(m *Message)

func WithDuration(duration time.Duration, bot *tb.Bot) MessageOption {
	return func(m *Message) {
		m.duration = duration
		go m.dispose(bot)
	}
}

func NewMessage(m *tb.Message, opts ...MessageOption) *Message {
	msg := &Message{
		Message: m,
	}
	for _, opt := range opts {
		opt(msg)
	}
	return msg
}

func (m Message) Key() string {
	if m.Message.ReplyTo != nil {
		return strconv.Itoa(m.Message.ReplyTo.ID)
	}
	return strconv.Itoa(m.Message.ID)
}

func (x Message) dispose(telegram *tb.Bot) {
	// do not delete messages from private chat
	if x.Message.Private() {
		return
	}
	go func() {
		time.Sleep(x.duration)
		err := telegram.Delete(x.Message)
		if err != nil {
			log.Println(err.Error())
			return
		}
	}()
}

func removeMessage(messages []*Message, s int) []*Message {
	if len(messages) == 1 {
		return make([]*Message, 0)
	}
	return append(messages[:s], messages[s+1:]...)
}
