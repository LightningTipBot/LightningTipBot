package telegram

import (
	"fmt"
	"strings"

	"github.com/LightningTipBot/LightningTipBot/internal/telegram/intercept"
	log "github.com/sirupsen/logrus"
	tb "gopkg.in/lightningtipbot/telebot.v3"
)

type InterceptionWrapper struct {
	Endpoints   []interface{}
	Handler     intercept.Func
	Interceptor *Interceptor
}

// registerTelegramHandlers will register all Telegram handlers.
func (bot TipBot) registerTelegramHandlers() {
	telegramHandlerRegistration.Do(func() {
		// Set up handlers
		for _, h := range bot.getHandler() {
			log.Traceln("registering", h.Endpoints)
			bot.register(h)
		}

	})
}

func getDefaultBeforeInterceptor(bot TipBot) []intercept.Func {
	return []intercept.Func{bot.idInterceptor}
}
func getDefaultDeferInterceptor(bot TipBot) []intercept.Func {
	return []intercept.Func{bot.unlockInterceptor}
}
func getDefaultAfterInterceptor(bot TipBot) []intercept.Func {
	return []intercept.Func{}
}

// registerHandlerWithInterceptor will register a ctx with all the predefined interceptors, based on the interceptor type
func (bot TipBot) registerHandlerWithInterceptor(h InterceptionWrapper) {
	h.Interceptor.Before = append(getDefaultBeforeInterceptor(bot), h.Interceptor.Before...)
	//h.Interceptor.After = append(h.Interceptor.After, getDefaultAfterInterceptor(bot)...)
	//h.Interceptor.OnDefer = append(h.Interceptor.OnDefer, getDefaultDeferInterceptor(bot)...)
	for _, endpoint := range h.Endpoints {
		bot.handle(endpoint, intercept.WithHandler(h.Handler,
			intercept.WithBefore(h.Interceptor.Before...),
			intercept.WithAfter(h.Interceptor.After...),
			intercept.WithDefer(h.Interceptor.OnDefer...)))
	}
}

// handle accepts an endpoint and handler for Telegram handler registration.
// function will automatically register string handlers as uppercase and first letter uppercase.
func (bot TipBot) handle(endpoint interface{}, handler tb.HandlerFunc) {
	// register the endpoint
	bot.Telegram.Handle(endpoint, handler)
	switch endpoint.(type) {
	case string:
		// check if this is a string endpoint
		sEndpoint := endpoint.(string)
		if strings.HasPrefix(sEndpoint, "/") {
			// Uppercase endpoint registration, because starting with slash
			bot.Telegram.Handle(strings.ToUpper(sEndpoint), handler)
			if len(sEndpoint) > 2 {
				// Also register endpoint with first letter uppercase
				bot.Telegram.Handle(fmt.Sprintf("/%s%s", strings.ToUpper(string(sEndpoint[1])), sEndpoint[2:]), handler)
			}
		}
	}
}

// register registers a handler, so that Telegram can handle the endpoint correctly.
func (bot TipBot) register(h InterceptionWrapper) {
	if h.Interceptor != nil {
		bot.registerHandlerWithInterceptor(h)
	} else {
		for _, endpoint := range h.Endpoints {
			bot.handle(endpoint, intercept.WithHandler(h.Handler))
		}
	}
}

// getHandler returns a list of all handlers, that need to be registered with Telegram
func (bot TipBot) getHandler() []InterceptionWrapper {
	return []InterceptionWrapper{
		{
			Endpoints: []interface{}{"/start"},
			Handler:   bot.startHandler,
			Interceptor: &Interceptor{
				Before: []intercept.Func{
					bot.localizerInterceptor,
					bot.logMessageInterceptor,
					bot.loadUserInterceptor,
					bot.lockInterceptor,
				},
				OnDefer: []intercept.Func{
					bot.unlockInterceptor,
				},
			},
		},
		// {
		// 	Endpoints: []interface{}{"/generate"},
		// 	Handler:   bot.generateImages,
		// 	Interceptor: &Interceptor{
		// 		Before: []intercept.Func{
		// 			bot.requirePrivateChatInterceptor,
		// 			bot.localizerInterceptor,
		// 			bot.logMessageInterceptor,
		// 			bot.loadUserInterceptor,
		// 			bot.lockInterceptor,
		// 		},
		// 		OnDefer: []intercept.Func{
		// 			bot.unlockInterceptor,
		// 		},
		// 	},
		// },
		{
			Endpoints: []interface{}{"/tip", "/t", "/honk", "/zap"},
			Handler:   bot.tipHandler,
			Interceptor: &Interceptor{
				Before: []intercept.Func{
					bot.localizerInterceptor,
					bot.logMessageInterceptor,
					bot.requireUserInterceptor,
					bot.loadReplyToInterceptor,
					bot.lockInterceptor,
				},
				OnDefer: []intercept.Func{
					bot.unlockInterceptor,
				},
			},
		},
		{
			Endpoints: []interface{}{"/pay"},
			Handler:   bot.payHandler,
			Interceptor: &Interceptor{
				Before: []intercept.Func{
					bot.requirePrivateChatInterceptor,
					bot.localizerInterceptor,
					bot.logMessageInterceptor,
					bot.requireUserInterceptor,
					bot.lockInterceptor,
				},
				OnDefer: []intercept.Func{
					bot.unlockInterceptor,
				},
			},
		},
		{
			Endpoints: []interface{}{"/invoice", &btnInvoiceMainMenu},
			Handler:   bot.invoiceHandler,
			Interceptor: &Interceptor{

				Before: []intercept.Func{
					bot.requirePrivateChatInterceptor,
					bot.localizerInterceptor,
					bot.logMessageInterceptor,
					bot.requireUserInterceptor,
					bot.lockInterceptor,
				},
				OnDefer: []intercept.Func{
					bot.unlockInterceptor,
				},
			},
		},
		{
			Endpoints: []interface{}{"/set"},
			Handler:   bot.settingHandler,
			Interceptor: &Interceptor{
				Before: []intercept.Func{
					bot.requirePrivateChatInterceptor,
					bot.localizerInterceptor,
					bot.logMessageInterceptor,
					bot.requireUserInterceptor,
					bot.lockInterceptor,
				},
				OnDefer: []intercept.Func{
					bot.unlockInterceptor,
				},
			},
		},
		{
			Endpoints: []interface{}{"/nostr"},
			Handler:   bot.nostrHandler,
			Interceptor: &Interceptor{
				Before: []intercept.Func{
					bot.requirePrivateChatInterceptor,
					bot.localizerInterceptor,
					bot.logMessageInterceptor,
					bot.requireUserInterceptor,
					bot.lockInterceptor,
				},
				OnDefer: []intercept.Func{
					bot.unlockInterceptor,
				},
			},
		},
		{
			Endpoints: []interface{}{"/node"},
			Handler:   bot.nodeHandler,
			Interceptor: &Interceptor{
				Before: []intercept.Func{
					bot.requirePrivateChatInterceptor,
					bot.localizerInterceptor,
					bot.logMessageInterceptor,
					bot.requireUserInterceptor,
					bot.lockInterceptor,
				},
				OnDefer: []intercept.Func{
					bot.unlockInterceptor,
				},
			},
		},
		{
			Endpoints: []interface{}{&btnSatdressCheckInvoice},
			Handler:   bot.satdressCheckInvoiceHandler,
			Interceptor: &Interceptor{

				Before: []intercept.Func{
					bot.localizerInterceptor,
					bot.requireUserInterceptor,
					bot.answerCallbackInterceptor,
					bot.lockInterceptor,
				},
				OnDefer: []intercept.Func{
					bot.unlockInterceptor,
				},
			},
		},
		{
			Endpoints: []interface{}{"/shops"},
			Handler:   bot.shopsHandler,
			Interceptor: &Interceptor{

				Before: []intercept.Func{
					bot.logMessageInterceptor,
					bot.requireUserInterceptor,
					bot.loadUserInterceptor,
					bot.lockInterceptor,
				},
				OnDefer: []intercept.Func{
					bot.unlockInterceptor,
				}},
		},
		{
			Endpoints: []interface{}{"/shop"},
			Handler:   bot.shopHandler,
			Interceptor: &Interceptor{

				Before: []intercept.Func{
					bot.localizerInterceptor,
					bot.logMessageInterceptor,
					bot.requireUserInterceptor,
					bot.loadUserInterceptor,
					bot.lockInterceptor,
				},
				OnDefer: []intercept.Func{
					bot.unlockInterceptor,
				}},
		},
		// {
		// 	Endpoints: []interface{}{"/gpt", "/chat"},
		// 	Handler:   bot.gptHandler,
		// 	Interceptor: &Interceptor{

		// 		Before: []intercept.Func{
		// 			bot.localizerInterceptor,
		// 			bot.logMessageInterceptor,
		// 			bot.requireUserInterceptor,
		// 			bot.lockInterceptor,
		// 		},
		// 		OnDefer: []intercept.Func{
		// 			bot.unlockInterceptor,
		// 		},
		// 	},
		// },
		{
			Endpoints: []interface{}{"/balance", &btnBalanceMainMenu},
			Handler:   bot.balanceHandler,
			Interceptor: &Interceptor{

				Before: []intercept.Func{
					bot.requirePrivateChatInterceptor,
					bot.localizerInterceptor,
					bot.logMessageInterceptor,
					bot.requireUserInterceptor,
					bot.lockInterceptor,
				},
				OnDefer: []intercept.Func{
					bot.unlockInterceptor,
				},
			},
		},
		{
			Endpoints: []interface{}{"/send", &btnSendMenuEnter},
			Handler:   bot.sendHandler,
			Interceptor: &Interceptor{

				Before: []intercept.Func{
					bot.localizerInterceptor,
					bot.logMessageInterceptor,
					bot.requireUserInterceptor,
					bot.loadReplyToInterceptor,
					bot.lockInterceptor,
				},
				OnDefer: []intercept.Func{
					bot.unlockInterceptor,
				},
			},
		},
		{
			Endpoints: []interface{}{&btnSendMainMenu},
			Handler:   bot.keyboardSendHandler,
			Interceptor: &Interceptor{

				Before: []intercept.Func{
					bot.localizerInterceptor,
					bot.logMessageInterceptor,
					bot.requireUserInterceptor,
					bot.loadReplyToInterceptor,
					bot.lockInterceptor,
				},
				OnDefer: []intercept.Func{
					bot.unlockInterceptor,
				},
			},
		},
		// previously, this was the send menu but it
		// was replaced with the webapp
		// {
		// 	Endpoints: []interface{}{&btnWebAppMainMenu},
		// 	Handler:   bot.keyboardSendHandler,
		// 	Interceptor: &Interceptor{

		// 		Before: []intercept.Func{
		// 			bot.localizerInterceptor,
		// 			bot.logMessageInterceptor,
		// 			bot.requireUserInterceptor,
		// 			bot.loadReplyToInterceptor,
		// 			bot.lockInterceptor,
		// 		},
		// 		OnDefer: []intercept.Func{
		// 			bot.unlockInterceptor,
		// 		},
		// 	},
		// },
		{
			Endpoints: []interface{}{"/transactions"},
			Handler:   bot.transactionsHandler,
			Interceptor: &Interceptor{
				Before: []intercept.Func{
					bot.requirePrivateChatInterceptor,
					bot.localizerInterceptor,
					bot.logMessageInterceptor,
					bot.requireUserInterceptor,
				}},
		},
		{
			Endpoints: []interface{}{&btnLeftTransactionsButton},
			Handler:   bot.transactionsScrollLeftHandler,
			Interceptor: &Interceptor{

				Before: []intercept.Func{
					bot.localizerInterceptor,
					bot.loadUserInterceptor,
					bot.answerCallbackInterceptor,
					bot.lockInterceptor,
				},
				OnDefer: []intercept.Func{
					bot.unlockInterceptor,
				},
			},
		},
		{
			Endpoints: []interface{}{&btnRightTransactionsButton},
			Handler:   bot.transactionsScrollRightHandler,
			Interceptor: &Interceptor{

				Before: []intercept.Func{
					bot.localizerInterceptor,
					bot.loadUserInterceptor,
					bot.answerCallbackInterceptor,
					bot.lockInterceptor,
				},
				OnDefer: []intercept.Func{
					bot.unlockInterceptor,
				},
			},
		},
		{
			Endpoints: []interface{}{"/faucet", "/zapfhahn", "/kraan", "/grifo"},
			Handler:   bot.faucetHandler,
			Interceptor: &Interceptor{
				Before: []intercept.Func{
					bot.localizerInterceptor,
					bot.logMessageInterceptor,
					bot.requireUserInterceptor,
					bot.lockInterceptor,
				},
				OnDefer: []intercept.Func{
					bot.unlockInterceptor,
				},
			},
		},
		{
			Endpoints: []interface{}{"/tipjar", "/spendendose"},
			Handler:   bot.tipjarHandler,
			Interceptor: &Interceptor{
				Before: []intercept.Func{
					bot.localizerInterceptor,
					bot.logMessageInterceptor,
					bot.requireUserInterceptor,
					bot.lockInterceptor,
				},
				OnDefer: []intercept.Func{
					bot.unlockInterceptor,
				},
			},
		},
		{
			Endpoints: []interface{}{"/help", &btnHelpMainMenu},
			Handler:   bot.helpHandler,
			Interceptor: &Interceptor{
				Before: []intercept.Func{
					bot.localizerInterceptor,
					bot.logMessageInterceptor,
					bot.loadUserInterceptor,
					bot.lockInterceptor,
				},
				OnDefer: []intercept.Func{
					bot.unlockInterceptor,
				},
			},
		},
		{
			Endpoints: []interface{}{"/basics"},
			Handler:   bot.basicsHandler,
			Interceptor: &Interceptor{

				Before: []intercept.Func{
					bot.localizerInterceptor,
					bot.logMessageInterceptor,
					bot.loadUserInterceptor,
					bot.lockInterceptor,
				},
				OnDefer: []intercept.Func{
					bot.unlockInterceptor,
				},
			},
		},
		{
			Endpoints: []interface{}{"/donate"},
			Handler:   bot.donationHandler,
			Interceptor: &Interceptor{

				Before: []intercept.Func{
					bot.localizerInterceptor,
					bot.logMessageInterceptor,
					bot.requireUserInterceptor,
					bot.lockInterceptor,
				},
				OnDefer: []intercept.Func{
					bot.unlockInterceptor,
				},
			},
		},
		{
			Endpoints: []interface{}{"/advanced"},
			Handler:   bot.advancedHelpHandler,
			Interceptor: &Interceptor{
				Before: []intercept.Func{
					bot.requirePrivateChatInterceptor,
					bot.localizerInterceptor,
					bot.logMessageInterceptor,
					bot.requireUserInterceptor,
					bot.lockInterceptor,
				},
				OnDefer: []intercept.Func{
					bot.unlockInterceptor,
				},
			},
		},
		{
			Endpoints: []interface{}{"/link"},
			Handler:   bot.lndhubHandler,
			Interceptor: &Interceptor{
				Before: []intercept.Func{
					bot.requirePrivateChatInterceptor,
					bot.localizerInterceptor,
					bot.logMessageInterceptor,
					bot.requireUserInterceptor,
					bot.lockInterceptor,
				},
				OnDefer: []intercept.Func{
					bot.unlockInterceptor,
				},
			},
		},
		{
			Endpoints: []interface{}{"/api"},
			Handler:   bot.apiHandler,
			Interceptor: &Interceptor{
				Before: []intercept.Func{
					bot.requirePrivateChatInterceptor,
					bot.localizerInterceptor,
					bot.logMessageInterceptor,
					bot.requireUserInterceptor,
					bot.lockInterceptor,
				},
				OnDefer: []intercept.Func{
					bot.unlockInterceptor,
				},
			},
		},
		{
			Endpoints: []interface{}{"/lnurl"},
			Handler:   bot.lnurlHandler,
			Interceptor: &Interceptor{
				Before: []intercept.Func{
					bot.requirePrivateChatInterceptor,
					bot.localizerInterceptor,
					bot.logMessageInterceptor,
					bot.requireUserInterceptor,
					bot.lockInterceptor,
				},
				OnDefer: []intercept.Func{
					bot.unlockInterceptor,
				},
			},
		},
		// group join
		{
			Endpoints: []interface{}{tb.OnUserJoined, tb.OnAddedToGroup},
			Handler:   bot.handleTelegramNewMember,
			Interceptor: &Interceptor{
				Before: []intercept.Func{
					bot.localizerInterceptor,
					bot.logMessageInterceptor,
					bot.tryLoadUserInterceptor,
					bot.lockInterceptor,
				},
				OnDefer: []intercept.Func{
					bot.unlockInterceptor,
				},
			},
		},
		// group tickets
		{
			Endpoints: []interface{}{"/group"},
			Handler:   bot.groupHandler,
			Interceptor: &Interceptor{
				Before: []intercept.Func{
					bot.localizerInterceptor,
					bot.logMessageInterceptor,
					bot.requireUserInterceptor,
					bot.lockInterceptor,
				},
				OnDefer: []intercept.Func{
					bot.unlockInterceptor,
				},
			},
		},
		{
			Endpoints: []interface{}{"/join"},
			Handler:   bot.groupRequestJoinHandler,
			Interceptor: &Interceptor{

				Before: []intercept.Func{
					bot.requirePrivateChatInterceptor,
					bot.localizerInterceptor,
					bot.logMessageInterceptor,
					bot.startUserInterceptor,
					bot.lockInterceptor,
				},
				OnDefer: []intercept.Func{
					bot.unlockInterceptor,
				},
			},
		},
		{
			Endpoints: []interface{}{&btnPayTicket},
			Handler:   bot.groupConfirmPayButtonHandler,
			Interceptor: &Interceptor{

				Before: []intercept.Func{
					bot.localizerInterceptor,
					bot.requireUserInterceptor,
					bot.answerCallbackInterceptor,
					bot.lockInterceptor,
				},
				OnDefer: []intercept.Func{
					bot.unlockInterceptor,
				},
			},
		},
		{
			Endpoints: []interface{}{tb.OnPhoto},
			Handler:   bot.photoHandler,
			Interceptor: &Interceptor{

				Before: []intercept.Func{
					bot.requirePrivateChatInterceptor,
					bot.localizerInterceptor,
					bot.logMessageInterceptor,
					bot.requireUserInterceptor,
					bot.lockInterceptor,
				},
				OnDefer: []intercept.Func{
					bot.unlockInterceptor,
				},
			},
		},
		{
			Endpoints: []interface{}{tb.OnDocument, tb.OnVideo, tb.OnAnimation, tb.OnVoice, tb.OnAudio, tb.OnSticker, tb.OnVideoNote},
			Handler:   bot.fileHandler,
			Interceptor: &Interceptor{

				Before: []intercept.Func{
					bot.requirePrivateChatInterceptor,
					bot.logMessageInterceptor,
					bot.loadUserInterceptor}},
		},
		{
			Endpoints: []interface{}{tb.OnText},
			Handler:   bot.anyTextHandler,
			Interceptor: &Interceptor{

				Before: []intercept.Func{
					bot.requirePrivateChatInterceptor, // Respond to any text only in private chat
					bot.localizerInterceptor,
					bot.logMessageInterceptor,
					bot.loadUserInterceptor, // need to use loadUserInterceptor instead of requireUserInterceptor, because user might not be registered yet
					bot.lockInterceptor,
				},
				OnDefer: []intercept.Func{
					bot.unlockInterceptor,
				},
			},
		},
		{
			Endpoints: []interface{}{tb.OnQuery},
			Handler:   bot.anyQueryHandler,
			Interceptor: &Interceptor{
				Before: []intercept.Func{
					bot.localizerInterceptor,
					bot.requireUserInterceptor,
					bot.lockInterceptor,
				},
				OnDefer: []intercept.Func{
					bot.unlockInterceptor,
				},
			},
		},
		{
			Endpoints: []interface{}{tb.OnInlineResult},
			Handler:   bot.anyChosenInlineHandler,
		},
		{
			Endpoints: []interface{}{&btnPay},
			Handler:   bot.confirmPayHandler,
			Interceptor: &Interceptor{

				Before: []intercept.Func{
					bot.localizerInterceptor,
					bot.requireUserInterceptor,
					bot.answerCallbackInterceptor,
					bot.lockInterceptor,
				},
				OnDefer: []intercept.Func{
					bot.unlockInterceptor,
				},
			},
		},
		{
			Endpoints: []interface{}{&btnCancelPay},
			Handler:   bot.cancelPaymentHandler,
			Interceptor: &Interceptor{

				Before: []intercept.Func{
					bot.localizerInterceptor,
					bot.requireUserInterceptor,
					bot.answerCallbackInterceptor,
					bot.lockInterceptor,
				},
				OnDefer: []intercept.Func{
					bot.unlockInterceptor,
				},
			},
		},
		{
			Endpoints: []interface{}{&btnSend},
			Handler:   bot.confirmSendHandler,
			Interceptor: &Interceptor{

				Before: []intercept.Func{
					bot.localizerInterceptor,
					bot.requireUserInterceptor,
					bot.answerCallbackInterceptor,
					bot.lockInterceptor,
				},
				OnDefer: []intercept.Func{
					bot.unlockInterceptor,
				},
			},
		},
		{
			Endpoints: []interface{}{&btnCancelSend},
			Handler:   bot.cancelSendHandler,
			Interceptor: &Interceptor{

				Before: []intercept.Func{
					bot.localizerInterceptor,
					bot.requireUserInterceptor,
					bot.answerCallbackInterceptor,
					bot.lockInterceptor,
				},
				OnDefer: []intercept.Func{
					bot.unlockInterceptor,
				},
			},
		},
		{
			Endpoints: []interface{}{&btnAcceptInlineSend},
			Handler:   bot.acceptInlineSendHandler,
			Interceptor: &Interceptor{

				Before: []intercept.Func{
					bot.localizerInterceptor,
					bot.loadUserInterceptor,
					bot.answerCallbackInterceptor,
					bot.lockInterceptor,
				},
				OnDefer: []intercept.Func{
					bot.unlockInterceptor,
				},
			},
		},
		{
			Endpoints: []interface{}{&btnCancelInlineSend},
			Handler:   bot.cancelInlineSendHandler,
			Interceptor: &Interceptor{

				Before: []intercept.Func{
					bot.localizerInterceptor,
					bot.requireUserInterceptor,
					bot.answerCallbackInterceptor,
					bot.lockInterceptor,
				},
				OnDefer: []intercept.Func{
					bot.unlockInterceptor,
				},
			},
		},
		{
			Endpoints: []interface{}{&btnAcceptInlineReceive},
			Handler:   bot.acceptInlineReceiveHandler,
			Interceptor: &Interceptor{

				Before: []intercept.Func{
					bot.localizerInterceptor,
					bot.loadUserInterceptor,
					bot.answerCallbackInterceptor,
					bot.lockInterceptor,
				},
				OnDefer: []intercept.Func{
					bot.unlockInterceptor,
				},
			},
		},
		{
			Endpoints: []interface{}{&btnCancelInlineReceive},
			Handler:   bot.cancelInlineReceiveHandler,
			Interceptor: &Interceptor{

				Before: []intercept.Func{
					bot.localizerInterceptor,
					bot.requireUserInterceptor,
					bot.answerCallbackInterceptor,
					bot.lockInterceptor,
				},
				OnDefer: []intercept.Func{
					bot.unlockInterceptor,
				},
			},
		},
		{
			Endpoints: []interface{}{&btnAcceptInlineFaucet},
			Handler:   bot.acceptInlineFaucetHandler,
			Interceptor: &Interceptor{

				Before: []intercept.Func{
					bot.singletonCallbackInterceptor,
					bot.localizerInterceptor,
					bot.loadUserInterceptor,
					bot.answerCallbackInterceptor,
					bot.lockInterceptor,
				},
				OnDefer: []intercept.Func{
					bot.unlockInterceptor,
					bot.answerCallbackInterceptor,
				},
			},
		},
		{
			Endpoints: []interface{}{&btnCancelInlineFaucet},
			Handler:   bot.cancelInlineFaucetHandler,
			Interceptor: &Interceptor{

				Before: []intercept.Func{
					bot.localizerInterceptor,
					bot.requireUserInterceptor,
					bot.answerCallbackInterceptor,
					bot.lockInterceptor,
				},
				OnDefer: []intercept.Func{
					bot.unlockInterceptor,
				},
			},
		},
		{
			Endpoints: []interface{}{&btnAcceptInlineTipjar},
			Handler:   bot.acceptInlineTipjarHandler,
			Interceptor: &Interceptor{

				Before: []intercept.Func{
					bot.localizerInterceptor,
					bot.requireUserInterceptor,
					bot.answerCallbackInterceptor,
					bot.lockInterceptor,
				},
				OnDefer: []intercept.Func{
					bot.unlockInterceptor,
				},
			},
		},
		{
			Endpoints: []interface{}{&btnCancelInlineTipjar},
			Handler:   bot.cancelInlineTipjarHandler,
			Interceptor: &Interceptor{

				Before: []intercept.Func{
					bot.localizerInterceptor,
					bot.requireUserInterceptor,
					bot.answerCallbackInterceptor,
					bot.lockInterceptor,
				},
				OnDefer: []intercept.Func{
					bot.unlockInterceptor,
				},
			},
		},
		{
			Endpoints: []interface{}{&btnWithdraw},
			Handler:   bot.confirmWithdrawHandler,
			Interceptor: &Interceptor{

				Before: []intercept.Func{
					bot.localizerInterceptor,
					bot.requireUserInterceptor,
					bot.answerCallbackInterceptor,
					bot.lockInterceptor,
				},
				OnDefer: []intercept.Func{
					bot.unlockInterceptor,
				},
			},
		},
		{
			Endpoints: []interface{}{&btnCancelWithdraw},
			Handler:   bot.cancelWithdrawHandler,
			Interceptor: &Interceptor{

				Before: []intercept.Func{
					bot.localizerInterceptor,
					bot.requireUserInterceptor,
					bot.answerCallbackInterceptor,
					bot.lockInterceptor,
				},
				OnDefer: []intercept.Func{
					bot.unlockInterceptor,
				},
			},
		},
		{
			Endpoints: []interface{}{&btnAuth},
			Handler:   bot.confirmLnurlAuthHandler,
			Interceptor: &Interceptor{

				Before: []intercept.Func{
					bot.localizerInterceptor,
					bot.requireUserInterceptor,
					bot.answerCallbackInterceptor,
					bot.lockInterceptor,
				},
				OnDefer: []intercept.Func{
					bot.unlockInterceptor,
				},
			},
		},
		{
			Endpoints: []interface{}{&btnCancelAuth},
			Handler:   bot.cancelLnurlAuthHandler,
			Interceptor: &Interceptor{

				Before: []intercept.Func{
					bot.localizerInterceptor,
					bot.requireUserInterceptor,
					bot.answerCallbackInterceptor,
					bot.lockInterceptor,
				},
				OnDefer: []intercept.Func{
					bot.unlockInterceptor,
				},
			},
		},
		{
			Endpoints: []interface{}{&shopNewShopButton},
			Handler:   bot.shopNewShopHandler,
			Interceptor: &Interceptor{

				Before: []intercept.Func{
					bot.localizerInterceptor,
					bot.loadUserInterceptor,
					bot.answerCallbackInterceptor,
					bot.lockInterceptor,
				},
				OnDefer: []intercept.Func{
					bot.unlockInterceptor,
				}},
		},
		{
			Endpoints: []interface{}{&shopAddItemButton},
			Handler:   bot.shopNewItemHandler,
			Interceptor: &Interceptor{

				Before: []intercept.Func{
					bot.localizerInterceptor,
					bot.loadUserInterceptor,
					bot.answerCallbackInterceptor,
					bot.lockInterceptor,
				},
				OnDefer: []intercept.Func{
					bot.unlockInterceptor,
				}},
		},
		{
			Endpoints: []interface{}{&shopBuyitemButton},
			Handler:   bot.shopGetItemFilesHandler,
			Interceptor: &Interceptor{

				Before: []intercept.Func{
					bot.localizerInterceptor,
					bot.loadUserInterceptor,
					bot.answerCallbackInterceptor,
					bot.lockInterceptor,
				},
				OnDefer: []intercept.Func{
					bot.unlockInterceptor,
				}},
		},
		{
			Endpoints: []interface{}{&shopNextitemButton},
			Handler:   bot.shopNextItemButtonHandler,
			Interceptor: &Interceptor{

				Before: []intercept.Func{
					bot.localizerInterceptor,
					bot.loadUserInterceptor,
					bot.answerCallbackInterceptor,
					bot.lockInterceptor,
				},
				OnDefer: []intercept.Func{
					bot.unlockInterceptor,
				}},
		},
		{
			Endpoints: []interface{}{&browseShopButton},
			Handler:   bot.shopsBrowser,
			Interceptor: &Interceptor{

				Before: []intercept.Func{
					bot.localizerInterceptor,
					bot.loadUserInterceptor,
					bot.answerCallbackInterceptor,
					bot.lockInterceptor,
				},
				OnDefer: []intercept.Func{
					bot.unlockInterceptor,
				}},
		},
		{
			Endpoints: []interface{}{&shopSelectButton},
			Handler:   bot.shopSelect,
			Interceptor: &Interceptor{

				Before: []intercept.Func{
					bot.localizerInterceptor,
					bot.loadUserInterceptor,
					bot.answerCallbackInterceptor,
					bot.lockInterceptor,
				},
				OnDefer: []intercept.Func{
					bot.unlockInterceptor,
				}},
		},
		// button that opens selection of shops to delete
		{
			Endpoints: []interface{}{&shopDeleteShopButton},
			Handler:   bot.shopsDeleteShopBrowser,
			Interceptor: &Interceptor{

				Before: []intercept.Func{
					bot.localizerInterceptor,
					bot.loadUserInterceptor,
					bot.answerCallbackInterceptor,
					bot.lockInterceptor,
				},
				OnDefer: []intercept.Func{
					bot.unlockInterceptor,
				}},
		},
		// button that selects which shop to delete
		{
			Endpoints: []interface{}{&shopDeleteSelectButton},
			Handler:   bot.shopSelectDelete,
			Interceptor: &Interceptor{

				Before: []intercept.Func{
					bot.localizerInterceptor,
					bot.loadUserInterceptor,
					bot.answerCallbackInterceptor,
					bot.lockInterceptor,
				},
				OnDefer: []intercept.Func{
					bot.unlockInterceptor,
				}},
		},
		// button that opens selection of shops to get links of
		{
			Endpoints: []interface{}{&shopLinkShopButton},
			Handler:   bot.shopsLinkShopBrowser,
			Interceptor: &Interceptor{

				Before: []intercept.Func{
					bot.localizerInterceptor,
					bot.loadUserInterceptor,
					bot.answerCallbackInterceptor,
					bot.lockInterceptor,
				},
				OnDefer: []intercept.Func{
					bot.unlockInterceptor,
				}},
		},
		// button that selects which shop to link
		{
			Endpoints: []interface{}{&shopLinkSelectButton},
			Handler:   bot.shopSelectLink,
			Interceptor: &Interceptor{

				Before: []intercept.Func{
					bot.localizerInterceptor,
					bot.loadUserInterceptor,
					bot.answerCallbackInterceptor,
					bot.lockInterceptor,
				},
				OnDefer: []intercept.Func{
					bot.unlockInterceptor,
				}},
		},
		// button that opens selection of shops to rename
		{
			Endpoints: []interface{}{&shopRenameShopButton},
			Handler:   bot.shopsRenameShopBrowser,
			Interceptor: &Interceptor{

				Before: []intercept.Func{
					bot.localizerInterceptor,
					bot.loadUserInterceptor,
					bot.answerCallbackInterceptor,
					bot.lockInterceptor,
				},
				OnDefer: []intercept.Func{
					bot.unlockInterceptor,
				}},
		},
		// button that selects which shop to rename
		{
			Endpoints: []interface{}{&shopRenameSelectButton},
			Handler:   bot.shopSelectRename,
			Interceptor: &Interceptor{

				Before: []intercept.Func{
					bot.localizerInterceptor,
					bot.loadUserInterceptor,
					bot.answerCallbackInterceptor,
					bot.lockInterceptor,
				},
				OnDefer: []intercept.Func{
					bot.unlockInterceptor,
				}},
		},
		// button that opens shops settings buttons view
		{
			Endpoints: []interface{}{&shopSettingsButton},
			Handler:   bot.shopSettingsHandler,
			Interceptor: &Interceptor{

				Before: []intercept.Func{
					bot.localizerInterceptor,
					bot.loadUserInterceptor,
					bot.answerCallbackInterceptor,
					bot.lockInterceptor,
				},
				OnDefer: []intercept.Func{
					bot.unlockInterceptor,
				}},
		},
		// button that lets user enter description for shops
		{
			Endpoints: []interface{}{&shopDescriptionShopButton},
			Handler:   bot.shopsDescriptionHandler,
			Interceptor: &Interceptor{

				Before: []intercept.Func{
					bot.localizerInterceptor,
					bot.loadUserInterceptor,
					bot.answerCallbackInterceptor,
					bot.lockInterceptor,
				},
				OnDefer: []intercept.Func{
					bot.unlockInterceptor,
				}},
		},
		// button that resets user shops
		{
			Endpoints: []interface{}{&shopResetShopButton},
			Handler:   bot.shopsResetHandler,
			Interceptor: &Interceptor{

				Before: []intercept.Func{
					bot.localizerInterceptor,
					bot.loadUserInterceptor,
					bot.answerCallbackInterceptor,
					bot.lockInterceptor,
				},
				OnDefer: []intercept.Func{
					bot.unlockInterceptor,
				}},
		},
		{
			Endpoints: []interface{}{&shopResetShopAskButton},
			Handler:   bot.shopsAskDeleteAllShopsHandler,
			Interceptor: &Interceptor{

				Before: []intercept.Func{
					bot.localizerInterceptor,
					bot.loadUserInterceptor,
					bot.answerCallbackInterceptor,
					bot.lockInterceptor,
				},
				OnDefer: []intercept.Func{
					bot.unlockInterceptor,
				}},
		},
		{
			Endpoints: []interface{}{&shopPrevitemButton},
			Handler:   bot.shopPrevItemButtonHandler,
			Interceptor: &Interceptor{

				Before: []intercept.Func{
					bot.localizerInterceptor,
					bot.loadUserInterceptor,
					bot.answerCallbackInterceptor,
					bot.lockInterceptor,
				},
				OnDefer: []intercept.Func{
					bot.unlockInterceptor,
				}},
		},
		{
			Endpoints: []interface{}{&shopShopsButton},
			Handler:   bot.shopsHandlerCallback,
			Interceptor: &Interceptor{

				Before: []intercept.Func{
					bot.localizerInterceptor,
					bot.loadUserInterceptor,
					bot.answerCallbackInterceptor,
					bot.lockInterceptor,
				},
				OnDefer: []intercept.Func{
					bot.unlockInterceptor,
				}},
		},
		// shop item settings buttons
		{
			Endpoints: []interface{}{&shopItemSettingsButton},
			Handler:   bot.shopItemSettingsHandler,
			Interceptor: &Interceptor{

				Before: []intercept.Func{
					bot.localizerInterceptor,
					bot.loadUserInterceptor,
					bot.answerCallbackInterceptor,
					bot.lockInterceptor,
				},
				OnDefer: []intercept.Func{
					bot.unlockInterceptor,
				}},
		},
		{
			Endpoints: []interface{}{&shopItemSettingsBackButton},
			Handler:   bot.displayShopItemHandler,
			Interceptor: &Interceptor{

				Before: []intercept.Func{
					bot.localizerInterceptor,
					bot.loadUserInterceptor,
					bot.answerCallbackInterceptor,
					bot.lockInterceptor,
				},
				OnDefer: []intercept.Func{
					bot.unlockInterceptor,
				}},
		},
		{
			Endpoints: []interface{}{&shopItemDeleteButton},
			Handler:   bot.shopItemDeleteHandler,
			Interceptor: &Interceptor{

				Before: []intercept.Func{
					bot.localizerInterceptor,
					bot.loadUserInterceptor,
					bot.answerCallbackInterceptor,
					bot.lockInterceptor,
				},
				OnDefer: []intercept.Func{
					bot.unlockInterceptor,
				}},
		},
		{
			Endpoints: []interface{}{&shopItemPriceButton},
			Handler:   bot.shopItemPriceHandler,
			Interceptor: &Interceptor{

				Before: []intercept.Func{
					bot.localizerInterceptor,
					bot.loadUserInterceptor,
					bot.answerCallbackInterceptor,
					bot.lockInterceptor,
				},
				OnDefer: []intercept.Func{
					bot.unlockInterceptor,
				}},
		},
		{
			Endpoints: []interface{}{&shopItemTitleButton},
			Handler:   bot.shopItemTitleHandler,
			Interceptor: &Interceptor{

				Before: []intercept.Func{
					bot.localizerInterceptor,
					bot.loadUserInterceptor,
					bot.answerCallbackInterceptor,
					bot.lockInterceptor,
				},
				OnDefer: []intercept.Func{
					bot.unlockInterceptor,
				}},
		},
		{
			Endpoints: []interface{}{&shopItemAddFileButton},
			Handler:   bot.shopItemAddItemHandler,
			Interceptor: &Interceptor{

				Before: []intercept.Func{
					bot.localizerInterceptor,
					bot.loadUserInterceptor,
					bot.answerCallbackInterceptor,
					bot.lockInterceptor,
				},
				OnDefer: []intercept.Func{
					bot.unlockInterceptor,
				}},
		},
		{
			Endpoints: []interface{}{&shopItemBuyButton},
			Handler:   bot.shopConfirmBuyHandler,
			Interceptor: &Interceptor{

				Before: []intercept.Func{
					bot.localizerInterceptor,
					bot.loadUserInterceptor,
					bot.answerCallbackInterceptor,
					bot.lockInterceptor,
				},
				OnDefer: []intercept.Func{
					bot.unlockInterceptor,
				}},
		},
		{
			Endpoints: []interface{}{&shopItemCancelBuyButton},
			Handler:   bot.displayShopItemHandler,
			Interceptor: &Interceptor{

				Before: []intercept.Func{
					bot.localizerInterceptor,
					bot.loadUserInterceptor,
					bot.answerCallbackInterceptor,
					bot.lockInterceptor,
				},
				OnDefer: []intercept.Func{
					bot.unlockInterceptor,
				}},
		},
	}
}
