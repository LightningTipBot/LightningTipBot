package intercept

import (
	"context"
	tb "gopkg.in/tucnak/telebot.v2"
)

type handlerCallbackFunc func(ctx context.Context, message *tb.Callback)
type interceptCallbackFunc func(ctx context.Context, message *tb.Callback) context.Context

type handlerCallbackInterceptor struct {
	handler handlerCallbackFunc
	before  InterceptCallbackChain
	after   InterceptCallbackChain
}
type InterceptCallbackChain []interceptCallbackFunc
type InterceptCallbackOption func(*handlerCallbackInterceptor)

func WithBeforeCallback(chain ...interceptCallbackFunc) InterceptCallbackOption {
	return func(a *handlerCallbackInterceptor) {
		a.before = chain
	}
}
func WithAfterCallback(chain ...interceptCallbackFunc) InterceptCallbackOption {
	return func(a *handlerCallbackInterceptor) {
		a.after = chain
	}
}

func interceptCallback(ctx context.Context, message *tb.Callback, hm InterceptCallbackChain) context.Context {
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

func HandlerWithCallback(handler handlerCallbackFunc, option ...InterceptCallbackOption) func(Callback *tb.Callback) {
	hm := &handlerCallbackInterceptor{handler: handler}
	for _, opt := range option {
		opt(hm)
	}
	return func(c *tb.Callback) {
		ctx := interceptCallback(context.Background(), c, hm.before)
		hm.handler(ctx, c)
		interceptCallback(ctx, c, hm.after)
	}
}
