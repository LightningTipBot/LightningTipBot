package lnurl

import (
	"encoding/json"
	"net/http"
	"net/url"
	"time"

	"github.com/LightningTipBot/LightningTipBot/internal/lnbits"
	"github.com/LightningTipBot/LightningTipBot/internal/storage"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
	tb "gopkg.in/tucnak/telebot.v2"
	"gorm.io/gorm"
)

type Server struct {
	httpServer       *http.Server
	bot              *tb.Bot
	c                *lnbits.Client
	database         *gorm.DB
	callbackHostname *url.URL
	buntdb           *storage.DB
	WebhookServer    string
}

const (
	statusError    = "ERROR"
	statusOk       = "OK"
	payRequestTag  = "payRequest"
	lnurlEndpoint  = ".well-known/lnurlp"
	minSendable    = 1000 // mSat
	MaxSendable    = 1_000_000_000
	CommentAllowed = 256
)

func NewServer(addr, callbackHostname *url.URL, webhookServer string, bot *tb.Bot, client *lnbits.Client, database *gorm.DB, buntdb *storage.DB) *Server {
	srv := &http.Server{
		Addr: addr.Host,
		// Good practice: enforce timeouts for servers you create!
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}
	apiServer := &Server{
		c:                client,
		database:         database,
		bot:              bot,
		httpServer:       srv,
		callbackHostname: callbackHostname,
		WebhookServer:    webhookServer,
		buntdb:           buntdb,
	}

	apiServer.httpServer.Handler = apiServer.newRouter()
	go apiServer.httpServer.ListenAndServe()
	log.Infof("[LNURL] Server started at %s", addr.Host)
	return apiServer
}

func (w *Server) newRouter() *mux.Router {
	router := mux.NewRouter()
	router.HandleFunc("/.well-known/lnurlp/{username}", w.handleLnUrl).Methods(http.MethodGet)
	router.HandleFunc("/@{username}", w.handleLnUrl).Methods(http.MethodGet)
	return router
}

func NotFoundHandler(writer http.ResponseWriter, err error) {
	log.Errorln(err)
	// return 404 on any error
	http.Error(writer, "404 page not found", http.StatusNotFound)
}

func writeResponse(writer http.ResponseWriter, response interface{}) error {
	jsonResponse, err := json.Marshal(response)
	if err != nil {
		return err
	}
	_, err = writer.Write(jsonResponse)
	return err
}
