package intercept

import (
	"context"
	tb "gopkg.in/tucnak/telebot.v2"
)

type handlerMessageFunc func(ctx context.Context, message *tb.Message)
type interceptMessageFunc func(ctx context.Context, message *tb.Message) context.Context

type handlerMessageInterceptor struct {
	handler handlerMessageFunc
	before  InterceptChain
	after   InterceptChain
}
type InterceptChain []interceptMessageFunc
type InterceptOption func(*handlerMessageInterceptor)

func WithBeforeMessage(chain ...interceptMessageFunc) InterceptOption {
	return func(a *handlerMessageInterceptor) {
		a.before = chain
	}
}
func WithAfterMessage(chain ...interceptMessageFunc) InterceptOption {
	return func(a *handlerMessageInterceptor) {
		a.after = chain
	}
}

func interceptMessage(ctx context.Context, message *tb.Message, hm InterceptChain) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if hm != nil {
		for _, m := range hm {
			ctx = m(ctx, message)
		}
	}
	return ctx
}

func HandlerWithMessage(handler handlerMessageFunc, option ...InterceptOption) func(message *tb.Message) {
	hm := &handlerMessageInterceptor{handler: handler}
	for _, opt := range option {
		opt(hm)
	}
	return func(message *tb.Message) {
		ctx := interceptMessage(context.Background(), message, hm.before)
		hm.handler(ctx, message)
		interceptMessage(ctx, message, hm.after)
	}
}
