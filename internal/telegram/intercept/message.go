package intercept

import (
	"context"

	log "github.com/sirupsen/logrus"
	tb "gopkg.in/lightningtipbot/telebot.v2"
)

type MessageFuncHandler func(ctx context.Context, message *tb.Message) (context.Context, error)

type handlerMessageInterceptor struct {
	handler MessageFuncHandler
	before  MessageChain
	after   MessageChain
	onDefer MessageChain
}
type MessageChain []Func
type MessageInterceptOption func(*handlerMessageInterceptor)

func WithBeforeMessage(chain ...Func) MessageInterceptOption {
	return func(a *handlerMessageInterceptor) {
		a.before = chain
	}
}
func WithAfterMessage(chain ...Func) MessageInterceptOption {
	return func(a *handlerMessageInterceptor) {
		a.after = chain
	}
}
func WithDeferMessage(chain ...Func) MessageInterceptOption {
	return func(a *handlerMessageInterceptor) {
		a.onDefer = chain
	}
}

func interceptMessage(ctx context.Context, message *tb.Message, hm MessageChain) (context.Context, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if hm != nil {
		var err error
		for _, m := range hm {
			ctx, err = m(ctx, message)
			if err != nil {
				return ctx, err
			}
		}
	}
	return ctx, nil
}

func HandlerWithMessage(handler MessageFuncHandler, option ...MessageInterceptOption) func(message *tb.Message) {
	hm := &handlerMessageInterceptor{handler: handler}
	for _, opt := range option {
		opt(hm)
	}
	return func(message *tb.Message) {
		ctx := context.Background()
		ctx, err := interceptMessage(ctx, message, hm.before)
		if err != nil {
			log.Traceln(err)
			return
		}
		ctx, err = hm.handler(ctx, message)
		defer interceptMessage(ctx, message, hm.onDefer)
		if err != nil {
			log.Traceln(err)
			return
		}
		_, err = interceptMessage(ctx, message, hm.after)
		if err != nil {
			log.Traceln(err)
			return
		}
	}
}
