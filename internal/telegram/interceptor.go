package telegram

import (
	"context"
	"fmt"
	"sync"

	"github.com/LightningTipBot/LightningTipBot/internal/i18n"
	i18n2 "github.com/nicksnyder/go-i18n/v2/i18n"

	"github.com/LightningTipBot/LightningTipBot/internal/lnbits"
	"github.com/LightningTipBot/LightningTipBot/internal/telegram/intercept"
	log "github.com/sirupsen/logrus"
	tb "gopkg.in/lightningtipbot/telebot.v2"
)

type InterceptorType int

const (
	MessageInterceptor InterceptorType = iota
	CallbackInterceptor
	QueryInterceptor
)

func init() {
	handlerMutex = make(map[int64]*sync.Mutex)
	handlerMapMutex = &sync.Mutex{}
}

var invalidTypeError = fmt.Errorf("invalid type")

type Interceptor struct {
	Type    InterceptorType
	Before  []intercept.Func
	After   []intercept.Func
	OnDefer []intercept.Func
}

// handlerMapMutex to prevent concurrent map read / writes on HandlerMutex map
var handlerMapMutex *sync.Mutex

// handlerMutex map holds mutex for every telegram user. Mutex locket as first before interceptor and unlocked on defer intercept
var handlerMutex map[int64]*sync.Mutex

// unlockInterceptor invoked as onDefer interceptor
func (bot TipBot) unlockInterceptor(ctx context.Context, i interface{}) (context.Context, error) {
	user := getTelegramUserFromInterface(i)
	if user != nil {
		handlerMapMutex.Lock()
		if handlerMutex[user.ID] != nil {
			handlerMutex[user.ID].Unlock()
		}
		handlerMapMutex.Unlock()
	}
	return ctx, nil
}

// lockInterceptor invoked as first before interceptor
func (bot TipBot) lockInterceptor(ctx context.Context, i interface{}) (context.Context, error) {
	user := getTelegramUserFromInterface(i)
	if user != nil {
		handlerMapMutex.Lock()
		if handlerMutex[user.ID] == nil {
			handlerMutex[user.ID] = &sync.Mutex{}
		}
		handlerMapMutex.Unlock()
		handlerMutex[user.ID].Lock()
		return ctx, nil
	}
	return nil, invalidTypeError
}

// requireUserInterceptor will return an error if user is not found
// user is here an lnbits.User
func (bot TipBot) requireUserInterceptor(ctx context.Context, i interface{}) (context.Context, error) {
	var user *lnbits.User
	var err error
	u := getTelegramUserFromInterface(i)
	if u != nil {
		user, err = GetUser(u, bot)
		if user != nil {
			return context.WithValue(ctx, "user", user), err
		}
	}
	return nil, invalidTypeError
}

func (bot TipBot) loadUserInterceptor(ctx context.Context, i interface{}) (context.Context, error) {
	ctx, _ = bot.requireUserInterceptor(ctx, i)
	return ctx, nil
}

// getTelegramUserFromInterface returns the tb user based in interface type
func getTelegramUserFromInterface(i interface{}) (user *tb.User) {
	switch i.(type) {
	case *tb.Query:
		user = &i.(*tb.Query).From
	case *tb.Callback:
		user = i.(*tb.Callback).Sender
	case *tb.Message:
		user = i.(*tb.Message).Sender
	}
	return
}

// loadReplyToInterceptor Loading the Telegram user with message intercept
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
		return ctx, nil
	}
	return ctx, invalidTypeError
}

func (bot TipBot) localizerInterceptor(ctx context.Context, i interface{}) (context.Context, error) {
	var userLocalizer *i18n2.Localizer
	var publicLocalizer *i18n2.Localizer

	// default language is english
	publicLocalizer = i18n2.NewLocalizer(i18n.Bundle, "en")
	ctx = context.WithValue(ctx, "publicLanguageCode", "en")
	ctx = context.WithValue(ctx, "publicLocalizer", publicLocalizer)

	switch i.(type) {
	case *tb.Message:
		m := i.(*tb.Message)
		userLocalizer = i18n2.NewLocalizer(i18n.Bundle, m.Sender.LanguageCode)
		ctx = context.WithValue(ctx, "userLanguageCode", m.Sender.LanguageCode)
		ctx = context.WithValue(ctx, "userLocalizer", userLocalizer)
		if m.Private() {
			// in pm overwrite public localizer with user localizer
			ctx = context.WithValue(ctx, "publicLanguageCode", m.Sender.LanguageCode)
			ctx = context.WithValue(ctx, "publicLocalizer", userLocalizer)
		}
		return ctx, nil
	case *tb.Callback:
		m := i.(*tb.Callback)
		userLocalizer = i18n2.NewLocalizer(i18n.Bundle, m.Sender.LanguageCode)
		ctx = context.WithValue(ctx, "userLanguageCode", m.Sender.LanguageCode)
		return context.WithValue(ctx, "userLocalizer", userLocalizer), nil
	case *tb.Query:
		m := i.(*tb.Query)
		userLocalizer = i18n2.NewLocalizer(i18n.Bundle, m.From.LanguageCode)
		ctx = context.WithValue(ctx, "userLanguageCode", m.From.LanguageCode)
		return context.WithValue(ctx, "userLocalizer", userLocalizer), nil
	}
	return ctx, nil
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

const photoTag = "<Photo>"

func (bot TipBot) logMessageInterceptor(ctx context.Context, i interface{}) (context.Context, error) {
	switch i.(type) {
	case *tb.Message:
		m := i.(*tb.Message)
		if m.Text != "" {
			log_string := fmt.Sprintf("[%s:%d %s:%d] %s", m.Chat.Title, m.Chat.ID, GetUserStr(m.Sender), m.Sender.ID, m.Text)
			if m.IsReply() {
				log_string = fmt.Sprintf("%s -> %s", log_string, GetUserStr(m.ReplyTo.Sender))
			}
			log.Infof(log_string)
		} else if m.Photo != nil {
			log.Infof("[%s:%d %s:%d] %s", m.Chat.Title, m.Chat.ID, GetUserStr(m.Sender), m.Sender.ID, photoTag)
		}
		return ctx, nil
	case *tb.Callback:
		m := i.(*tb.Callback)
		log.Infof("[Callback %s:%d] Data: %s", GetUserStr(m.Sender), m.Sender.ID, m.Data)
		return ctx, nil
	}
	return nil, invalidTypeError
}

// LoadUser from context
func LoadUserLocalizer(ctx context.Context) *i18n2.Localizer {
	u := ctx.Value("userLocalizer")
	if u != nil {
		return u.(*i18n2.Localizer)
	}
	return nil
}

// LoadUser from context
func LoadPublicLocalizer(ctx context.Context) *i18n2.Localizer {
	u := ctx.Value("publicLocalizer")
	if u != nil {
		return u.(*i18n2.Localizer)
	}
	return nil
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
