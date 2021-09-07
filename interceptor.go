package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/LightningTipBot/LightningTipBot/internal/lnbits"
	log "github.com/sirupsen/logrus"
	tb "gopkg.in/tucnak/telebot.v2"
	"gorm.io/gorm"
)

// updateUserInterceptor Update the telegram user with message intercept
func (bot TipBot) updateUserInterceptor(ctx context.Context, m *tb.Message) context.Context {
	user, err := GetUser(m.Sender, bot)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		u := &lnbits.User{Telegram: m.Sender, Initialized: true}
		err := bot.createWallet(u)
		if err != nil {
			return ctx
		}
		err = UpdateUserRecord(u, bot)
		if err != nil {
			log.Errorln(fmt.Sprintf("[UpdateUserRecord] error updating user: %s", err.Error()))
		}
	}
	return context.WithValue(ctx, "user", user)
}

// loadUserQueryInterceptor Loading the telegram user with query intercept
func (bot TipBot) loadUserQueryInterceptor(ctx context.Context, c *tb.Query) context.Context {
	user, err := GetUser(&c.From, bot)
	if err != nil {
		return ctx
	}
	return context.WithValue(ctx, "user", user)
}

// loadUserCallbackInterceptor Loading the telegram user with callback intercept
func (bot TipBot) loadUserCallbackInterceptor(ctx context.Context, c *tb.Callback) context.Context {
	m := *c.Message
	m.Sender = c.Sender
	return bot.loadUserInterceptor(ctx, &m)
}

// loadReplyToInterceptor Loading the telegram user with message intercept
func (bot TipBot) loadReplyToInterceptor(ctx context.Context, m *tb.Message) context.Context {
	if m.ReplyTo.Sender != nil {
		user, _ := GetUser(m.ReplyTo.Sender, bot)
		user.Telegram = m.ReplyTo.Sender
		return context.WithValue(ctx, "reply_to_user", user)
	}
	return context.Background()
}

// loadUserInterceptor Loading the telegram user with message intercept
func (bot TipBot) loadUserInterceptor(ctx context.Context, m *tb.Message) context.Context {
	user, err := GetUser(m.Sender, bot)
	if err != nil {
		return ctx
	}
	return context.WithValue(ctx, "user", user)
}
