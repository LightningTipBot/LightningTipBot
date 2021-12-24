package lnurl

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/LightningTipBot/LightningTipBot/internal/lnbits"
	"github.com/LightningTipBot/LightningTipBot/internal/runtime"
	"github.com/LightningTipBot/LightningTipBot/internal/telegram"
	"github.com/fiatjaf/go-lnurl"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
)

type LNURLInvoice struct {
	*telegram.Invoice
	Comment   string       `json:"comment"`
	User      *lnbits.User `json:"user"`
	CreatedAt time.Time    `json:"created_at"`
	Paid      bool         `json:"paid"`
	PaidAt    time.Time    `json:"paid_at"`
}

func (lnurlInvoice LNURLInvoice) Key() string {
	return fmt.Sprintf("lnurl-p:%s", lnurlInvoice.PaymentHash)
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
			NotFoundHandler(writer, fmt.Errorf("[handleLnUrl] Form value 'amount' is not set"))
			return
		}
		amount, parseError := strconv.Atoi(stringAmount)
		if parseError != nil {
			NotFoundHandler(writer, fmt.Errorf("[handleLnUrl] Couldn't cast amount to int %v", parseError))
			return
		}
		comment := request.FormValue("comment")
		if len(comment) > CommentAllowed {
			NotFoundHandler(writer, fmt.Errorf("[handleLnUrl] Comment is too long"))
			return
		}
		response, err = w.serveLNURLpSecond(username, int64(amount), comment)
	}
	// check if error was returned from first or second handlers
	if err != nil {
		// log the error
		log.Errorf("[LNURL] %v", err.Error())
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
func (w Server) serveLNURLpFirst(username string) (*lnurl.LNURLPayParams, error) {
	log.Infof("[LNURL] Serving endpoint for user %s", username)
	callbackURL, err := url.Parse(fmt.Sprintf("%s/%s/%s", w.callbackHostname.String(), lnurlEndpoint, username))
	if err != nil {
		return nil, err
	}
	metadata := w.metaData(username)

	return &lnurl.LNURLPayParams{
		LNURLResponse:   lnurl.LNURLResponse{Status: statusOk},
		Tag:             payRequestTag,
		Callback:        callbackURL.String(),
		MinSendable:     minSendable,
		MaxSendable:     MaxSendable,
		EncodedMetadata: metadata.Encode(),
		CommentAllowed:  CommentAllowed,
	}, nil

}

// serveLNURLpSecond serves the second LNURL response with the payment request with the correct description hash
func (w Server) serveLNURLpSecond(username string, amount_msat int64, comment string) (*lnurl.LNURLPayValues, error) {
	log.Infof("[LNURL] Serving invoice for user %s", username)
	if amount_msat < minSendable || amount_msat > MaxSendable {
		// amount is not ok
		return &lnurl.LNURLPayValues{
			LNURLResponse: lnurl.LNURLResponse{
				Status: statusError,
				Reason: fmt.Sprintf("Amount out of bounds (min: %d mSat, max: %d mSat).", minSendable, MaxSendable)},
		}, fmt.Errorf("amount out of bounds")
	}
	// check comment length
	if len(comment) > CommentAllowed {
		return &lnurl.LNURLPayValues{
			LNURLResponse: lnurl.LNURLResponse{
				Status: statusError,
				Reason: fmt.Sprintf("Comment too long (max: %d characters).", CommentAllowed)},
		}, fmt.Errorf("comment too long")
	}

	// now check for the user
	user := &lnbits.User{}
	// check if "username" is actually the user ID
	tx := w.database
	if _, err := strconv.ParseInt(username, 10, 64); err == nil {
		// asume it's a user ID
		tx = w.database.Where("anon_id = ?", username).First(user)
	} else {
		// assume it's a string @username
		tx = w.database.Where("telegram_username = ? COLLATE NOCASE", username).First(user)
	}

	if tx.Error != nil {
		return &lnurl.LNURLPayValues{
			LNURLResponse: lnurl.LNURLResponse{
				Status: statusError,
				Reason: fmt.Sprintf("Invalid user.")},
		}, fmt.Errorf("[GetUser] Couldn't fetch user info from database: %v", tx.Error)
	}
	if user.Wallet == nil {
		return &lnurl.LNURLPayValues{
			LNURLResponse: lnurl.LNURLResponse{
				Status: statusError,
				Reason: fmt.Sprintf("Invalid user.")},
		}, fmt.Errorf("[serveLNURLpSecond] user %s not found", username)
	}
	// user is ok now create invoice
	// set wallet lnbits client

	var resp *lnurl.LNURLPayValues

	// the same description_hash needs to be built in the second request
	metadata := w.metaData(username)
	descriptionHash, err := w.descriptionHash(metadata)
	if err != nil {
		return nil, err
	}
	invoice, err := user.Wallet.Invoice(
		lnbits.InvoiceParams{
			Amount:          amount_msat / 1000,
			Out:             false,
			DescriptionHash: descriptionHash,
			Webhook:         w.WebhookServer},
		w.c)
	if err != nil {
		err = fmt.Errorf("[serveLNURLpSecond] Couldn't create invoice: %v", err.Error())
		resp = &lnurl.LNURLPayValues{
			LNURLResponse: lnurl.LNURLResponse{
				Status: statusError,
				Reason: "Couldn't create invoice."},
		}
		return resp, err
	}
	invoiceStruct := &telegram.Invoice{
		PaymentRequest: invoice.PaymentRequest,
		PaymentHash:    invoice.PaymentHash,
		Amount:         amount_msat / 1000,
	}
	// save lnurl invoice struct for later use (will hold the comment or other metdata for a notification when paid)
	runtime.IgnoreError(w.buntdb.Set(
		LNURLInvoice{
			Invoice:   invoiceStruct,
			User:      user,
			Comment:   comment,
			CreatedAt: time.Now(),
		}))
	// save the invoice Event that will be loaded when the invoice is paid and trigger the comment display callback
	runtime.IgnoreError(w.buntdb.Set(
		telegram.InvoiceEvent{
			Invoice:  invoiceStruct,
			User:     user,
			Callback: telegram.InvoiceCallbackLNURLPayReceive,
		}))

	return &lnurl.LNURLPayValues{
		LNURLResponse: lnurl.LNURLResponse{Status: statusOk},
		PR:            invoice.PaymentRequest,
		Routes:        make([]struct{}, 0),
		SuccessAction: &lnurl.SuccessAction{Message: "Payment received!", Tag: "message"},
	}, nil

}

// descriptionHash is the SHA256 hash of the metadata
func (w Server) descriptionHash(metadata lnurl.Metadata) (string, error) {
	hash := sha256.Sum256([]byte(metadata.Encode()))
	hashString := hex.EncodeToString(hash[:])
	return hashString, nil
}

// metaData returns the metadata that is sent in the first response
// and is used again in the second response to verify the description hash
func (w Server) metaData(username string) lnurl.Metadata {
	return lnurl.Metadata{
		Description:      fmt.Sprintf("Pay to %s@%s", username, w.callbackHostname.Hostname()),
		LightningAddress: fmt.Sprintf("%s@%s", username, w.callbackHostname.Hostname()),
	}
}
