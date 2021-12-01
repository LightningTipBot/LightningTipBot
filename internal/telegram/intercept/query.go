package intercept

import (
	"context"
	log "github.com/sirupsen/logrus"
	tb "gopkg.in/tucnak/telebot.v2"
)

type QueryFuncHandler func(ctx context.Context, message *tb.Query)

type handlerQueryInterceptor struct {
	handler QueryFuncHandler
	before  QueryChain
	after   QueryChain
}
type QueryChain []Func
type QueryInterceptOption func(*handlerQueryInterceptor)

func WithBeforeQuery(chain ...Func) QueryInterceptOption {
	return func(a *handlerQueryInterceptor) {
		a.before = chain
	}
}
func WithAfterQuery(chain ...Func) QueryInterceptOption {
	return func(a *handlerQueryInterceptor) {
		a.after = chain
	}
}

func interceptQuery(ctx context.Context, message *tb.Query, hm QueryChain) (context.Context, error) {
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

func HandlerWithQuery(handler QueryFuncHandler, option ...QueryInterceptOption) func(message *tb.Query) {
	hm := &handlerQueryInterceptor{handler: handler}
	for _, opt := range option {
		opt(hm)
	}
	return func(message *tb.Query) {
		ctx, err := interceptQuery(context.Background(), message, hm.before)
		if err != nil {
			log.Traceln(err)
			return
		}
		hm.handler(ctx, message)
		_, err = interceptQuery(ctx, message, hm.after)
		if err != nil {
			log.Traceln(err)
			return
		}
	}
}
