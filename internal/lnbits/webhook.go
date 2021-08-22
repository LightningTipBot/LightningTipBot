package lnbits

import (
	"encoding/json"
	"fmt"
	"net"
	"strings"

	log "github.com/sirupsen/logrus"

	"gorm.io/gorm"

	"github.com/gorilla/mux"
	tb "gopkg.in/tucnak/telebot.v2"

	"net/http"
	"time"
)

const (
	invoiceReceivedMessage = "⚡️ You've received %d sat."
)

type WebhookServer struct {
	httpServer *http.Server
	bot        *tb.Bot
	c          *Client
	database   *gorm.DB
}

func NewWebhook(webhookServer string, bot *tb.Bot, client *Client, database *gorm.DB) *WebhookServer {
	if !strings.Contains(webhookServer, "://") {
		log.Fatal("invalid webhook server configuration. please add a scheme.")
	}
	_, port, err := net.SplitHostPort(strings.Split(webhookServer, "//")[1])
	if err != nil {
		return nil
	}
	srv := &http.Server{
		Addr: fmt.Sprintf("0.0.0.0:%s", port),
		// Good practice: enforce timeouts for servers you create!
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}
	apiServer := &WebhookServer{
		c:          client,
		database:   database,
		bot:        bot,
		httpServer: srv,
	}
	apiServer.httpServer.Handler = apiServer.newRouter()
	go apiServer.httpServer.ListenAndServe()
	return apiServer
}

func (w *WebhookServer) GetUserByWalletId(walletId string) (*User, error) {
	user := &User{}
	tx := w.database.Where("wallet_id = ?", walletId).First(user)
	if tx.Error != nil {
		return user, tx.Error
	}
	user.Wallet.Client = w.c
	return user, nil
}

func (w *WebhookServer) newRouter() *mux.Router {
	router := mux.NewRouter()
	router.HandleFunc("/", w.receive).Methods(http.MethodPost)
	return router
}

func (w WebhookServer) receive(writer http.ResponseWriter, request *http.Request) {
	depositEvent := Webhook{}
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
	_, err = w.bot.Send(user.Telegram, fmt.Sprintf(invoiceReceivedMessage, depositEvent.Amount/1000))
	if err != nil {
		log.Errorln(err)
	}
	writer.WriteHeader(200)
}
