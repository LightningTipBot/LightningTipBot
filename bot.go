package main

import (
	"fmt"
	"github.com/LightningTipBot/LightningTipBot/internal/intercept"
	"strings"
	"sync"
	"time"

	"github.com/LightningTipBot/LightningTipBot/internal/storage"

	"github.com/LightningTipBot/LightningTipBot/internal/lnurl"

	log "github.com/sirupsen/logrus"

	"github.com/LightningTipBot/LightningTipBot/internal/lnbits"
	"gopkg.in/tucnak/telebot.v2"
	tb "gopkg.in/tucnak/telebot.v2"

	"gorm.io/gorm"
)

type TipBot struct {
	database *gorm.DB
	bunt     *storage.DB
	logger   *gorm.DB
	telegram *telebot.Bot
	client   *lnbits.Client
}

var (
	paymentConfirmationMenu = &tb.ReplyMarkup{ResizeReplyKeyboard: true}
	btnCancelPay            = paymentConfirmationMenu.Data("🚫 Cancel", "cancel_pay")
	btnPay                  = paymentConfirmationMenu.Data("✅ Pay", "confirm_pay")
	sendConfirmationMenu    = &tb.ReplyMarkup{ResizeReplyKeyboard: true}
	btnCancelSend           = sendConfirmationMenu.Data("🚫 Cancel", "cancel_send")
	btnSend                 = sendConfirmationMenu.Data("✅ Send", "confirm_send")

	botWalletInitialisation     = sync.Once{}
	telegramHandlerRegistration = sync.Once{}
)

// NewBot migrates data and creates a new bot
func NewBot() TipBot {
	db, txLogger := migration()
	return TipBot{
		database: db,
		logger:   txLogger,
		bunt:     storage.NewBunt(Configuration.Database.BuntDbPath),
	}
}

// newTelegramBot will create a new telegram bot.
func newTelegramBot() *tb.Bot {
	tgb, err := tb.NewBot(tb.Settings{
		Token:     Configuration.Telegram.ApiKey,
		Poller:    &tb.LongPoller{Timeout: 60 * time.Second},
		ParseMode: tb.ModeMarkdown,
	})
	if err != nil {
		panic(err)
	}
	return tgb
}

// initBotWallet will create / initialize the bot wallet
// todo -- may want to derive user wallets from this specific bot wallet (master wallet), since lnbits usermanager extension is able to do that.
func (bot TipBot) initBotWallet() error {
	botWalletInitialisation.Do(func() {
		_, err := bot.initWallet(bot.telegram.Me)
		if err != nil {
			log.Errorln(fmt.Sprintf("[initBotWallet] Could not initialize bot wallet: %s", err.Error()))
			return
		}
	})
	return nil
}

// registerTelegramHandlers will register all telegram handlers.
func (bot TipBot) registerTelegramHandlers() {
	telegramHandlerRegistration.Do(func() {
		// Set up handlers
		beforeMessage := intercept.WithBeforeMessage(bot.loadUserInterceptor)
		var endpointHandler = map[string]interface{}{
			"/tip":                  intercept.HandlerWithMessage(bot.tipHandler, intercept.WithBeforeMessage(bot.loadUserInterceptor, bot.loadReplyToInterceptor)),
			"/pay":                  intercept.HandlerWithMessage(bot.confirmPaymentHandler, beforeMessage),
			"/invoice":              intercept.HandlerWithMessage(bot.invoiceHandler, beforeMessage),
			"/balance":              intercept.HandlerWithMessage(bot.balanceHandler, beforeMessage),
			"/start":                bot.startHandler,
			"/send":                 intercept.HandlerWithMessage(bot.confirmSendHandler, beforeMessage),
			"/help":                 bot.helpHandler,
			tb.OnPhoto:              intercept.HandlerWithMessage(bot.privatePhotoHandler, beforeMessage),
			tb.OnText:               intercept.HandlerWithMessage(bot.anyTextHandler, beforeMessage),
			"/basics":               bot.basicsHandler,
			"/donate":               bot.donationHandler,
			"/advanced":             bot.advancedHelpHandler,
			"/link":                 intercept.HandlerWithMessage(bot.lndhubHandler, beforeMessage),
			"/lnurl":                intercept.HandlerWithMessage(bot.lnurlHandler, beforeMessage),
			tb.OnQuery:              intercept.HandlerWithQuery(bot.anyQueryHandler, intercept.WithBeforeQuery(bot.loadUserQueryInterceptor)),
			tb.OnChosenInlineResult: bot.anyChosenInlineHandler,
		}
		// assign handler to endpoint
		for endpoint, handler := range endpointHandler {
			log.Debugf("Registering: %s", endpoint)
			bot.telegram.Handle(endpoint, handler)

			// if the endpoint is a string command (not photo etc)
			if strings.HasPrefix(endpoint, "/") {
				// register upper case versions as well
				bot.telegram.Handle(strings.ToUpper(endpoint), handler)
			}
		}
		beforeCallback := intercept.WithBeforeCallback(bot.loadUserCallbackInterceptor)

		// button handlers
		// for /pay
		bot.telegram.Handle(&btnPay, intercept.HandlerWithCallback(bot.payHandler, beforeCallback))
		bot.telegram.Handle(&btnCancelPay, intercept.HandlerWithCallback(bot.cancelPaymentHandler, beforeCallback))
		// for /send
		bot.telegram.Handle(&btnSend, intercept.HandlerWithCallback(bot.sendHandler, beforeCallback))
		bot.telegram.Handle(&btnCancelSend, intercept.HandlerWithCallback(bot.cancelSendHandler, beforeCallback))
		// register inline button handlers
		// button for inline send
		bot.telegram.Handle(&btnSendInline, intercept.HandlerWithCallback(bot.sendInlineHandler, beforeCallback))
		bot.telegram.Handle(&btnCancelSendInline, bot.cancelSendInlineHandler)

	})
}

// Start will initialize the telegram bot and lnbits.
func (bot TipBot) Start() {
	// set up lnbits api
	bot.client = lnbits.NewClient(Configuration.Lnbits.AdminKey, Configuration.Lnbits.Url)
	// set up telebot
	bot.telegram = newTelegramBot()
	log.Infof("[Telegram] Authorized on account @%s", bot.telegram.Me.Username)
	// initialize the bot wallet
	err := bot.initBotWallet()
	if err != nil {
		log.Errorf("Could not initialize bot wallet: %s", err.Error())
	}
	bot.registerTelegramHandlers()
	lnbits.NewWebhookServer(Configuration.Lnbits.WebhookServerUrl, bot.telegram, bot.client, bot.database)
	lnurl.NewServer(Configuration.Bot.LNURLServerUrl, Configuration.Bot.LNURLHostUrl, Configuration.Lnbits.WebhookServer, bot.telegram, bot.client, bot.database)
	bot.telegram.Start()
}
