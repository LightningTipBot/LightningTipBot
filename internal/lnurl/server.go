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
	statusError   = "ERROR"
	statusOk      = "OK"
	payRequestTag = "payRequest"
	lnurlEndpoint = "/.well-known/lnurlp"
	minSendable   = 1000 // mSat
	MaxSendable   = 1000000000
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
	return router
}

func (w Server) handleLnUrl(writer http.ResponseWriter, request *http.Request) {
	var err error
	var response interface{}
	username := mux.Vars(request)["username"]
	if request.URL.RawQuery == "" {
		response, err = w.serveLNURLpFirst(username)
	} else {
		stringAmount := request.FormValue("amount")
		if stringAmount == "" {
			NotFoundHandler(writer, fmt.Errorf("[serveLNURLpSecond] Form value 'amount' is not set"))
			return
		}
		amount, err := strconv.Atoi(stringAmount)
		if err != nil {
			NotFoundHandler(writer, fmt.Errorf("[serveLNURLpSecond] Couldn't cast amount to int %v", err))
			return
		}
		response, err = w.serveLNURLpSecond(username, int64(amount))
	}
	if err != nil {
		log.Errorln(err)
		if response != nil {
			err = writeResponse(writer, response)
			if err != nil {
				NotFoundHandler(writer, err)
			}
		}
	}
	err = writeResponse(writer, response)
	if err != nil {
		NotFoundHandler(writer, err)

	}
}
func NotFoundHandler(writer http.ResponseWriter, err error) {
	log.Errorln(err)
	// return 404 on any error
	http.Error(writer, "404 page not found", http.StatusNotFound)
}

// descriptionHash is the SHA256 hash of the metadata
func (w Server) descriptionHash(metadata lnurl.Metadata) (string, error) {
	jsonMeta, err := json.Marshal(metadata)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256([]byte(string(jsonMeta)))
	hashString := hex.EncodeToString(hash[:])
	return hashString, nil
}

// metaData returns the metadata that is sent in the first response
// and is used again in the second response to verify the description hash
func (w Server) metaData(username string) lnurl.Metadata {
	return lnurl.Metadata{
		{"text/identifier", fmt.Sprintf("%s@ln.tips", username)},
		{"text/plain", fmt.Sprintf("Pay to %s@%s", username, w.callbackUrl)}}
}

// serveLNURLpFirst serves the first part of the LNURLp protocol with the endpoint
// to call and the metadata that matches the description hash of the second response
func (w Server) serveLNURLpFirst(username string) (*lnurl.LNURLPayResponse1, error) {
	log.Infof("[LNURL] Serving endpoint for user %s", username)
	callbackURL, err := url.Parse(fmt.Sprintf("https://%s%s/%s", w.callbackUrl, lnurlEndpoint, username))
	if err != nil {
		return nil, err
	}
	metadata := w.metaData(username)
	jsonMeta, err := json.Marshal(metadata)
	if err != nil {
		return nil, err
	}

	return &lnurl.LNURLPayResponse1{
		LNURLResponse:   lnurl.LNURLResponse{Status: statusOk},
		Tag:             payRequestTag,
		Callback:        callbackURL.String(),
		CallbackURL:     callbackURL, // probably no need to set this here
		MinSendable:     minSendable,
		MaxSendable:     MaxSendable,
		EncodedMetadata: string(jsonMeta),
	}, nil

}

// serveLNURLpSecond serves the second LNURL response with the payment request with the correct description hash
func (w Server) serveLNURLpSecond(username string, amount int64) (*lnurl.LNURLPayResponse2, error) {

	log.Infof("[LNURL] Serving invoice for user %s", username)

	user := &lnbits.User{}
	tx := w.database.Where("telegram_username = ?", strings.ToLower(username)).First(user)
	if tx.Error != nil {
		return nil, fmt.Errorf("[GetUser] Couldn't fetch user info from database: %v", tx.Error)
	}
	if user.Wallet == nil || user.Initialized == false {
		return nil, fmt.Errorf("[serveLNURLpSecond] invalid user data")
	}

	// set wallet lnbits client
	user.Wallet.Client = w.c

	var resp *lnurl.LNURLPayResponse2

	if amount < minSendable || amount > MaxSendable {
		// amount is not ok
		return &lnurl.LNURLPayResponse2{
			LNURLResponse: lnurl.LNURLResponse{
				Status: statusError,
				Reason: fmt.Sprintf("Amount out of bounds (min: %d mSat, max: %d mSat).", minSendable, MaxSendable)},
		}, fmt.Errorf("amount out of bounds")
	}
	// amount is ok
	// the same description_hash needs to be built in the second request
	metadata := w.metaData(username)
	descriptionHash, err := w.descriptionHash(metadata)
	if err != nil {
		return nil, err
	}
	invoice, err := user.Wallet.Invoice(
		lnbits.InvoiceParams{
			Amount:          amount / 1000,
			Out:             false,
			DescriptionHash: descriptionHash,
			Webhook:         w.WebhookServer},
		*user.Wallet)
	if err != nil {
		err = fmt.Errorf("[serveLNURLpSecond] Couldn't create invoice: %v", err)
		resp = &lnurl.LNURLPayResponse2{
			LNURLResponse: lnurl.LNURLResponse{
				Status: statusError,
				Reason: "Couldn't create invoice."},
		}
		return resp, err
	}
	return &lnurl.LNURLPayResponse2{
		LNURLResponse: lnurl.LNURLResponse{Status: statusOk},
		PR:            invoice.PaymentRequest,
		Routes:        make([][]lnurl.RouteInfo, 0),
		SuccessAction: &lnurl.SuccessAction{Message: "Payment received!", Tag: "message"},
	}, nil

}

func writeResponse(writer http.ResponseWriter, response interface{}) error {
	jsonResponse, err := json.Marshal(response)
	if err != nil {
		return err
	}
	_, err = writer.Write(jsonResponse)
	return err
}
