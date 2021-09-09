package lnurl

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/LightningTipBot/LightningTipBot/internal/lnbits"
	"github.com/fiatjaf/go-lnurl"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
)

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
		amount, parseError := strconv.Atoi(stringAmount)
		if parseError != nil {
			NotFoundHandler(writer, fmt.Errorf("[serveLNURLpSecond] Couldn't cast amount to int %v", parseError))
			return
		}
		response, err = w.serveLNURLpSecond(username, int64(amount))
	}
	// check if error was returned from first or second handlers
	if err != nil {
		// log the error
		log.Errorf("[LNURL] %v", err)
		if response != nil {
			// there is a valid error response
			err = writeResponse(writer, response)
			if err != nil {
				NotFoundHandler(writer, err)
			}
		}
		return
	}
	// no error from first or second handler
	err = writeResponse(writer, response)
	if err != nil {
		NotFoundHandler(writer, err)
	}
}

// serveLNURLpFirst serves the first part of the LNURLp protocol with the endpoint
// to call and the metadata that matches the description hash of the second response
func (w Server) serveLNURLpFirst(username string) (*lnurl.LNURLPayResponse1, error) {
	log.Infof("[LNURL] Serving endpoint for user %s", username)
	callbackURL, err := url.Parse(fmt.Sprintf("%s/%s/%s", w.callbackHostname.String(), lnurlEndpoint, username))
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
	if amount < minSendable || amount > MaxSendable {
		// amount is not ok
		return &lnurl.LNURLPayResponse2{
			LNURLResponse: lnurl.LNURLResponse{
				Status: statusError,
				Reason: fmt.Sprintf("Amount out of bounds (min: %d mSat, max: %d mSat).", minSendable, MaxSendable)},
		}, fmt.Errorf("amount out of bounds")
	}
	// amount is ok now check for the user
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
		{"text/plain", fmt.Sprintf("Pay to %s@%s", username, w.callbackHostname)}}
}
