package intercept

import (
	"context"

	log "github.com/sirupsen/logrus"
	tb "gopkg.in/lightningtipbot/telebot.v3"
)

type Context struct {
	context.Context
	TeleContext
}
type TeleContext struct {
	tb.Context
}

type Func func(ctx Context) (Context, error)

type handlerInterceptor struct {
	handler Func
	before  Chain
	after   Chain
	onDefer Chain
}
type Chain []Func
type Option func(*handlerInterceptor)

func WithBefore(chain ...Func) Option {
	return func(a *handlerInterceptor) {
		a.before = chain
	}
}
func WithAfter(chain ...Func) Option {
	return func(a *handlerInterceptor) {
		a.after = chain
	}
}
func WithDefer(chain ...Func) Option {
	return func(a *handlerInterceptor) {
		a.onDefer = chain
	}
}

func intercept(h Context, hm Chain) (Context, error) {

	if hm != nil {
		var err error
		for _, m := range hm {
			h, err = m(h)
			if err != nil {
				return h, err
			}
		}
	}
	return h, nil
}

func WithHandler(handler Func, option ...Option) tb.HandlerFunc {
	hm := &handlerInterceptor{handler: handler}
	for _, opt := range option {
		opt(hm)
	}
	return func(c tb.Context) error {
		h := Context{TeleContext: TeleContext{Context: c}, Context: context.Background()}
		h, err := intercept(h, hm.before)
		if err != nil {
			log.Traceln(err)
			return err
		}
		defer intercept(h, hm.onDefer)
		h, err = hm.handler(h)
		if err != nil {
			log.Traceln(err)
			return err
		}
		_, err = intercept(h, hm.after)
		if err != nil {
			log.Traceln(err)
			return err
		}
		return nil
	}
}
