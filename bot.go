package main

import (
	"fmt"
	"github.com/LightningTipBot/LightningTipBot/internal/storage"
	"github.com/tidwall/buntdb"
	"github.com/tidwall/gjson"
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
			tb.OnText:   bot.anyTextHandler,
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
func setDebug(bot TipBot) {
	bot.bunt.Update(func(tx *buntdb.Tx) error {
		tx.Set("6141", `{"message":{"message_id":6141,"from":{"id":1937464902,"first_name":"lnjfdmbot","last_name":"","username":"lnjfdm_bot","language_code":"","is_bot":true,"can_join_groups":false,"can_read_all_group_messages":false,"supports_inline_queries":false},"date":1630441909,"chat":{"id":-592981286,"type":"group","title":"🔧 BOT DEVELOPMENT","first_name":"","last_name":"","username":""},"forward_from":null,"forward_from_chat":null,"forward_from_message_id":0,"forward_signature":"","forward_sender_name":"","forward_date":0,"reply_to_message":{"message_id":6053,"from":{"id":8587101,"first_name":"calle","last_name":"","username":"calllllllllle","language_code":"en","is_bot":false,"can_join_groups":false,"can_read_all_group_messages":false,"supports_inline_queries":false},"date":1630437425,"chat":{"id":-592981286,"type":"group","title":"🔧 BOT DEVELOPMENT","first_name":"","last_name":"","username":""},"forward_from":{"id":1795496248,"first_name":"LightningTipBot","last_name":"","username":"LightningTipBot","language_code":"","is_bot":true,"can_join_groups":false,"can_read_all_group_messages":false,"supports_inline_queries":false},"forward_from_chat":null,"forward_from_message_id":0,"forward_signature":"","forward_sender_name":"","forward_date":1630437384,"reply_to_message":null,"via_bot":null,"edit_date":0,"media_group_id":"","author_signature":"","text":"📖 Oops, that didn't work. Did you enter an amount?\n\nUsage: /invoice \u003camount\u003e [\u003cmemo\u003e]\nExample: /invoice 1000 Take this! 💸","entities":[{"type":"bold","offset":53,"length":6},{"type":"code","offset":60,"length":26},{"type":"bold","offset":87,"length":8},{"type":"code","offset":96,"length":27}],"audio":null,"document":null,"photo":null,"sticker":null,"voice":null,"video_note":null,"video":null,"animation":null,"contact":null,"location":null,"venue":null,"poll":null,"dice":null,"new_chat_member":null,"left_chat_member":null,"new_chat_title":"","new_chat_photo":null,"new_chat_members":null,"delete_chat_photo":false,"group_chat_created":false,"supergroup_chat_created":false,"channel_chat_created":false,"migrate_to_chat_id":0,"migrate_from_chat_id":0,"pinned_message":null,"invoice":null,"successful_payment":null,"reply_markup":{}},"via_bot":null,"edit_date":0,"media_group_id":"","author_signature":"","text":"🏅 1 sat (by @gohumble)","entities":[{"type":"mention","offset":13,"length":9}],"audio":null,"document":null,"photo":null,"sticker":null,"voice":null,"video_note":null,"video":null,"animation":null,"contact":null,"location":null,"venue":null,"poll":null,"dice":null,"new_chat_member":null,"left_chat_member":null,"new_chat_title":"","new_chat_photo":null,"new_chat_members":null,"delete_chat_photo":false,"group_chat_created":false,"supergroup_chat_created":false,"channel_chat_created":false,"migrate_to_chat_id":0,"migrate_from_chat_id":0,"pinned_message":null,"invoice":null,"successful_payment":null,"reply_markup":{}},"tip_amount":1,"ntips":1,"last_tip":"2021-08-31T22:31:49.325436+02:00","tippers":[{"id":125296974,"first_name":"Be","last_name":"Humble","username":"gohumble","language_code":"de","is_bot":false,"can_join_groups":false,"can_read_all_group_messages":false,"supports_inline_queries":false}]}`, nil)
		tx.Set("6142", `{"message":{"message_id":6142,"from":{"id":1937464902,"first_name":"lnjfdmbot","last_name":"","username":"lnjfdm_bot","language_code":"","is_bot":true,"can_join_groups":false,"can_read_all_group_messages":false,"supports_inline_queries":false},"date":1630441909,"chat":{"id":-592981286,"type":"group","title":"🔧 BOT DEVELOPMENT","first_name":"","last_name":"","username":""},"forward_from":null,"forward_from_chat":null,"forward_from_message_id":0,"forward_signature":"","forward_sender_name":"","forward_date":0,"reply_to_message":{"message_id":6054,"from":{"id":8587102,"first_name":"calle","last_name":"","username":"calllllllllle","language_code":"en","is_bot":false,"can_join_groups":false,"can_read_all_group_messages":false,"supports_inline_queries":false},"date":1630437425,"chat":{"id":-592981286,"type":"group","title":"🔧 BOT DEVELOPMENT","first_name":"","last_name":"","username":""},"forward_from":{"id":1795496248,"first_name":"LightningTipBot","last_name":"","username":"LightningTipBot","language_code":"","is_bot":true,"can_join_groups":false,"can_read_all_group_messages":false,"supports_inline_queries":false},"forward_from_chat":null,"forward_from_message_id":0,"forward_signature":"","forward_sender_name":"","forward_date":1630437384,"reply_to_message":null,"via_bot":null,"edit_date":0,"media_group_id":"","author_signature":"","text":"📖 Oops, that didn't work. Did you enter an amount?\n\nUsage: /invoice \u003camount\u003e [\u003cmemo\u003e]\nExample: /invoice 1000 Take this! 💸","entities":[{"type":"bold","offset":53,"length":6},{"type":"code","offset":60,"length":26},{"type":"bold","offset":87,"length":8},{"type":"code","offset":96,"length":27}],"audio":null,"document":null,"photo":null,"sticker":null,"voice":null,"video_note":null,"video":null,"animation":null,"contact":null,"location":null,"venue":null,"poll":null,"dice":null,"new_chat_member":null,"left_chat_member":null,"new_chat_title":"","new_chat_photo":null,"new_chat_members":null,"delete_chat_photo":false,"group_chat_created":false,"supergroup_chat_created":false,"channel_chat_created":false,"migrate_to_chat_id":0,"migrate_from_chat_id":0,"pinned_message":null,"invoice":null,"successful_payment":null,"reply_markup":{}},"via_bot":null,"edit_date":0,"media_group_id":"","author_signature":"","text":"🏅 1 sat (by @gohumble)","entities":[{"type":"mention","offset":13,"length":9}],"audio":null,"document":null,"photo":null,"sticker":null,"voice":null,"video_note":null,"video":null,"animation":null,"contact":null,"location":null,"venue":null,"poll":null,"dice":null,"new_chat_member":null,"left_chat_member":null,"new_chat_title":"","new_chat_photo":null,"new_chat_members":null,"delete_chat_photo":false,"group_chat_created":false,"supergroup_chat_created":false,"channel_chat_created":false,"migrate_to_chat_id":0,"migrate_from_chat_id":0,"pinned_message":null,"invoice":null,"successful_payment":null,"reply_markup":{}},"tip_amount":1,"ntips":1,"last_tip":"2021-08-31T22:31:49.325436+02:00","tippers":[{"id":125296974,"first_name":"Be","last_name":"Humble","username":"gohumble","language_code":"de","is_bot":false,"can_join_groups":false,"can_read_all_group_messages":false,"supports_inline_queries":false}]}`, nil)
		tx.Set("6143", `{"message":{"message_id":6143,"from":{"id":1937464902,"first_name":"lnjfdmbot","last_name":"","username":"lnjfdm_bot","language_code":"","is_bot":true,"can_join_groups":false,"can_read_all_group_messages":false,"supports_inline_queries":false},"date":1630441909,"chat":{"id":-592981286,"type":"group","title":"🔧 BOT DEVELOPMENT","first_name":"","last_name":"","username":""},"forward_from":null,"forward_from_chat":null,"forward_from_message_id":0,"forward_signature":"","forward_sender_name":"","forward_date":0,"reply_to_message":{"message_id":6055,"from":{"id":8587101,"first_name":"calle","last_name":"","username":"calllllllllle","language_code":"en","is_bot":false,"can_join_groups":false,"can_read_all_group_messages":false,"supports_inline_queries":false},"date":1630437425,"chat":{"id":-592981286,"type":"group","title":"🔧 BOT DEVELOPMENT","first_name":"","last_name":"","username":""},"forward_from":{"id":1795496248,"first_name":"LightningTipBot","last_name":"","username":"LightningTipBot","language_code":"","is_bot":true,"can_join_groups":false,"can_read_all_group_messages":false,"supports_inline_queries":false},"forward_from_chat":null,"forward_from_message_id":0,"forward_signature":"","forward_sender_name":"","forward_date":1630437384,"reply_to_message":null,"via_bot":null,"edit_date":0,"media_group_id":"","author_signature":"","text":"📖 Oops, that didn't work. Did you enter an amount?\n\nUsage: /invoice \u003camount\u003e [\u003cmemo\u003e]\nExample: /invoice 1000 Take this! 💸","entities":[{"type":"bold","offset":53,"length":6},{"type":"code","offset":60,"length":26},{"type":"bold","offset":87,"length":8},{"type":"code","offset":96,"length":27}],"audio":null,"document":null,"photo":null,"sticker":null,"voice":null,"video_note":null,"video":null,"animation":null,"contact":null,"location":null,"venue":null,"poll":null,"dice":null,"new_chat_member":null,"left_chat_member":null,"new_chat_title":"","new_chat_photo":null,"new_chat_members":null,"delete_chat_photo":false,"group_chat_created":false,"supergroup_chat_created":false,"channel_chat_created":false,"migrate_to_chat_id":0,"migrate_from_chat_id":0,"pinned_message":null,"invoice":null,"successful_payment":null,"reply_markup":{}},"via_bot":null,"edit_date":0,"media_group_id":"","author_signature":"","text":"🏅 1 sat (by @gohumble)","entities":[{"type":"mention","offset":13,"length":9}],"audio":null,"document":null,"photo":null,"sticker":null,"voice":null,"video_note":null,"video":null,"animation":null,"contact":null,"location":null,"venue":null,"poll":null,"dice":null,"new_chat_member":null,"left_chat_member":null,"new_chat_title":"","new_chat_photo":null,"new_chat_members":null,"delete_chat_photo":false,"group_chat_created":false,"supergroup_chat_created":false,"channel_chat_created":false,"migrate_to_chat_id":0,"migrate_from_chat_id":0,"pinned_message":null,"invoice":null,"successful_payment":null,"reply_markup":{}},"tip_amount":1,"ntips":1,"last_tip":"2021-08-31T22:31:49.325436+02:00","tippers":[{"id":125296974,"first_name":"Be","last_name":"Humble","username":"gohumble","language_code":"de","is_bot":false,"can_join_groups":false,"can_read_all_group_messages":false,"supports_inline_queries":false}]}`, nil)
		tx.Set("6141", `{"message":{"message_id":6141,"from":{"id":1937464902,"first_name":"lnjfdmbot","last_name":"","username":"lnjfdm_bot","language_code":"","is_bot":true,"can_join_groups":false,"can_read_all_group_messages":false,"supports_inline_queries":false},"date":1630441909,"chat":{"id":-592981286,"type":"group","title":"🔧 BOT DEVELOPMENT","first_name":"","last_name":"","username":""},"forward_from":null,"forward_from_chat":null,"forward_from_message_id":0,"forward_signature":"","forward_sender_name":"","forward_date":0,"reply_to_message":{"message_id":6053,"from":{"id":8587101,"first_name":"calle","last_name":"","username":"calllllllllle","language_code":"en","is_bot":false,"can_join_groups":false,"can_read_all_group_messages":false,"supports_inline_queries":false},"date":1630437425,"chat":{"id":-592981286,"type":"group","title":"🔧 BOT DEVELOPMENT","first_name":"","last_name":"","username":""},"forward_from":{"id":1795496248,"first_name":"LightningTipBot","last_name":"","username":"LightningTipBot","language_code":"","is_bot":true,"can_join_groups":false,"can_read_all_group_messages":false,"supports_inline_queries":false},"forward_from_chat":null,"forward_from_message_id":0,"forward_signature":"","forward_sender_name":"","forward_date":1630437384,"reply_to_message":null,"via_bot":null,"edit_date":0,"media_group_id":"","author_signature":"","text":"📖 Oops, that didn't work. Did you enter an amount?\n\nUsage: /invoice \u003camount\u003e [\u003cmemo\u003e]\nExample: /invoice 1000 Take this! 💸","entities":[{"type":"bold","offset":53,"length":6},{"type":"code","offset":60,"length":26},{"type":"bold","offset":87,"length":8},{"type":"code","offset":96,"length":27}],"audio":null,"document":null,"photo":null,"sticker":null,"voice":null,"video_note":null,"video":null,"animation":null,"contact":null,"location":null,"venue":null,"poll":null,"dice":null,"new_chat_member":null,"left_chat_member":null,"new_chat_title":"","new_chat_photo":null,"new_chat_members":null,"delete_chat_photo":false,"group_chat_created":false,"supergroup_chat_created":false,"channel_chat_created":false,"migrate_to_chat_id":0,"migrate_from_chat_id":0,"pinned_message":null,"invoice":null,"successful_payment":null,"reply_markup":{}},"via_bot":null,"edit_date":0,"media_group_id":"","author_signature":"","text":"🏅 1 sat (by @gohumble)","entities":[{"type":"mention","offset":13,"length":9}],"audio":null,"document":null,"photo":null,"sticker":null,"voice":null,"video_note":null,"video":null,"animation":null,"contact":null,"location":null,"venue":null,"poll":null,"dice":null,"new_chat_member":null,"left_chat_member":null,"new_chat_title":"","new_chat_photo":null,"new_chat_members":null,"delete_chat_photo":false,"group_chat_created":false,"supergroup_chat_created":false,"channel_chat_created":false,"migrate_to_chat_id":0,"migrate_from_chat_id":0,"pinned_message":null,"invoice":null,"successful_payment":null,"reply_markup":{}},"tip_amount":1,"ntips":1,"last_tip":"2021-08-31T22:31:49.325436+02:00","tippers":[{"id":125296974,"first_name":"Be","last_name":"Humble","username":"gohumble","language_code":"de","is_bot":false,"can_join_groups":false,"can_read_all_group_messages":false,"supports_inline_queries":false}]}`, nil)
		return nil
	})
	bot.bunt.View(func(tx *buntdb.Tx) error {
		tx.Ascend(storage.MessageOrderedByReplyToFrom, func(key, value string) bool {
			replyToUserId := gjson.Get(value, storage.MessageOrderedByReplyToFrom)
			if replyToUserId.String() == "8587102" {
				fmt.Printf("%s: %s\n", key, value)
			}
			return true
		})
		return nil
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
