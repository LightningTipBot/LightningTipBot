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

func (msg Message) Key() string {
	return strconv.Itoa(msg.Message.ID)
}

func (msg Message) dispose(telegram *tb.Bot) {
	// do not delete messages from private chat
	if msg.Message.Private() {
		return
	}
	go func() {
		time.Sleep(msg.duration)
		err := telegram.Delete(msg.Message)
		if err != nil {
			log.Println(err.Error())
			return
		}
	}()
}
