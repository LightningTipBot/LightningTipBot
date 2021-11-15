package webhook

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/LightningTipBot/LightningTipBot/internal"
	"github.com/LightningTipBot/LightningTipBot/internal/lnbits"
	"github.com/LightningTipBot/LightningTipBot/internal/str"
	"github.com/LightningTipBot/LightningTipBot/internal/telegram"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"net/http"

	"github.com/LightningTipBot/LightningTipBot/internal/lnurl"
	"github.com/LightningTipBot/LightningTipBot/internal/storage"

	"github.com/gorilla/mux"
	tb "gopkg.in/tucnak/telebot.v2"

	"github.com/LightningTipBot/LightningTipBot/internal/i18n"
)

const (
// invoiceReceivedMessage = "⚡️ You received %d sat."
)

type Server struct {
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

func NewServer(bot *telegram.TipBot) *Server {
	srv := &http.Server{
		Addr: internal.Configuration.Lnbits.WebhookServerUrl.Host,
		// Good practice: enforce timeouts for servers you create!
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}
	apiServer := &Server{
		c:          bot.Client,
		database:   bot.Database,
		bot:        bot.Telegram,
		httpServer: srv,
		buntdb:     bot.Bunt,
	}
	apiServer.httpServer.Handler = apiServer.newRouter()
	go apiServer.httpServer.ListenAndServe()
	log.Infof("[Webhook] Server started at %s", internal.Configuration.Lnbits.WebhookServerUrl)
	return apiServer
}

func (w *Server) GetUserByWalletId(walletId string) (*lnbits.User, error) {
	user := &lnbits.User{}
	tx := w.database.Where("wallet_id = ?", walletId).First(user)
	if tx.Error != nil {
		return user, tx.Error
	}
	return user, nil
}

func (w *Server) newRouter() *mux.Router {
	router := mux.NewRouter()
	router.HandleFunc("/", w.receive).Methods(http.MethodPost)
	return router
}

func (w Server) receive(writer http.ResponseWriter, request *http.Request) {
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
	log.Infoln(fmt.Sprintf("[⚡️ WebHook] User %s (%d) received invoice of %d sat.", user.Telegram.Username, user.Telegram.ID, depositEvent.Amount/1000))
	_, err = w.bot.Send(user.Telegram, fmt.Sprintf(i18n.Translate(user.Telegram.LanguageCode, "invoiceReceivedMessage"), depositEvent.Amount/1000))
	if err != nil {
		log.Errorln(err)
	}

	// if this invoice is saved in bunt.db, we load it and display the comment from an LNURL invoice
	tx := &lnurl.Invoice{PaymentHash: depositEvent.PaymentHash}
	err = w.buntdb.Get(tx)
	if err != nil {
		log.Errorln(err)
	} else {
		if len(tx.Comment) > 0 {
			_, err = w.bot.Send(user.Telegram, fmt.Sprintf(`✉️ %s`, str.MarkdownEscape(tx.Comment)))
			if err != nil {
				log.Errorln(err)
			}
		}
	}
	writer.WriteHeader(200)
}
