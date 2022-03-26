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
	tb "gopkg.in/lightningtipbot/telebot.v3"
)

type Interceptor struct {
	Before  []intercept.Func
	After   []intercept.Func
	OnDefer []intercept.Func
}

// singletonClickInterceptor uses the onceMap to determine whether the object k1 already interacted
// with the user k2. If so, it will return an error.
func (bot TipBot) singletonCallbackInterceptor(ctx intercept.Context) (intercept.Context, error) {
	if ctx.Callback() != nil {
		return ctx, once.Once(ctx.Callback().Data, strconv.FormatInt(ctx.Callback().Sender.ID, 10))
	}
	return ctx, errors.Create(errors.InvalidTypeError)
}

// lockInterceptor invoked as first before interceptor
func (bot TipBot) lockInterceptor(ctx intercept.Context) (intercept.Context, error) {
	user := ctx.Sender()
	if user != nil {
		mutex.Lock(strconv.FormatInt(user.ID, 10))
		return ctx, nil
	}
	return ctx, errors.Create(errors.InvalidTypeError)
}

// unlockInterceptor invoked as onDefer interceptor
func (bot TipBot) unlockInterceptor(ctx intercept.Context) (intercept.Context, error) {
	user := ctx.Sender()
	if user != nil {
		mutex.Unlock(strconv.FormatInt(user.ID, 10))
		return ctx, nil
	}
	return ctx, errors.Create(errors.InvalidTypeError)
}
func (bot TipBot) idInterceptor(ctx intercept.Context) (intercept.Context, error) {
	ctx.Context = context.WithValue(ctx, "uid", RandStringRunes(64))
	return ctx, nil
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
func (bot TipBot) requireUserInterceptor(ctx intercept.Context) (intercept.Context, error) {
	var user *lnbits.User
	var err error
	u := ctx.Sender()
	if u != nil {
		user, err = GetUser(u, bot)
		// do not respond to banned users
		if bot.UserIsBanned(user) {
			ctx.Context = context.WithValue(ctx, "banned", true)
			ctx.Context = context.WithValue(ctx, "user", user)
			return ctx, errors.Create(errors.InvalidTypeError)
		}
		if user != nil {
			ctx.Context = context.WithValue(ctx, "user", user)
			return ctx, err
		}
	}
	return ctx, errors.Create(errors.InvalidTypeError)
}

// startUserInterceptor will invoke /start if user not exists.
func (bot TipBot) startUserInterceptor(ctx intercept.Context) (intercept.Context, error) {
	handler, err := bot.loadUserInterceptor(ctx)
	if err != nil {
		// user banned
		return handler, err
	}
	// load user
	u := ctx.Value("user")
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
func (bot TipBot) loadUserInterceptor(ctx intercept.Context) (intercept.Context, error) {
	ctx, _ = bot.requireUserInterceptor(ctx)
	// if user is banned, also loadUserInterceptor will return an error
	if ctx.Value("banned") != nil && ctx.Value("banned").(bool) {
		return ctx, errors.Create(errors.InvalidTypeError)
	}
	return ctx, nil
}

// loadReplyToInterceptor Loading the Telegram user with message intercept
func (bot TipBot) loadReplyToInterceptor(ctx intercept.Context) (intercept.Context, error) {
	if ctx.Message() != nil {
		if ctx.Message().ReplyTo != nil {
			if ctx.Message().ReplyTo.Sender != nil {
				user, _ := GetUser(ctx.Message().ReplyTo.Sender, bot)
				user.Telegram = ctx.Message().ReplyTo.Sender
				ctx.Context = context.WithValue(ctx, "reply_to_user", user)
				return ctx, nil

			}
		}
		return ctx, nil
	}
	return ctx, errors.Create(errors.InvalidTypeError)
}

func (bot TipBot) localizerInterceptor(ctx intercept.Context) (intercept.Context, error) {
	var userLocalizer *i18n2.Localizer
	var publicLocalizer *i18n2.Localizer

	// default language is english
	publicLocalizer = i18n2.NewLocalizer(i18n.Bundle, "en")
	ctx.Context = context.WithValue(ctx, "publicLanguageCode", "en")
	ctx.Context = context.WithValue(ctx, "publicLocalizer", publicLocalizer)

	if ctx.Message() != nil {
		userLocalizer = i18n2.NewLocalizer(i18n.Bundle, ctx.Message().Sender.LanguageCode)
		ctx.Context = context.WithValue(ctx, "userLanguageCode", ctx.Message().Sender.LanguageCode)
		ctx.Context = context.WithValue(ctx, "userLocalizer", userLocalizer)
		if ctx.Message().Private() {
			// in pm overwrite public localizer with user localizer
			ctx.Context = context.WithValue(ctx, "publicLanguageCode", ctx.Message().Sender.LanguageCode)
			ctx.Context = context.WithValue(ctx, "publicLocalizer", userLocalizer)
		}
		return ctx, nil
	} else if ctx.Callback() != nil {
		userLocalizer = i18n2.NewLocalizer(i18n.Bundle, ctx.Callback().Sender.LanguageCode)
		ctx.Context = context.WithValue(ctx, "userLanguageCode", ctx.Callback().Sender.LanguageCode)
		ctx.Context = context.WithValue(ctx, "userLocalizer", userLocalizer)
		return ctx, nil
	} else if ctx.Query() != nil {
		userLocalizer = i18n2.NewLocalizer(i18n.Bundle, ctx.Query().Sender.LanguageCode)
		ctx.Context = context.WithValue(ctx, "userLanguageCode", ctx.Query().Sender.LanguageCode)
		ctx.Context = context.WithValue(ctx, "userLocalizer", userLocalizer)
		return ctx, nil
	}

	return ctx, nil
}

func (bot TipBot) requirePrivateChatInterceptor(ctx intercept.Context) (intercept.Context, error) {
	if ctx.Message() != nil {
		if ctx.Message().Chat.Type != tb.ChatPrivate {
			return ctx, fmt.Errorf("[requirePrivateChatInterceptor] no private chat")
		}
		return ctx, nil
	}
	return ctx, errors.Create(errors.InvalidTypeError)
}

const photoTag = "<Photo>"

func (bot TipBot) logMessageInterceptor(ctx intercept.Context) (intercept.Context, error) {
	if ctx.Message() != nil {

		if ctx.Message().Text != "" {
			log_string := fmt.Sprintf("[%s:%d %s:%d] %s", ctx.Message().Chat.Title, ctx.Message().Chat.ID, GetUserStr(ctx.Message().Sender), ctx.Message().Sender.ID, ctx.Message().Text)
			if ctx.Message().IsReply() {
				log_string = fmt.Sprintf("%s -> %s", log_string, GetUserStr(ctx.Message().ReplyTo.Sender))
			}
			log.Infof(log_string)
		} else if ctx.Message().Photo != nil {
			log.Infof("[%s:%d %s:%d] %s", ctx.Message().Chat.Title, ctx.Message().Chat.ID, GetUserStr(ctx.Message().Sender), ctx.Message().Sender.ID, photoTag)
		}
		return ctx, nil
	} else if ctx.Callback() != nil {
		log.Infof("[Callback %s:%d] Data: %s", GetUserStr(ctx.Callback().Sender), ctx.Callback().Sender.ID, ctx.Callback().Data)
		return ctx, nil

	}
	return ctx, errors.Create(errors.InvalidTypeError)
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
