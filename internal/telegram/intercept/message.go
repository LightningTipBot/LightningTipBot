package intercept

import (
	"context"
	log "github.com/sirupsen/logrus"
	tb "gopkg.in/tucnak/telebot.v2"
)

type MessageInterface interface {
	a() func(ctx context.Context, message *tb.Message)
}
type MessageFuncHandler func(ctx context.Context, message *tb.Message)

type handlerMessageInterceptor struct {
	handler MessageFuncHandler
	before  MessageChain
	after   MessageChain
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
		ctx, err := interceptMessage(context.Background(), message, hm.before)
		if err != nil {
			log.Errorln(err)
			return
		}
		hm.handler(ctx, message)
		_, err = interceptMessage(ctx, message, hm.after)
		if err != nil {
			log.Errorln(err)
			return
		}
	}
}
