package main

import (
	"context"
	"fmt"
	"github.com/LightningTipBot/LightningTipBot/internal/lnbits"
	"github.com/LightningTipBot/LightningTipBot/internal/telegram/intercept"
	tb "gopkg.in/tucnak/telebot.v2"
)

type InterceptorType int

const (
	MessageInterceptor InterceptorType = iota
	CallbackInterceptor
	QueryInterceptor
)

var invalidTypeError = fmt.Errorf("invalid type")

type Interceptor struct {
	Type   InterceptorType
	Before []intercept.Func
	After  []intercept.Func
}

func (bot TipBot) requireUserInterceptor(ctx context.Context, i interface{}) (context.Context, error) {
	switch i.(type) {
	case *tb.Query:
		user, err := GetUser(&i.(*tb.Query).From, bot)
		return context.WithValue(ctx, "user", user), err
	case *tb.Callback:
		c := i.(*tb.Callback)
		m := *c.Message
		m.Sender = c.Sender
		user, err := GetUser(i.(*tb.Callback).Sender, bot)
		return context.WithValue(ctx, "user", user), err
	case *tb.Message:
		user, err := GetUser(i.(*tb.Message).Sender, bot)
		return context.WithValue(ctx, "user", user), err
	}
	return nil, invalidTypeError
}
func (bot TipBot) loadUserInterceptor(ctx context.Context, i interface{}) (context.Context, error) {
	ctx, _ = bot.requireUserInterceptor(ctx, i)
	return ctx, nil
}

// loadReplyToInterceptor Loading the telegram user with message intercept
func (bot TipBot) loadReplyToInterceptor(ctx context.Context, i interface{}) (context.Context, error) {
	switch i.(type) {
	case *tb.Message:
		m := i.(*tb.Message)
		if m.ReplyTo != nil {
			if m.ReplyTo.Sender != nil {
				user, _ := GetUser(m.ReplyTo.Sender, bot)
				user.Telegram = m.ReplyTo.Sender
				return context.WithValue(ctx, "reply_to_user", user), nil
			}
		}
		return context.Background(), fmt.Errorf("reply to user not found")
	}
	return nil, invalidTypeError
}

func (bot TipBot) requirePrivateChatInterceptor(ctx context.Context, i interface{}) (context.Context, error) {
	switch i.(type) {
	case *tb.Message:
		m := i.(*tb.Message)
		if m.Chat.Type != tb.ChatPrivate {
			return nil, fmt.Errorf("no private chat")
		}
		return ctx, nil
	}
	return nil, invalidTypeError
}

// LoadUser from context
func LoadUser(ctx context.Context) *lnbits.User {
	u := ctx.Value("user")
	if u != nil {
		return u.(*lnbits.User)
	}
	return nil
}

// LoadReplyToUser from context
func LoadReplyToUser(ctx context.Context) *lnbits.User {
	u := ctx.Value("reply_to_user")
	if u != nil {
		return u.(*lnbits.User)
	}
	return nil
}
