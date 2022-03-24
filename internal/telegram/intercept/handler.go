package intercept

import (
	"context"

	log "github.com/sirupsen/logrus"
	tb "gopkg.in/telebot.v3"
)

type Handler struct {
	Ctx context.Context
	tb.Context
}

type Func func(handler Handler) (Handler, error)

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

func intercept(h Handler, hm Chain) (Handler, error) {

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
		h := Handler{Ctx: context.Background(), Context: c}
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
