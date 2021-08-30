package lnurl

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
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
	httpServer    *http.Server
	bot           *tb.Bot
	c             *lnbits.Client
	database      *gorm.DB
	callbackUrl   string
	WebhookServer string
}

const (
	lnurlEndpoint = "/.well-known/lnurlp"
	minSendable   = 1000 // mSat
	MaxSendable   = 100000000
)

func NewServer(lnurlserver string, webhookserver string, bot *tb.Bot, client *lnbits.Client, database *gorm.DB) *Server {
	host, port, err := net.SplitHostPort(strings.Split(lnurlserver, "//")[1])
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
		c:             client,
		database:      database,
		bot:           bot,
		httpServer:    srv,
		callbackUrl:   host,
		WebhookServer: webhookserver,
	}

	apiServer.httpServer.Handler = apiServer.newRouter()
	go apiServer.httpServer.ListenAndServe()
	log.Infof("[LNURL] Server started at %s port %s", host, port)
	return apiServer
}

func (w *Server) newRouter() *mux.Router {
	router := mux.NewRouter()
	router.HandleFunc(lnurlEndpoint+"/{username}", w.handleLnUrl).Methods(http.MethodGet)
	router.HandleFunc("/@{username}", w.handleLnUrl).Methods(http.MethodGet)
	//router.HandleFunc("/.well-know/lnurlp/{username}", w.handleLnUrl).Methods(http.MethodGet)
	return router
}

func (w Server) handleLnUrl(writer http.ResponseWriter, request *http.Request) {
	if request.URL.RawQuery == "" {
		var err error
		err = w.serveLNURLpFirst(writer, request)
		if err != nil {
			log.Errorln(err)
			return
		}
	} else {
		w.serveLNURLpSecond(writer, request)
	}
}

// descriptionHash is the SHA256 hash of the metadata
func (w Server) descriptionHash(metadata lnurl.Metadata, writer http.ResponseWriter) string {
	jsonMeta, err := json.Marshal(metadata)
	if err != nil {
		writer.WriteHeader(400)
		return ""
	}
	hash := sha256.Sum256([]byte(string(jsonMeta)))
	hash_string := hex.EncodeToString(hash[:])
	return hash_string
}

// metaData returns the metadata that is sent in the first response
// and is used again in the second response to verify the description hash
func (w Server) metaData(request *http.Request) lnurl.Metadata {
	vars := mux.Vars(request)
	return lnurl.Metadata{{"text/identifier", fmt.Sprintf("%s@ln.tips", vars["username"])}, {"text/plain", fmt.Sprintf("Pay to %s@%s", vars["username"], w.callbackUrl)}}
}

// serveLNURLpFirst serves the first part of the LNURLp protocol with the endpoint
// to call and the metadata that matches the description hash of the second response
func (w Server) serveLNURLpFirst(writer http.ResponseWriter, request *http.Request) error {
	vars := mux.Vars(request)

	callback := fmt.Sprintf("%s%s/%s", w.callbackUrl, lnurlEndpoint, vars["username"])
	callbackURL, err := url.Parse(callback)
	if err != nil {
		log.Errorln("callback is not a valid URL")
		writer.WriteHeader(400)
		return err
	}
	metadata := w.metaData(request)
	jsonMeta, err := json.Marshal(metadata)
	if err != nil {
		writer.WriteHeader(400)
		return err
	}

	resp := lnurl.LNURLPayResponse1{
		LNURLResponse:   lnurl.LNURLResponse{Status: "OK"},
		Tag:             "payRequest",
		Callback:        fmt.Sprintf("https://%s", callback),
		CallbackURL:     callbackURL,
		MinSendable:     minSendable,
		MaxSendable:     MaxSendable,
		CommentAllowed:  512,
		EncodedMetadata: string(jsonMeta),
	}
	jsonResponse, err := json.Marshal(resp)
	if err != nil {
		writer.WriteHeader(400)
		return err
	}
	writer.WriteHeader(200)
	writer.Write(jsonResponse)
	return nil
}

// serveLNURLpSecond serves the second LNURL response with the payment request with the correct description hash
func (w Server) serveLNURLpSecond(writer http.ResponseWriter, request *http.Request) {
	vars := mux.Vars(request)
	user := &lnbits.User{}
	tx := w.database.Where("telegram_username = ?", strings.ToLower(vars["username"])).First(user)
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
		errmsg := fmt.Sprintf("[serveLNURLpSecond] Couldn't cast amount to int")
		log.Warnln(errmsg)
		return
	}

	var resp lnurl.LNURLPayResponse2

	if amount < minSendable || amount > MaxSendable {
		// amount is not ok
		errmsg := fmt.Sprintf("[serveLNURLpSecond] Amount out of bounds")
		log.Warnln(errmsg)
		resp = lnurl.LNURLPayResponse2{
			LNURLResponse: lnurl.LNURLResponse{Status: "ERROR", Reason: fmt.Sprintf("Amount out of bounds (min: %d mSat, max: %d mSat).", minSendable, MaxSendable)},
		}
	} else {
		// amount is ok

		// the same description_hash needs to be built in the second request
		metadata := w.metaData(request)
		hash_string := w.descriptionHash(metadata, writer)
		invoice, err := user.Wallet.Invoice(
			lnbits.InvoiceParams{
				Amount:          int64(amount / 1000),
				Out:             false,
				DescriptionHash: hash_string,
				Webhook:         w.WebhookServer},
			*user.Wallet)
		if err != nil {
			errmsg := fmt.Sprintf("[serveLNURLpSecond] Couldn't create invoice: %s", err.Error())
			log.Warnln(errmsg)
			resp = lnurl.LNURLPayResponse2{
				LNURLResponse: lnurl.LNURLResponse{Status: "ERROR", Reason: "Couldn't create invoice."},
			}
		} else {
			resp = lnurl.LNURLPayResponse2{
				LNURLResponse: lnurl.LNURLResponse{Status: "OK"},
				PR:            invoice.PaymentRequest,
				Routes:        make([][]lnurl.RouteInfo, 0),
				SuccessAction: &lnurl.SuccessAction{Message: "Payment received!", Tag: "message"},
			}
		}
	}

	jsonResponse, err := json.Marshal(resp)
	if err != nil {
		writer.WriteHeader(400)
		return
	}
	writer.Write(jsonResponse)
}
