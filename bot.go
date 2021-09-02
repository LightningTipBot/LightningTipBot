package main

import (
	"fmt"
	"github.com/LightningTipBot/LightningTipBot/internal/storage"
	"strings"
	"sync"
	"time"

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
		bunt:     storage.NewBunt(Configuration.BuntDbPath),
	}
}

// newTelegramBot will create a new telegram bot.
func newTelegramBot() *tb.Bot {
	tgb, err := tb.NewBot(tb.Settings{
		Token:     Configuration.ApiKey,
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
		err := bot.initWallet(bot.telegram.Me)
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
		var endpointHandler = map[string]interface{}{
			"/tip":      bot.tipHandler,
			"/pay":      bot.confirmPaymentHandler,
			"/invoice":  bot.invoiceHandler,
			"/balance":  bot.balanceHandler,
			"/start":    bot.startHandler,
			"/send":     bot.confirmSendHandler,
			"/help":     bot.helpHandler,
			"/info":     bot.infoHandler,
			"/donate":   bot.donationHandler,
			"/advanced": bot.advancedHelpHandler,
			"/link":     bot.lndhubHandler,
			"/lnurl":    bot.lnurlHandler,
			tb.OnPhoto:  bot.privatePhotoHandler,
			tb.OnText:   bot.anyTextHandler,
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

		// button handlers
		// for /pay
		bot.telegram.Handle(&btnPay, bot.payHandler)
		bot.telegram.Handle(&btnCancelPay, bot.cancelPaymentHandler)
		// for /send
		bot.telegram.Handle(&btnSend, bot.sendHandler)
		bot.telegram.Handle(&btnCancelSend, bot.cancelSendHandler)

	})
}

// Start will initialize the telegram bot and lnbits.
func (bot TipBot) Start() {
	// set up lnbits api
	bot.client = lnbits.NewClient(Configuration.LnbitsKey, Configuration.LnbitsUrl)
	// set up telebot
	bot.telegram = newTelegramBot()
	log.Infof("[Telegram] Authorized on account @%s", bot.telegram.Me.Username)
	// initialize the bot wallet
	err := bot.initBotWallet()
	if err != nil {
		log.Errorf("Could not initialize bot wallet: %s", err.Error())
	}
	bot.registerTelegramHandlers()
	lnbits.NewWebhook(Configuration.WebhookServer, bot.telegram, bot.client, bot.database)
	lnurl.NewServer(Configuration.LNURLServer, Configuration.WebhookServer, bot.telegram, bot.client, bot.database)
	bot.telegram.Start()
}
