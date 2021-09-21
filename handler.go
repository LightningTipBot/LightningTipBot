package main

import (
	"context"
	"fmt"

	"github.com/LightningTipBot/LightningTipBot/internal/telegram/intercept"
	tb "gopkg.in/tucnak/telebot.v2"
)

type Handler struct {
	Endpoints   []interface{}
	Handler     interface{}
	Interceptor *Interceptor
}

// registerTelegramHandlers will register all telegram handlers.
func (bot TipBot) registerTelegramHandlers() {
	telegramHandlerRegistration.Do(func() {
		// Set up handlers
		for _, h := range bot.getHandler() {
			fmt.Println("registering", h.Endpoints)
			bot.register(h)
		}

	})
}

func (bot TipBot) registerHandlerWithInterceptor(h Handler) {
	switch h.Interceptor.Type {
	case MessageInterceptor:
		for _, endpoint := range h.Endpoints {
			bot.telegram.Handle(endpoint,
				intercept.HandlerWithMessage(h.Handler.(func(ctx context.Context, query *tb.Message)),
					intercept.WithBeforeMessage(h.Interceptor.Before...),
					intercept.WithAfterMessage(h.Interceptor.After...)))
		}

	case QueryInterceptor:
		for _, endpoint := range h.Endpoints {
			bot.telegram.Handle(endpoint,
				intercept.HandlerWithQuery(h.Handler.(func(ctx context.Context, query *tb.Query)),
					intercept.WithBeforeQuery(h.Interceptor.Before...),
					intercept.WithAfterQuery(h.Interceptor.After...)))
		}
	case CallbackInterceptor:
		for _, endpoint := range h.Endpoints {
			bot.telegram.Handle(endpoint,
				intercept.HandlerWithCallback(h.Handler.(func(ctx context.Context, callback *tb.Callback)),
					intercept.WithBeforeCallback(h.Interceptor.Before...),
					intercept.WithAfterCallback(h.Interceptor.After...)))
		}
	}
}
func (bot TipBot) register(h Handler) {
	if h.Interceptor != nil {
		bot.registerHandlerWithInterceptor(h)
	} else {
		for _, endpoint := range h.Endpoints {
			bot.telegram.Handle(endpoint, h.Handler)
		}
	}
}
func (bot TipBot) getHandler() []Handler {
	return []Handler{
		{
			Endpoints: []interface{}{"/start"},
			Handler:   bot.startHandler,
		},
		{
			Endpoints: []interface{}{"/faucet", "/zapfhahn", "/kraan"},
			Handler:   bot.faucetHandler,
			Interceptor: &Interceptor{
				Type:   MessageInterceptor,
				Before: []intercept.Func{bot.requireUserInterceptor}},
		},
		{
			Endpoints: []interface{}{"/tip"},
			Handler:   bot.tipHandler,
			Interceptor: &Interceptor{
				Type: MessageInterceptor,
				Before: []intercept.Func{
					bot.loadUserInterceptor,
					bot.loadReplyToInterceptor,
				}},
		},
		{
			Endpoints: []interface{}{"/pay"},
			Handler:   bot.confirmPaymentHandler,
			Interceptor: &Interceptor{
				Type:   MessageInterceptor,
				Before: []intercept.Func{bot.loadUserInterceptor}},
		},
		{
			Endpoints: []interface{}{"/invoice"},
			Handler:   bot.invoiceHandler,
			Interceptor: &Interceptor{
				Type:   MessageInterceptor,
				Before: []intercept.Func{bot.loadUserInterceptor}},
		},
		{
			Endpoints: []interface{}{"/balance"},
			Handler:   bot.balanceHandler,
			Interceptor: &Interceptor{
				Type:   MessageInterceptor,
				Before: []intercept.Func{bot.loadUserInterceptor}},
		},
		{
			Endpoints: []interface{}{"/send"},
			Handler:   bot.confirmSendHandler,
			Interceptor: &Interceptor{
				Type:   MessageInterceptor,
				Before: []intercept.Func{bot.loadUserInterceptor}},
		},
		{
			Endpoints: []interface{}{"/help"},
			Handler:   bot.helpHandler,
			Interceptor: &Interceptor{
				Type:   MessageInterceptor,
				Before: []intercept.Func{bot.loadUserInterceptor}},
		},
		{
			Endpoints: []interface{}{"/basics"},
			Handler:   bot.basicsHandler,
			Interceptor: &Interceptor{
				Type:   MessageInterceptor,
				Before: []intercept.Func{bot.loadUserInterceptor}},
		},
		{
			Endpoints: []interface{}{"/donate"},
			Handler:   bot.donationHandler,
			Interceptor: &Interceptor{
				Type:   MessageInterceptor,
				Before: []intercept.Func{bot.loadUserInterceptor}},
		},
		{
			Endpoints: []interface{}{"/advanced"},
			Handler:   bot.advancedHelpHandler,
			Interceptor: &Interceptor{
				Type:   MessageInterceptor,
				Before: []intercept.Func{bot.loadUserInterceptor}},
		},
		{
			Endpoints: []interface{}{"/link"},
			Handler:   bot.lndhubHandler,
			Interceptor: &Interceptor{
				Type:   MessageInterceptor,
				Before: []intercept.Func{bot.loadUserInterceptor}},
		},
		{
			Endpoints: []interface{}{"/lnurl"},
			Handler:   bot.lnurlHandler,
			Interceptor: &Interceptor{
				Type:   MessageInterceptor,
				Before: []intercept.Func{bot.loadUserInterceptor}},
		},
		{
			Endpoints: []interface{}{tb.OnPhoto},
			Handler:   bot.photoHandler,
			Interceptor: &Interceptor{
				Type: MessageInterceptor,
				Before: []intercept.Func{
					bot.logMessageInterceptor,
					bot.requirePrivateChatInterceptor,
					bot.loadUserInterceptor}},
		},
		{
			Endpoints: []interface{}{tb.OnText},
			Handler:   bot.anyTextHandler,
			Interceptor: &Interceptor{
				Type: MessageInterceptor,
				Before: []intercept.Func{
					bot.logMessageInterceptor,
					bot.requirePrivateChatInterceptor,
					bot.loadUserInterceptor,
				}},
		},
		{
			Endpoints: []interface{}{tb.OnQuery},
			Handler:   bot.anyQueryHandler,
			Interceptor: &Interceptor{
				Type:   QueryInterceptor,
				Before: []intercept.Func{bot.requireUserInterceptor}},
		},
		{
			Endpoints: []interface{}{tb.OnChosenInlineResult},
			Handler:   bot.anyChosenInlineHandler,
		},
		{
			Endpoints: []interface{}{&btnPay},
			Handler:   bot.payHandler,
			Interceptor: &Interceptor{
				Type:   CallbackInterceptor,
				Before: []intercept.Func{bot.loadUserInterceptor}},
		},
		{
			Endpoints: []interface{}{&btnCancelPay},
			Handler:   bot.cancelPaymentHandler,
			Interceptor: &Interceptor{
				Type:   CallbackInterceptor,
				Before: []intercept.Func{bot.loadUserInterceptor}},
		},
		{
			Endpoints: []interface{}{&btnSend},
			Handler:   bot.sendHandler,
			Interceptor: &Interceptor{
				Type:   CallbackInterceptor,
				Before: []intercept.Func{bot.loadUserInterceptor}},
		},
		{
			Endpoints: []interface{}{&btnCancelSend},
			Handler:   bot.cancelSendHandler,
			Interceptor: &Interceptor{
				Type:   CallbackInterceptor,
				Before: []intercept.Func{bot.loadUserInterceptor}},
		},
		{
			Endpoints: []interface{}{&btnAcceptInlineSend},
			Handler:   bot.acceptInlineSendHandler,
			Interceptor: &Interceptor{
				Type:   CallbackInterceptor,
				Before: []intercept.Func{bot.loadUserInterceptor}},
		},
		{
			Endpoints: []interface{}{&btnCancelInlineSend},
			Handler:   bot.cancelInlineSendHandler,
		},
		{
			Endpoints: []interface{}{&btnAcceptInlineReceive},
			Handler:   bot.acceptInlineReceiveHandler,
			Interceptor: &Interceptor{
				Type:   CallbackInterceptor,
				Before: []intercept.Func{bot.loadUserInterceptor}},
		},
		{
			Endpoints: []interface{}{&btnCancelInlineReceive},
			Handler:   bot.cancelInlineReceiveHandler,
		},
		{
			Endpoints: []interface{}{&btnAcceptInlineFaucet},
			Handler:   bot.accpetInlineFaucetHandler,
			Interceptor: &Interceptor{
				Type:   CallbackInterceptor,
				Before: []intercept.Func{bot.loadUserInterceptor}},
		},
		{
			Endpoints: []interface{}{&btnCancelInlineFaucet},
			Handler:   bot.cancelInlineFaucetHandler,
		},
	}
}
