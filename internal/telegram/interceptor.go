package telegram

import (
	"context"
	"fmt"
	"strconv"

	"github.com/LightningTipBot/LightningTipBot/internal/errors"

	"github.com/LightningTipBot/LightningTipBot/internal/runtime/mutex"
	"github.com/LightningTipBot/LightningTipBot/internal/runtime/once"

	"github.com/LightningTipBot/LightningTipBot/internal/i18n"
	i18n2 "github.com/nicksnyder/go-i18n/v2/i18n"

	"github.com/LightningTipBot/LightningTipBot/internal/lnbits"
	"github.com/LightningTipBot/LightningTipBot/internal/telegram/intercept"
	log "github.com/sirupsen/logrus"
	tb "gopkg.in/telebot.v3"
)

type Interceptor struct {
	Before  []intercept.Func
	After   []intercept.Func
	OnDefer []intercept.Func
}

// singletonClickInterceptor uses the onceMap to determine whether the object k1 already interacted
// with the user k2. If so, it will return an error.
func (bot TipBot) singletonCallbackInterceptor(handler intercept.Handler) (intercept.Handler, error) {
	if handler.Callback() != nil {
		return handler, once.Once(handler.Callback().Data, strconv.FormatInt(handler.Callback().Sender.ID, 10))
	}
	return handler, errors.Create(errors.InvalidTypeError)
}

// lockInterceptor invoked as first before interceptor
func (bot TipBot) lockInterceptor(handler intercept.Handler) (intercept.Handler, error) {
	user := handler.Sender()
	if user != nil {
		mutex.Lock(strconv.FormatInt(user.ID, 10))
		return handler, nil
	}
	return handler, errors.Create(errors.InvalidTypeError)
}

// unlockInterceptor invoked as onDefer interceptor
func (bot TipBot) unlockInterceptor(handler intercept.Handler) (intercept.Handler, error) {
	user := handler.Sender()
	if user != nil {
		mutex.Unlock(strconv.FormatInt(user.ID, 10))
		return handler, nil
	}
	return handler, errors.Create(errors.InvalidTypeError)
}
func (bot TipBot) idInterceptor(handler intercept.Handler) (intercept.Handler, error) {
	handler.Ctx = context.WithValue(handler.Ctx, "uid", RandStringRunes(64))
	return handler, nil
}

// answerCallbackInterceptor will answer the callback with the given text in the context
func (bot TipBot) answerCallbackInterceptor(ctx context.Context, i interface{}) (context.Context, error) {
	switch i.(type) {
	case *tb.Callback:
		c := i.(*tb.Callback)
		ctxcr := ctx.Value("callback_response")
		var res []*tb.CallbackResponse
		if ctxcr != nil {
			res = append(res, &tb.CallbackResponse{CallbackID: c.ID, Text: ctxcr.(string)})
		}
		// if the context wasn't set, still respond with an empty callback response
		if len(res) == 0 {
			res = append(res, &tb.CallbackResponse{CallbackID: c.ID, Text: ""})
		}
		var err error
		err = bot.Telegram.Respond(c, res...)
		return ctx, err
	}
	return ctx, errors.Create(errors.InvalidTypeError)
}

// requireUserInterceptor will return an error if user is not found
// user is here an lnbits.User
func (bot TipBot) requireUserInterceptor(handler intercept.Handler) (intercept.Handler, error) {
	var user *lnbits.User
	var err error
	u := handler.Sender()
	if u != nil {
		user, err = GetUser(u, bot)
		// do not respond to banned users
		if bot.UserIsBanned(user) {
			handler.Ctx = context.WithValue(handler.Ctx, "banned", true)
			handler.Ctx = context.WithValue(handler.Ctx, "user", user)
			return handler, errors.Create(errors.InvalidTypeError)
		}
		if user != nil {
			handler.Ctx = context.WithValue(handler.Ctx, "user", user)
			return handler, err
		}
	}
	return handler, errors.Create(errors.InvalidTypeError)
}

// startUserInterceptor will invoke /start if user not exists.
func (bot TipBot) startUserInterceptor(handler intercept.Handler) (intercept.Handler, error) {
	handler, err := bot.loadUserInterceptor(handler)
	if err != nil {
		// user banned
		return handler, err
	}
	// load user
	u := handler.Ctx.Value("user")
	// check user nil
	if u != nil {
		user := u.(*lnbits.User)
		// check wallet nil or !initialized
		if user.Wallet == nil || !user.Initialized {
			handler, err = bot.startHandler(handler)
			if err != nil {
				return handler, err
			}
			return handler, nil
		}
	}
	return handler, nil
}
func (bot TipBot) loadUserInterceptor(handler intercept.Handler) (intercept.Handler, error) {
	handler, _ = bot.requireUserInterceptor(handler)
	// if user is banned, also loadUserInterceptor will return an error
	if handler.Ctx.Value("banned") != nil && handler.Ctx.Value("banned").(bool) {
		return handler, errors.Create(errors.InvalidTypeError)
	}
	return handler, nil
}

// loadReplyToInterceptor Loading the Telegram user with message intercept
func (bot TipBot) loadReplyToInterceptor(handler intercept.Handler) (intercept.Handler, error) {
	if handler.Message() != nil {
		if handler.Message().ReplyTo != nil {
			if handler.Message().ReplyTo.Sender != nil {
				user, _ := GetUser(handler.Message().ReplyTo.Sender, bot)
				user.Telegram = handler.Message().ReplyTo.Sender
				handler.Ctx = context.WithValue(handler.Ctx, "reply_to_user", user)
				return handler, nil

			}
		}
		return handler, nil
	}
	return handler, errors.Create(errors.InvalidTypeError)
}

func (bot TipBot) localizerInterceptor(handler intercept.Handler) (intercept.Handler, error) {
	var userLocalizer *i18n2.Localizer
	var publicLocalizer *i18n2.Localizer

	// default language is english
	publicLocalizer = i18n2.NewLocalizer(i18n.Bundle, "en")
	handler.Ctx = context.WithValue(handler.Ctx, "publicLanguageCode", "en")
	handler.Ctx = context.WithValue(handler.Ctx, "publicLocalizer", publicLocalizer)

	if handler.Message() != nil {
		userLocalizer = i18n2.NewLocalizer(i18n.Bundle, handler.Message().Sender.LanguageCode)
		handler.Ctx = context.WithValue(handler.Ctx, "userLanguageCode", handler.Message().Sender.LanguageCode)
		handler.Ctx = context.WithValue(handler.Ctx, "userLocalizer", userLocalizer)
		if handler.Message().Private() {
			// in pm overwrite public localizer with user localizer
			handler.Ctx = context.WithValue(handler.Ctx, "publicLanguageCode", handler.Message().Sender.LanguageCode)
			handler.Ctx = context.WithValue(handler.Ctx, "publicLocalizer", userLocalizer)
		}
		return handler, nil
	} else if handler.Callback() != nil {
		userLocalizer = i18n2.NewLocalizer(i18n.Bundle, handler.Callback().Sender.LanguageCode)
		handler.Ctx = context.WithValue(handler.Ctx, "userLanguageCode", handler.Callback().Sender.LanguageCode)
		handler.Ctx = context.WithValue(handler.Ctx, "userLocalizer", userLocalizer)
		return handler, nil
	} else if handler.Query() != nil {
		userLocalizer = i18n2.NewLocalizer(i18n.Bundle, handler.Query().Sender.LanguageCode)
		handler.Ctx = context.WithValue(handler.Ctx, "userLanguageCode", handler.Query().Sender.LanguageCode)
		handler.Ctx = context.WithValue(handler.Ctx, "userLocalizer", userLocalizer)
		return handler, nil
	}

	return handler, nil
}

func (bot TipBot) requirePrivateChatInterceptor(handler intercept.Handler) (intercept.Handler, error) {
	if handler.Message() != nil {
		if handler.Message().Chat.Type != tb.ChatPrivate {
			return handler, fmt.Errorf("[requirePrivateChatInterceptor] no private chat")
		}
		return handler, nil
	}
	return handler, errors.Create(errors.InvalidTypeError)
}

const photoTag = "<Photo>"

func (bot TipBot) logMessageInterceptor(handler intercept.Handler) (intercept.Handler, error) {
	if handler.Message() != nil {

		if handler.Message().Text != "" {
			log_string := fmt.Sprintf("[%s:%d %s:%d] %s", handler.Message().Chat.Title, handler.Message().Chat.ID, GetUserStr(handler.Message().Sender), handler.Message().Sender.ID, handler.Message().Text)
			if handler.Message().IsReply() {
				log_string = fmt.Sprintf("%s -> %s", log_string, GetUserStr(handler.Message().ReplyTo.Sender))
			}
			log.Infof(log_string)
		} else if handler.Message().Photo != nil {
			log.Infof("[%s:%d %s:%d] %s", handler.Message().Chat.Title, handler.Message().Chat.ID, GetUserStr(handler.Message().Sender), handler.Message().Sender.ID, photoTag)
		}
		return handler, nil
	} else if handler.Callback() != nil {
		log.Infof("[Callback %s:%d] Data: %s", GetUserStr(handler.Callback().Sender), handler.Callback().Sender.ID, handler.Callback().Data)
		return handler, nil

	}
	return handler, errors.Create(errors.InvalidTypeError)
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
