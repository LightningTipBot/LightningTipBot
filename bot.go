package main

import (
	"fmt"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/LightningTipBot/LightningTipBot/internal/lnbits"
	"gopkg.in/tucnak/telebot.v2"
	tb "gopkg.in/tucnak/telebot.v2"

	"gorm.io/gorm"
)

type TipBot struct {
	database *gorm.DB
	logger   *gorm.DB
	telegram *telebot.Bot
	client   *lnbits.Client
	tips     map[int][]*Message
}

var (
	paymentConfirmationMenu = &tb.ReplyMarkup{ResizeReplyKeyboard: true, OneTimeKeyboard: true}
	btnCancelPay            = paymentConfirmationMenu.Data("🚫 Cancel", "cancel")
	btnPay                  = paymentConfirmationMenu.Data("✅ Pay", "pay")
	sendConfirmationMenu    = &tb.ReplyMarkup{ResizeReplyKeyboard: true, OneTimeKeyboard: true}
	btnCancelSend           = sendConfirmationMenu.Data("🚫 Cancel", "cancel")
	btnSend                 = sendConfirmationMenu.Data("✅ Pay", "pay")
)

// NewBot migrates data and creates a new bot
func NewBot() TipBot {
	db, txLogger := migration()
	return TipBot{
		database: db,
		logger:   txLogger,
		tips:     make(map[int][]*Message, 0),
	}
}

// Start will initialize the all social media bots and lnbits.
func (bot TipBot) Start() {
	// set logger
	setLogger()
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
	bot.telegram.Start()
}

// setLogger will initialize the log format
func setLogger() {
	log.SetLevel(log.DebugLevel)
	customFormatter := new(log.TextFormatter)
	customFormatter.TimestampFormat = "2006-01-02 15:04:05"
	customFormatter.FullTimestamp = true
	log.SetFormatter(customFormatter)
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

var botWalletInitialisation = sync.Once{}

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

var telegramHandlerRegistration = sync.Once{}

// registerTelegramHandlers will register all telegram handlers.
func (bot TipBot) registerTelegramHandlers() {
	telegramHandlerRegistration.Do(func() {
		// Set up handlers
		var endpointHandler = map[string]interface{}{
			"/tip":     bot.tipHandler,
			"/pay":     bot.confirmPaymentHandler,
			"/invoice": bot.invoiceHandler,
			"/balance": bot.balanceHandler,
			"/start":   bot.startHandler,
			"/send":    bot.confirmSendHandler,
			"/help":    bot.helpHandler,
			"/info":    bot.infoHandler,
		}
		// assign handler to endpoint
		for endpoint, handler := range endpointHandler {
			log.Debugf("Registering: %s", endpoint)
			bot.telegram.Handle(endpoint, handler)
			// register upper case versions as well
			bot.telegram.Handle(strings.ToUpper(endpoint), handler)
		}

		// button handlers
		// for /pay
		bot.telegram.Handle(&btnPay, bot.payHandler)
		bot.telegram.Handle(&btnCancelPay, bot.cancelPaymentHandler)
		// for /send
		bot.telegram.Handle(&btnSend, bot.sendHandler)
		bot.telegram.Handle(&btnCancelSend, bot.cancelSendHandler)

		// basic handlers
		bot.telegram.Handle(tb.OnText, func(m *tb.Message) {
			log.Infof("[%s:%d %s:%d] %s", m.Chat.Title, m.Chat.ID, GetUserStr(m.Sender), m.Sender.ID, m.Text)
			bot.anyTextHandler(m)
		})
	})
}

func (bot TipBot) anyTextHandler(m *tb.Message) {
	if m.Chat.Type != tb.ChatPrivate {
		return
	}
	// could be an invoice
	invoice_str := strings.ToLower(m.Text)
	if strings.HasPrefix(invoice_str, "lnbc") || strings.HasPrefix(invoice_str, "lightning:lnbc") {
		// if it's only one word
		if !strings.Contains(invoice_str, " ") {
			m.Text = "/pay " + invoice_str
			bot.confirmPaymentHandler(m)
		}
	}
}

func removeTip(tips []*Message, s int) []*Message {
	if len(tips) == 1 {
		return make([]*Message, 0)
	}
	return append(tips[:s], tips[s+1:]...)
}

func (bot *TipBot) cleanupTipsFromMemory() {
	go func() {
		defer withRecovery()
		for {
			for userId, userTips := range bot.tips {
				for i, tip := range userTips {
					if time.Now().Sub(tip.LastTip) > (time.Hour*24)*7 {
						userTips = removeTip(userTips, i)
						err := bot.telegram.Delete(tip.Message)
						if err != nil {
							log.WithField("error", err.Error()).Error("could not delete tip tool tip")
						}
						bot.tips[userId] = userTips
					}
				}

			}
			time.Sleep(time.Hour)
		}
	}()
}
