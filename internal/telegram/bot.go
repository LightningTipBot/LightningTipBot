package telegram

import (
	"fmt"
<<<<<<< HEAD:internal/telegram/bot.go
	"github.com/LightningTipBot/LightningTipBot/internal/limiter"
	"golang.org/x/time/rate"
	"sync"
	"time"

	"github.com/eko/gocache/store"

	"github.com/LightningTipBot/LightningTipBot/internal"
	"github.com/LightningTipBot/LightningTipBot/internal/lnbits"
=======
	"github.com/LightningTipBot/LightningTipBot/internal/i18n"
	"github.com/LightningTipBot/LightningTipBot/internal/lnbits"
	"github.com/LightningTipBot/LightningTipBot/internal/lnurl"
	"github.com/LightningTipBot/LightningTipBot/internal/rate"
>>>>>>> 51bd3ea (differentiate between global and chat rate limiting):bot.go
	"github.com/LightningTipBot/LightningTipBot/internal/storage"
	gocache "github.com/patrickmn/go-cache"
	log "github.com/sirupsen/logrus"
	"gopkg.in/lightningtipbot/telebot.v2"
	tb "gopkg.in/lightningtipbot/telebot.v2"
	"gorm.io/gorm"
	"sync"
	"time"
)

type TipBot struct {
	Database *gorm.DB
	Bunt     *storage.DB
	logger   *gorm.DB
<<<<<<< HEAD:internal/telegram/bot.go
	Telegram *telebot.Bot
	Client   *lnbits.Client
	Cache
}
type Cache struct {
	*store.GoCacheStore
=======
	telegram *telebot.Bot
	client   *lnbits.Client
	bundle   *i18n2.Bundle
<<<<<<< HEAD:internal/telegram/bot.go
	limiter  *limiter.ChatIDRateLimiter
>>>>>>> 2dc80c7 (add rate limiter per chat):bot.go
=======
	limiter  *rate.Limiter
>>>>>>> 51bd3ea (differentiate between global and chat rate limiting):bot.go
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
	return TipBot{
		Database: db,
		Client:   lnbits.NewClient(internal.Configuration.Lnbits.AdminKey, internal.Configuration.Lnbits.Url),
		logger:   txLogger,
<<<<<<< HEAD:internal/telegram/bot.go
		Bunt:     createBunt(),
		Telegram: newTelegramBot(),
		Cache:    Cache{GoCacheStore: gocacheStore},
=======
		bunt:     storage.NewBunt(Configuration.Database.BuntDbPath),
		bundle:   i18n.RegisterLanguages(),
<<<<<<< HEAD:internal/telegram/bot.go
		limiter:  limiter.NewChatIDRateLimiter(rate.Limit(30), 30),
>>>>>>> 2dc80c7 (add rate limiter per chat):bot.go
=======
		limiter:  rate.NewLimiter(),
>>>>>>> 51bd3ea (differentiate between global and chat rate limiting):bot.go
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
func (bot TipBot) Start() {
	log.Infof("[Telegram] Authorized on account @%s", bot.Telegram.Me.Username)
	// initialize the bot wallet
	err := bot.initBotWallet()
	if err != nil {
		log.Errorf("Could not initialize bot wallet: %s", err.Error())
	}
	bot.registerTelegramHandlers()
	bot.Telegram.Start()
}
