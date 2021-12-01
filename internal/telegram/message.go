package telegram

import (
	"strconv"
	"time"

	tb "gopkg.in/lightningtipbot/telebot.v2"
)

type Message struct {
	Message  *tb.Message `json:"message"`
	duration time.Duration
}

type MessageOption func(m *Message)

func WithDuration(duration time.Duration, tipBot *TipBot) MessageOption {
	return func(m *Message) {
		m.duration = duration
		go m.dispose(tipBot)
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

func (msg Message) dispose(tipBot *TipBot) {
	// do not delete messages from private chat
	if msg.Message.Private() {
		return
	}
	go func() {
		time.Sleep(msg.duration)
		tipBot.tryDeleteMessage(msg.Message)
	}()
}
