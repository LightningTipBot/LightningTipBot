package main

import (
	"strconv"
	"time"

	log "github.com/sirupsen/logrus"

	tb "gopkg.in/tucnak/telebot.v2"
)

type Message struct {
	Message   *tb.Message
	TipAmount int
	Ntips     int
	LastTip   time.Time
	Tippers   []*tb.User
}

func (m Message) Index() string {
	return "messages"
}
func (m Message) Key() string {
	return strconv.Itoa(m.Message.ID)
}

const maxNamesInTipperMessage = 5

type messageOption func(m *Message)

func TipAmount(amount int) messageOption {
	return func(m *Message) {
		m.TipAmount = amount
	}
}
func Tips(nTips int) messageOption {
	return func(m *Message) {
		m.LastTip = time.Now()
		m.Ntips = nTips
	}
}

func NewMessage(m *tb.Message, opts ...messageOption) *Message {
	msg := &Message{
		Message: m,
		Tippers: make([]*tb.User, 0),
	}
	for _, opt := range opts {
		opt(msg)
	}
	return msg

}

func (x Message) Dispose(duration time.Duration, telegram *tb.Bot) {
	// do not delete messages from private chat
	if x.Message.Private() {
		return
	}
	go func() {
		time.Sleep(duration)
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
