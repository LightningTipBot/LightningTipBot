package intercept

import (
	"context"
	tb "gopkg.in/tucnak/telebot.v2"
)

type handlerQueryFunc func(ctx context.Context, message *tb.Query)
type interceptQueryFunc func(ctx context.Context, message *tb.Query) context.Context

type handlerQueryInterceptor struct {
	handler handlerQueryFunc
	before  QueryChain
	after   QueryChain
}
type QueryChain []interceptQueryFunc
type QueryInterceptOption func(*handlerQueryInterceptor)

func WithBeforeQuery(chain ...interceptQueryFunc) QueryInterceptOption {
	return func(a *handlerQueryInterceptor) {
		a.before = chain
	}
}
func WithAfterQuery(chain ...interceptQueryFunc) QueryInterceptOption {
	return func(a *handlerQueryInterceptor) {
		a.after = chain
	}
}

func interceptQuery(ctx context.Context, message *tb.Query, hm QueryChain) context.Context {
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

func HandlerWithQuery(handler handlerQueryFunc, option ...QueryInterceptOption) func(message *tb.Query) {
	hm := &handlerQueryInterceptor{handler: handler}
	for _, opt := range option {
		opt(hm)
	}
	return func(message *tb.Query) {
		ctx := interceptQuery(context.Background(), message, hm.before)
		hm.handler(ctx, message)
		interceptQuery(ctx, message, hm.after)
	}
}
