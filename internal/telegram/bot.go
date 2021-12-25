package telegram

import (
	"fmt"
	"sync"
	"time"

	limiter "github.com/LightningTipBot/LightningTipBot/internal/rate"

	"github.com/eko/gocache/store"

	"github.com/LightningTipBot/LightningTipBot/internal"
	"github.com/LightningTipBot/LightningTipBot/internal/lnbits"
	"github.com/LightningTipBot/LightningTipBot/internal/storage"
	gocache "github.com/patrickmn/go-cache"
	log "github.com/sirupsen/logrus"
	tb "gopkg.in/lightningtipbot/telebot.v2"
	"gorm.io/gorm"
)

type TipBot struct {
	Database *gorm.DB
	Bunt     *storage.DB
	ShopBunt *storage.DB
	logger   *gorm.DB
	Telegram *tb.Bot
	Client   *lnbits.Client
	limiter  map[string]limiter.Limiter
	Cache
}
type Cache struct {
	*store.GoCacheStore
}

var (
	botWalletInitialisation     = sync.Once{}
	telegramHandlerRegistration = sync.Once{}
)

// NewBot migrates data and creates a new bot
func NewBot() TipBot {
	gocacheClient := gocache.New(5*time.Minute, 10*time.Minute)
	gocacheStore := store.NewGoCache(gocacheClient, nil)
	// create sqlite databases
	db, txLogger := AutoMigration()
	limiter.Start()
	return TipBot{
		Database: db,
		Client:   lnbits.NewClient(internal.Configuration.Lnbits.AdminKey, internal.Configuration.Lnbits.Url),
		logger:   txLogger,
		Bunt:     createBunt(internal.Configuration.Database.BuntDbPath),
		ShopBunt: createBunt(internal.Configuration.Database.ShopBuntDbPath),
		Telegram: newTelegramBot(),
		Cache:    Cache{GoCacheStore: gocacheStore},
	}
}

// newTelegramBot will create a new Telegram bot.
func newTelegramBot() *tb.Bot {
	tgb, err := tb.NewBot(tb.Settings{
		Token:     internal.Configuration.Telegram.ApiKey,
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
		_, err := bot.initWallet(bot.Telegram.Me)
		if err != nil {
			log.Errorln(fmt.Sprintf("[initBotWallet] Could not initialize bot wallet: %s", err.Error()))
			return
		}
	})
	return nil
}

// Start will initialize the Telegram bot and lnbits.
func (bot *TipBot) Start() {
	log.Infof("[Telegram] Authorized on account @%s", bot.Telegram.Me.Username)
	// initialize the bot wallet
	err := bot.initBotWallet()
	if err != nil {
		log.Errorf("Could not initialize bot wallet: %s", err.Error())
	}
	bot.startEditWorker()
	bot.registerTelegramHandlers()
	initInvoiceEventCallbacks(bot)
	initializeStateCallbackMessage(bot)
	bot.Telegram.Start()
}
