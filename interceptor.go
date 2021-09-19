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

type Interceptor struct {
	Type           InterceptorType
	BeforeMessage  []intercept.MessageFunc
	AfterMessage   []intercept.MessageFunc
	BeforeQuery    []intercept.QueryFunc
	AfterQuery     []intercept.QueryFunc
	BeforeCallback []intercept.CallbackFunc
	AfterCallback  []intercept.CallbackFunc
}

// requireUserQueryInterceptor Loading the telegram user with query intercept
func (bot TipBot) requireUserQueryInterceptor(ctx context.Context, c *tb.Query) (context.Context, error) {
	user, err := GetUser(&c.From, bot)
	if err != nil {
		return ctx, err
	}
	return context.WithValue(ctx, "user", user), nil
}

// loadUserCallbackInterceptor Loading the telegram user with callback intercept
func (bot TipBot) loadUserCallbackInterceptor(ctx context.Context, c *tb.Callback) (context.Context, error) {
	m := *c.Message
	m.Sender = c.Sender
	return bot.loadUserInterceptor(ctx, &m)
}

// loadReplyToInterceptor Loading the telegram user with message intercept
func (bot TipBot) loadReplyToInterceptor(ctx context.Context, m *tb.Message) (context.Context, error) {
	if m.ReplyTo != nil {
		if m.ReplyTo.Sender != nil {
			user, _ := GetUser(m.ReplyTo.Sender, bot)
			user.Telegram = m.ReplyTo.Sender
			return context.WithValue(ctx, "reply_to_user", user), nil
		}
	}
	return context.Background(), fmt.Errorf("reply to user not found")
}

// loadUserInterceptor Loading the telegram into lnbits user. will create user without wallet, if user is not found.
func (bot TipBot) loadUserInterceptor(ctx context.Context, m *tb.Message) (context.Context, error) {
	ctx, _ = bot.requireUserInterceptor(ctx, m)
	return ctx, nil
}

// requireUserInterceptor Loading the telegram into lnbits user. will not invoke the handler if user is not found.
func (bot TipBot) requireUserInterceptor(ctx context.Context, m *tb.Message) (context.Context, error) {
	user, err := GetUser(m.Sender, bot)
	return context.WithValue(ctx, "user", user), err
}

func (bot TipBot) requirePrivateChatInterceptor(ctx context.Context, m *tb.Message) (context.Context, error) {
	if m.Chat.Type != tb.ChatPrivate {
		return nil, fmt.Errorf("no private chat")
	}
	return ctx, nil
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
	u := ctx.Value("user")
	if u != nil {
		return u.(*lnbits.User)
	}
	return nil
}
