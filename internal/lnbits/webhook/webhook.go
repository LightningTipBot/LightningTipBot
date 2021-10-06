package webhook

import (
	"encoding/json"
	"fmt"
	"github.com/LightningTipBot/LightningTipBot/internal/lnbits"
	"net/url"
	"time"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"github.com/LightningTipBot/LightningTipBot/internal/lnurl"
	"github.com/LightningTipBot/LightningTipBot/internal/storage"
	"net/http"

	"github.com/gorilla/mux"
	tb "gopkg.in/tucnak/telebot.v2"

	"github.com/LightningTipBot/LightningTipBot/internal/i18n"
)

const (
// invoiceReceivedMessage = "⚡️ You received %d sat."
)

type WebhookServer struct {
	httpServer *http.Server
	bot        *tb.Bot
	c          *lnbits.Client
	database   *gorm.DB
	buntdb     *storage.DB
}

type Webhook struct {
	CheckingID  string `json:"checking_id"`
	Pending     int    `json:"pending"`
	Amount      int    `json:"amount"`
	Fee         int    `json:"fee"`
	Memo        string `json:"memo"`
	Time        int    `json:"time"`
	Bolt11      string `json:"bolt11"`
	Preimage    string `json:"preimage"`
	PaymentHash string `json:"payment_hash"`
	Extra       struct {
	} `json:"extra"`
	WalletID      string      `json:"wallet_id"`
	Webhook       string      `json:"webhook"`
	WebhookStatus interface{} `json:"webhook_status"`
}

func NewServer(addr *url.URL, bot *tb.Bot, client *lnbits.Client, database *gorm.DB, buntdb *storage.DB) *WebhookServer {
	srv := &http.Server{
		Addr: addr.Host,
		// Good practice: enforce timeouts for servers you create!
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}
	apiServer := &WebhookServer{
		c:          client,
		database:   database,
		bot:        bot,
		httpServer: srv,
		buntdb:     buntdb,
	}
	apiServer.httpServer.Handler = apiServer.newRouter()
	go apiServer.httpServer.ListenAndServe()
	log.Infof("[Webhook] Server started at %s", addr)
	return apiServer
}

func (w *WebhookServer) GetUserByWalletId(walletId string) (*lnbits.User, error) {
	user := &lnbits.User{}
	tx := w.database.Where("wallet_id = ?", walletId).First(user)
	if tx.Error != nil {
		return user, tx.Error
	}
	return user, nil
}

func (w *WebhookServer) newRouter() *mux.Router {
	router := mux.NewRouter()
	router.HandleFunc("/", w.receive).Methods(http.MethodPost)
	return router
}

func (w WebhookServer) receive(writer http.ResponseWriter, request *http.Request) {
	depositEvent := Webhook{}
	// need to delete the header otherwise the Decode will fail
	request.Header.Del("content-length")
	err := json.NewDecoder(request.Body).Decode(&depositEvent)
	if err != nil {
		writer.WriteHeader(400)
		return
	}
	user, err := w.GetUserByWalletId(depositEvent.WalletID)
	if err != nil {
		writer.WriteHeader(400)
		return
	}
	log.Infoln(fmt.Sprintf("[WebHook] User %s (%d) received invoice of %d sat.", user.Telegram.Username, user.Telegram.ID, depositEvent.Amount/1000))
	_, err = w.bot.Send(user.Telegram, fmt.Sprintf(i18n.Translate(user.Telegram.LanguageCode, "invoiceReceivedMessage"), depositEvent.Amount/1000))
	if err != nil {
		log.Errorln(err)
	}

	// if this invoice is saved in bunt.db, we load it and display the comment from an LNURL invoice
	tx := &lnurl.LNURLInvoice{PaymentHash: depositEvent.PaymentHash}
	err = w.buntdb.Get(tx)
	if err != nil {
		log.Errorln(err)
	} else {
		_, err = w.bot.Send(user.Telegram, tx.Comment)
		if err != nil {
			log.Errorln(err)
		}
	}
	writer.WriteHeader(200)
}
