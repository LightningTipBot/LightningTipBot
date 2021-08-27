package lnurl

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/LightningTipBot/LightningTipBot/internal/lnbits"
	"github.com/fiatjaf/go-lnurl"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
	tb "gopkg.in/tucnak/telebot.v2"
	"gorm.io/gorm"
)

type Server struct {
	httpServer  *http.Server
	bot         *tb.Bot
	c           *lnbits.Client
	database    *gorm.DB
	callbackUrl string
}

const (
	lnurlEndpoint = "/.well-known/lnurlp"
)

func NewServer(lnurlServer string, bot *tb.Bot, client *lnbits.Client, database *gorm.DB) *Server {
	host, port, err := net.SplitHostPort(strings.Split(lnurlServer, "//")[1])
	if err != nil {
		return nil
	}
	srv := &http.Server{
		Addr: fmt.Sprintf("0.0.0.0:%s", port),
		// Good practice: enforce timeouts for servers you create!
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}
	apiServer := &Server{
		c:           client,
		database:    database,
		bot:         bot,
		httpServer:  srv,
		callbackUrl: host,
	}

	apiServer.httpServer.Handler = apiServer.newRouter()
	go apiServer.httpServer.ListenAndServe()
	return apiServer
}

func (w *Server) newRouter() *mux.Router {
	router := mux.NewRouter()
	router.HandleFunc(lnurlEndpoint+"/{username}", w.handleLnUrl).Methods(http.MethodGet)
	//router.HandleFunc("/.well-know/lnurlp/{username}", w.handleLnUrl).Methods(http.MethodGet)
	return router
}

func (w Server) handleLnUrl(writer http.ResponseWriter, request *http.Request) {
	if request.URL.RawQuery == "" {
		w.createInitialLNURLPayResponse(writer, request)
	} else {
		w.createLNURLPayResponse(writer, request)
	}
}

func (w Server) createLNURLPayResponse(writer http.ResponseWriter, request *http.Request) {
	vars := mux.Vars(request)
	user := &lnbits.User{}
	tx := w.database.Where("telegram_username = ?", vars["username"]).First(user)
	if tx.Error != nil || user.Wallet == nil || user.Initialized == false {
		errmsg := fmt.Sprintf("[GetUser] Couldn't fetch user info from database.")
		log.Warnln(errmsg)
		return
	}

	// set wallet lnbits client
	user.Wallet.Client = w.c
	stringAmount := request.FormValue("amount")
	amount, err := strconv.Atoi(stringAmount)
	if err != nil {
		errmsg := fmt.Sprintf("[Atio] Couldn't cast amount to int")
		log.Warnln(errmsg)
		return
	}

	invoice, err := user.Wallet.Invoice(
		lnbits.InvoiceParams{
			Amount: int64(amount / 1000),
			Out:    false,
			Memo:   fmt.Sprintf("Pay to @%s", vars["username"])},
		*user.Wallet)
	var resp lnurl.LNURLPayResponse2
	if err != nil {
		errmsg := fmt.Sprintf("[Invoice] Couldn't create invoice: %s", err.Error())
		log.Warnln(errmsg)
		resp = lnurl.LNURLPayResponse2{
			LNURLResponse: lnurl.LNURLResponse{Status: "ERROR", Reason: "Couldn't create invoice."},
		}
	} else {
		resp = lnurl.LNURLPayResponse2{
			LNURLResponse: lnurl.LNURLResponse{Status: "OK"},
			PR:            invoice.PaymentRequest,
		}
	}

	jsonResponse, err := json.Marshal(resp)
	if err != nil {
		writer.WriteHeader(400)
		return
	}
	writer.Write(jsonResponse)
}

func (w Server) createInitialLNURLPayResponse(writer http.ResponseWriter, request *http.Request) {
	vars := mux.Vars(request)

	callback := fmt.Sprintf("%s%s/%s", w.callbackUrl, lnurlEndpoint, vars["username"])

	resp := lnurl.LNURLPayResponse1{Callback: callback,
		MinSendable:    1000,
		MaxSendable:    100000000,
		CommentAllowed: 422}

	jsonResponse, err := json.Marshal(resp)
	if err != nil {
		writer.WriteHeader(400)
		return
	}
	writer.Write(jsonResponse)
}
