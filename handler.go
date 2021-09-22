package main

import (
	"context"
	"fmt"
	"strings"

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

// registerHandlerWithInterceptor will register a handler with all the predefined interceptors, based on the interceptor type
func (bot TipBot) registerHandlerWithInterceptor(h Handler) {
	switch h.Interceptor.Type {
	case MessageInterceptor:
		for _, endpoint := range h.Endpoints {
			bot.handle(endpoint, intercept.HandlerWithMessage(h.Handler.(func(ctx context.Context, query *tb.Message)),
				intercept.WithBeforeMessage(h.Interceptor.Before...),
				intercept.WithAfterMessage(h.Interceptor.After...)))
		}
	case QueryInterceptor:
		for _, endpoint := range h.Endpoints {
			bot.handle(endpoint, intercept.HandlerWithQuery(h.Handler.(func(ctx context.Context, query *tb.Query)),
				intercept.WithBeforeQuery(h.Interceptor.Before...),
				intercept.WithAfterQuery(h.Interceptor.After...)))
		}
	case CallbackInterceptor:
		for _, endpoint := range h.Endpoints {
			bot.handle(endpoint, intercept.HandlerWithCallback(h.Handler.(func(ctx context.Context, callback *tb.Callback)),
				intercept.WithBeforeCallback(h.Interceptor.Before...),
				intercept.WithAfterCallback(h.Interceptor.After...)))
		}
	}
}

// handle accepts an endpoint and handler for telegram handler registration.
// function will automatically register string handlers as uppercase and first letter uppercase.
func (bot TipBot) handle(endpoint interface{}, handler interface{}) {
	// register the endpoint
	bot.telegram.Handle(endpoint, handler)
	switch endpoint.(type) {
	case string:
		// check if this is a string endpoint
		sEndpoint := endpoint.(string)
		if strings.HasPrefix(sEndpoint, "/") {
			// Uppercase endpoint registration, because starting with slash
			bot.telegram.Handle(strings.ToUpper(sEndpoint), handler)
			if len(sEndpoint) > 2 {
				// Also register endpoint with first letter uppercase
				bot.telegram.Handle(fmt.Sprintf("/%s%s", strings.ToUpper(string(sEndpoint[1])), sEndpoint[2:]), handler)
			}
		}
	}
}

// register registers a handler, so that telegram can handle the endpoint correctly.
func (bot TipBot) register(h Handler) {
	if h.Interceptor != nil {
		bot.registerHandlerWithInterceptor(h)
	} else {
		for _, endpoint := range h.Endpoints {
			bot.handle(endpoint, h.Handler)
		}
	}
}

// getHandler returns a list of all handlers, that need to be registered with telegram
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
			Handler:   bot.payHandler,
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
			Handler:   bot.sendHandler,
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
			Handler:   bot.confirmPayHandler,
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
			Handler:   bot.confirmSendHandler,
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
