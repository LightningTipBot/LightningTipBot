package telegram

import (
	"context"
	"fmt"
	"strings"

	"github.com/LightningTipBot/LightningTipBot/internal/telegram/intercept"
	log "github.com/sirupsen/logrus"
	tb "gopkg.in/lightningtipbot/telebot.v2"
)

type Handler struct {
	Endpoints   []interface{}
	Handler     interface{}
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

// registerHandlerWithInterceptor will register a handler with all the predefined interceptors, based on the interceptor type
func (bot TipBot) registerHandlerWithInterceptor(h Handler) {
	h.Interceptor.Before = append(getDefaultBeforeInterceptor(bot), h.Interceptor.Before...)
	//h.Interceptor.After = append(h.Interceptor.After, getDefaultAfterInterceptor(bot)...)
	//h.Interceptor.OnDefer = append(h.Interceptor.OnDefer, getDefaultDeferInterceptor(bot)...)

	switch h.Interceptor.Type {
	case MessageInterceptor:
		for _, endpoint := range h.Endpoints {
			bot.handle(endpoint, intercept.HandlerWithMessage(h.Handler.(func(ctx context.Context, query *tb.Message) (context.Context, error)),
				intercept.WithBeforeMessage(h.Interceptor.Before...),
				intercept.WithAfterMessage(h.Interceptor.After...),
				intercept.WithDeferMessage(h.Interceptor.OnDefer...)))
		}
	case QueryInterceptor:
		for _, endpoint := range h.Endpoints {
			bot.handle(endpoint, intercept.HandlerWithQuery(h.Handler.(func(ctx context.Context, query *tb.Query) (context.Context, error)),
				intercept.WithBeforeQuery(h.Interceptor.Before...),
				intercept.WithAfterQuery(h.Interceptor.After...),
				intercept.WithDeferQuery(h.Interceptor.OnDefer...)))
		}
	case CallbackInterceptor:
		for _, endpoint := range h.Endpoints {
			bot.handle(endpoint, intercept.HandlerWithCallback(h.Handler.(func(ctx context.Context, callback *tb.Callback) (context.Context, error)),
				intercept.WithBeforeCallback(h.Interceptor.Before...),
				intercept.WithAfterCallback(h.Interceptor.After...),
				intercept.WithDeferCallback(append(h.Interceptor.OnDefer, bot.answerCallbackInterceptor)...)))
		}
	}
}

// handle accepts an endpoint and handler for Telegram handler registration.
// function will automatically register string handlers as uppercase and first letter uppercase.
func (bot TipBot) handle(endpoint interface{}, handler interface{}) {
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
func (bot TipBot) register(h Handler) {
	if h.Interceptor != nil {
		bot.registerHandlerWithInterceptor(h)
	} else {
		for _, endpoint := range h.Endpoints {
			bot.handle(endpoint, h.Handler)
		}
	}
}

// getHandler returns a list of all handlers, that need to be registered with Telegram
func (bot TipBot) getHandler() []Handler {
	return []Handler{
		{
			Endpoints: []interface{}{"/start"},
			Handler:   bot.startHandler,
			Interceptor: &Interceptor{
				Type: MessageInterceptor,
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
			Endpoints: []interface{}{"/tip"},
			Handler:   bot.tipHandler,
			Interceptor: &Interceptor{
				Type: MessageInterceptor,
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
				Type: MessageInterceptor,
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
			Endpoints: []interface{}{"/invoice", &btnInvoiceMainMenu},
			Handler:   bot.invoiceHandler,
			Interceptor: &Interceptor{
				Type: MessageInterceptor,
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
			Endpoints: []interface{}{"/shops"},
			Handler:   bot.shopsHandler,
			Interceptor: &Interceptor{
				Type: MessageInterceptor,
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
				Type: MessageInterceptor,
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
		{
			Endpoints: []interface{}{"/balance", &btnBalanceMainMenu},
			Handler:   bot.balanceHandler,
			Interceptor: &Interceptor{
				Type: MessageInterceptor,
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
			Endpoints: []interface{}{"/send", &btnSendMenuEnter},
			Handler:   bot.sendHandler,
			Interceptor: &Interceptor{
				Type: MessageInterceptor,
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
				Type: MessageInterceptor,
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
			Endpoints: []interface{}{"/faucet", "/zapfhahn", "/kraan", "/grifo"},
			Handler:   bot.faucetHandler,
			Interceptor: &Interceptor{
				Type: MessageInterceptor,
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
				Type: MessageInterceptor,
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
				Type: MessageInterceptor,
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
				Type: MessageInterceptor,
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
				Type: MessageInterceptor,
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
				Type: MessageInterceptor,
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
			Endpoints: []interface{}{"/link"},
			Handler:   bot.lndhubHandler,
			Interceptor: &Interceptor{
				Type: MessageInterceptor,
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
			Endpoints: []interface{}{"/lnurl"},
			Handler:   bot.lnurlHandler,
			Interceptor: &Interceptor{
				Type: MessageInterceptor,
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
			Endpoints: []interface{}{tb.OnPhoto},
			Handler:   bot.photoHandler,
			Interceptor: &Interceptor{
				Type: MessageInterceptor,
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
				Type: MessageInterceptor,
				Before: []intercept.Func{
					bot.requirePrivateChatInterceptor,
					bot.logMessageInterceptor,
					bot.loadUserInterceptor}},
		},
		{
			Endpoints: []interface{}{tb.OnText},
			Handler:   bot.anyTextHandler,
			Interceptor: &Interceptor{
				Type: MessageInterceptor,
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
				Type: QueryInterceptor,
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
			Endpoints: []interface{}{tb.OnChosenInlineResult},
			Handler:   bot.anyChosenInlineHandler,
		},
		{
			Endpoints: []interface{}{&btnPay},
			Handler:   bot.confirmPayHandler,
			Interceptor: &Interceptor{
				Type: CallbackInterceptor,
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
			Endpoints: []interface{}{&btnCancelPay},
			Handler:   bot.cancelPaymentHandler,
			Interceptor: &Interceptor{
				Type: CallbackInterceptor,
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
			Endpoints: []interface{}{&btnSend},
			Handler:   bot.confirmSendHandler,
			Interceptor: &Interceptor{
				Type: CallbackInterceptor,
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
			Endpoints: []interface{}{&btnCancelSend},
			Handler:   bot.cancelSendHandler,
			Interceptor: &Interceptor{
				Type: CallbackInterceptor,
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
			Endpoints: []interface{}{&btnAcceptInlineSend},
			Handler:   bot.acceptInlineSendHandler,
			Interceptor: &Interceptor{
				Type: CallbackInterceptor,
				Before: []intercept.Func{
					bot.localizerInterceptor,
					bot.loadUserInterceptor,
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
				Type: CallbackInterceptor,
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
			Endpoints: []interface{}{&btnAcceptInlineReceive},
			Handler:   bot.acceptInlineReceiveHandler,
			Interceptor: &Interceptor{
				Type: CallbackInterceptor,
				Before: []intercept.Func{
					bot.localizerInterceptor,
					bot.loadUserInterceptor,
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
				Type: CallbackInterceptor,
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
			Endpoints: []interface{}{&btnAcceptInlineFaucet},
			Handler:   bot.acceptInlineFaucetHandler,
			Interceptor: &Interceptor{
				Type: CallbackInterceptor,
				Before: []intercept.Func{
					bot.singletonCallbackInterceptor,
					bot.localizerInterceptor,
					bot.loadUserInterceptor,
					bot.lockInterceptor,
				},
				OnDefer: []intercept.Func{
					bot.unlockInterceptor,
				},
			},
		},
		{
			Endpoints: []interface{}{&btnCancelInlineFaucet},
			Handler:   bot.cancelInlineFaucetHandler,
			Interceptor: &Interceptor{
				Type: CallbackInterceptor,
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
			Endpoints: []interface{}{&btnAcceptInlineTipjar},
			Handler:   bot.acceptInlineTipjarHandler,
			Interceptor: &Interceptor{
				Type: CallbackInterceptor,
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
			Endpoints: []interface{}{&btnCancelInlineTipjar},
			Handler:   bot.cancelInlineTipjarHandler,
			Interceptor: &Interceptor{
				Type: CallbackInterceptor,
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
			Endpoints: []interface{}{&btnWithdraw},
			Handler:   bot.confirmWithdrawHandler,
			Interceptor: &Interceptor{
				Type: CallbackInterceptor,
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
			Endpoints: []interface{}{&btnCancelWithdraw},
			Handler:   bot.cancelWithdrawHandler,
			Interceptor: &Interceptor{
				Type: CallbackInterceptor,
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
			Endpoints: []interface{}{&shopNewShopButton},
			Handler:   bot.shopNewShopHandler,
			Interceptor: &Interceptor{
				Type: CallbackInterceptor,
				Before: []intercept.Func{
					bot.localizerInterceptor,
					bot.loadUserInterceptor,
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
				Type: CallbackInterceptor,
				Before: []intercept.Func{
					bot.localizerInterceptor,
					bot.loadUserInterceptor,
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
				Type: CallbackInterceptor,
				Before: []intercept.Func{
					bot.localizerInterceptor,
					bot.loadUserInterceptor,
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
				Type: CallbackInterceptor,
				Before: []intercept.Func{
					bot.localizerInterceptor,
					bot.loadUserInterceptor,
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
				Type: CallbackInterceptor,
				Before: []intercept.Func{
					bot.localizerInterceptor,
					bot.loadUserInterceptor,
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
				Type: CallbackInterceptor,
				Before: []intercept.Func{
					bot.localizerInterceptor,
					bot.loadUserInterceptor,
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
				Type: CallbackInterceptor,
				Before: []intercept.Func{
					bot.localizerInterceptor,
					bot.loadUserInterceptor,
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
				Type: CallbackInterceptor,
				Before: []intercept.Func{
					bot.localizerInterceptor,
					bot.loadUserInterceptor,
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
				Type: CallbackInterceptor,
				Before: []intercept.Func{
					bot.localizerInterceptor,
					bot.loadUserInterceptor,
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
				Type: CallbackInterceptor,
				Before: []intercept.Func{
					bot.localizerInterceptor,
					bot.loadUserInterceptor,
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
				Type: CallbackInterceptor,
				Before: []intercept.Func{
					bot.localizerInterceptor,
					bot.loadUserInterceptor,
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
				Type: CallbackInterceptor,
				Before: []intercept.Func{
					bot.localizerInterceptor,
					bot.loadUserInterceptor,
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
				Type: CallbackInterceptor,
				Before: []intercept.Func{
					bot.localizerInterceptor,
					bot.loadUserInterceptor,
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
				Type: CallbackInterceptor,
				Before: []intercept.Func{
					bot.localizerInterceptor,
					bot.loadUserInterceptor,
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
				Type: CallbackInterceptor,
				Before: []intercept.Func{
					bot.localizerInterceptor,
					bot.loadUserInterceptor,
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
				Type: CallbackInterceptor,
				Before: []intercept.Func{
					bot.localizerInterceptor,
					bot.loadUserInterceptor,
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
				Type: CallbackInterceptor,
				Before: []intercept.Func{
					bot.localizerInterceptor,
					bot.loadUserInterceptor,
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
				Type: CallbackInterceptor,
				Before: []intercept.Func{
					bot.localizerInterceptor,
					bot.loadUserInterceptor,
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
				Type: CallbackInterceptor,
				Before: []intercept.Func{
					bot.localizerInterceptor,
					bot.loadUserInterceptor,
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
				Type: CallbackInterceptor,
				Before: []intercept.Func{
					bot.localizerInterceptor,
					bot.loadUserInterceptor,
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
				Type: CallbackInterceptor,
				Before: []intercept.Func{
					bot.localizerInterceptor,
					bot.loadUserInterceptor,
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
				Type: CallbackInterceptor,
				Before: []intercept.Func{
					bot.localizerInterceptor,
					bot.loadUserInterceptor,
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
				Type: CallbackInterceptor,
				Before: []intercept.Func{
					bot.localizerInterceptor,
					bot.loadUserInterceptor,
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
				Type: CallbackInterceptor,
				Before: []intercept.Func{
					bot.localizerInterceptor,
					bot.loadUserInterceptor,
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
				Type: CallbackInterceptor,
				Before: []intercept.Func{
					bot.localizerInterceptor,
					bot.loadUserInterceptor,
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
				Type: CallbackInterceptor,
				Before: []intercept.Func{
					bot.localizerInterceptor,
					bot.loadUserInterceptor,
					bot.lockInterceptor,
				},
				OnDefer: []intercept.Func{
					bot.unlockInterceptor,
				}},
		},
	}
}
