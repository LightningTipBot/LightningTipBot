package lnurl

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/eko/gocache/store"
	tb "gopkg.in/lightningtipbot/telebot.v2"

	"github.com/LightningTipBot/LightningTipBot/internal"
	"github.com/LightningTipBot/LightningTipBot/internal/api"
	"github.com/LightningTipBot/LightningTipBot/internal/storage"
	"gorm.io/gorm"

	"github.com/LightningTipBot/LightningTipBot/internal/lnbits"
	"github.com/LightningTipBot/LightningTipBot/internal/runtime"
	"github.com/LightningTipBot/LightningTipBot/internal/telegram"
	"github.com/fiatjaf/go-lnurl"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
)

const (
	PayRequestTag  = "payRequest"
	Endpoint       = ".well-known/lnurlp"
	MinSendable    = 1000 // mSat
	MaxSendable    = 1_000_000_000
	CommentAllowed = 500
)

type Invoice struct {
	*telegram.Invoice
	Comment   string       `json:"comment"`
	User      *lnbits.User `json:"user"`
	CreatedAt time.Time    `json:"created_at"`
	Paid      bool         `json:"paid"`
	PaidAt    time.Time    `json:"paid_at"`
}
type Lnurl struct {
	telegram         *tb.Bot
	c                *lnbits.Client
	database         *gorm.DB
	callbackHostname *url.URL
	buntdb           *storage.DB
	WebhookServer    string
	cache            telegram.Cache
}

func New(bot *telegram.TipBot) Lnurl {
	return Lnurl{
		c:                bot.Client,
		database:         bot.Database,
		callbackHostname: internal.Configuration.Bot.LNURLHostUrl,
		WebhookServer:    internal.Configuration.Lnbits.WebhookServer,
		buntdb:           bot.Bunt,
		telegram:         bot.Telegram,
		cache:            bot.Cache,
	}
}
func (lnurlInvoice Invoice) Key() string {
	return fmt.Sprintf("lnurl-p:%s", lnurlInvoice.PaymentHash)
}

func (w Lnurl) Handle(writer http.ResponseWriter, request *http.Request) {
	var err error
	var response interface{}
	username := mux.Vars(request)["username"]
	if request.URL.RawQuery == "" {
		response, err = w.serveLNURLpFirst(username)
	} else {
		stringAmount := request.FormValue("amount")
		if stringAmount == "" {
			api.NotFoundHandler(writer, fmt.Errorf("[handleLnUrl] Form value 'amount' is not set"))
			return
		}
		amount, parseError := strconv.Atoi(stringAmount)
		if parseError != nil {
			api.NotFoundHandler(writer, fmt.Errorf("[handleLnUrl] Couldn't cast amount to int %v", parseError))
			return
		}
		comment := request.FormValue("comment")
		if len(comment) > CommentAllowed {
			api.NotFoundHandler(writer, fmt.Errorf("[handleLnUrl] Comment is too long"))
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
			err = api.WriteResponse(writer, response)
			if err != nil {
				api.NotFoundHandler(writer, err)
			}
		}
		return
	}
	// no error from first or second handler
	err = api.WriteResponse(writer, response)
	if err != nil {
		api.NotFoundHandler(writer, err)
	}
}
func (w Lnurl) getMetaDataCached(username string) lnurl.Metadata {
	key := fmt.Sprintf("lnurl_metadata_%s", username)

	// load metadata from cache
	if m, err := w.cache.Get(key); err == nil {
		return m.(lnurl.Metadata)
	}

	// otherwise, create new metadata
	metadata := w.metaData(username)

	// load the user profile picture
	if internal.Configuration.Bot.LNURLSendImage {
		// get the user from the database
		user, tx := findUser(w.database, username)
		if tx.Error == nil && user.Telegram != nil {
			addImageToMetaData(w.telegram, &metadata, username, user.Telegram)
		}
	}

	// save into cache
	runtime.IgnoreError(w.cache.Set(key, metadata, &store.Options{Expiration: 12 * time.Hour}))
	return metadata
}

// serveLNURLpFirst serves the first part of the LNURLp protocol with the endpoint
// to call and the metadata that matches the description hash of the second response
func (w Lnurl) serveLNURLpFirst(username string) (*lnurl.LNURLPayParams, error) {
	log.Infof("[LNURL] Serving endpoint for user %s", username)
	callbackURL, err := url.Parse(fmt.Sprintf("%s/%s/%s", w.callbackHostname.String(), Endpoint, username))
	if err != nil {
		return nil, err
	}

	// produce the metadata including the image
	metadata := w.getMetaDataCached(username)

	return &lnurl.LNURLPayParams{
		LNURLResponse:   lnurl.LNURLResponse{Status: api.StatusOk},
		Tag:             PayRequestTag,
		Callback:        callbackURL.String(),
		MinSendable:     MinSendable,
		MaxSendable:     MaxSendable,
		EncodedMetadata: metadata.Encode(),
		CommentAllowed:  CommentAllowed,
	}, nil
}

// serveLNURLpSecond serves the second LNURL response with the payment request with the correct description hash
func (w Lnurl) serveLNURLpSecond(username string, amount_msat int64, comment string) (*lnurl.LNURLPayValues, error) {
	log.Infof("[LNURL] Serving invoice for user %s", username)
	if amount_msat < MinSendable || amount_msat > MaxSendable {
		// amount is not ok
		return &lnurl.LNURLPayValues{
			LNURLResponse: lnurl.LNURLResponse{
				Status: api.StatusError,
				Reason: fmt.Sprintf("Amount out of bounds (min: %d mSat, max: %d mSat).", MinSendable, MinSendable)},
		}, fmt.Errorf("amount out of bounds")
	}
	// check comment length
	if len(comment) > CommentAllowed {
		return &lnurl.LNURLPayValues{
			LNURLResponse: lnurl.LNURLResponse{
				Status: api.StatusError,
				Reason: fmt.Sprintf("Comment too long (max: %d characters).", CommentAllowed)},
		}, fmt.Errorf("comment too long")
	}
	user, tx := findUser(w.database, username)
	if tx.Error != nil {
		return &lnurl.LNURLPayValues{
			LNURLResponse: lnurl.LNURLResponse{
				Status: api.StatusError,
				Reason: fmt.Sprintf("Invalid user.")},
		}, fmt.Errorf("[GetUser] Couldn't fetch user info from database: %v", tx.Error)
	}
	if user.Wallet == nil {
		return &lnurl.LNURLPayValues{
			LNURLResponse: lnurl.LNURLResponse{
				Status: api.StatusError,
				Reason: fmt.Sprintf("Invalid user.")},
		}, fmt.Errorf("[serveLNURLpSecond] user %s not found", username)
	}
	// user is ok now create invoice
	// set wallet lnbits client

	var resp *lnurl.LNURLPayValues

	// the same description_hash needs to be built in the second request
	metadata := w.getMetaDataCached(username)

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
				Status: api.StatusError,
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
		Invoice{
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
		LNURLResponse: lnurl.LNURLResponse{Status: api.StatusOk},
		PR:            invoice.PaymentRequest,
		Routes:        make([]struct{}, 0),
		SuccessAction: &lnurl.SuccessAction{Message: "Payment received!", Tag: "message"},
	}, nil

}

// descriptionHash is the SHA256 hash of the metadata
func (w Lnurl) descriptionHash(metadata lnurl.Metadata) (string, error) {
	hash := sha256.Sum256([]byte(metadata.Encode()))
	hashString := hex.EncodeToString(hash[:])
	return hashString, nil
}

// metaData returns the metadata that is sent in the first response
// and is used again in the second response to verify the description hash
func (w Lnurl) metaData(username string) lnurl.Metadata {
	// this is a bit stupid but if the address is a UUID starting with 1x...
	// we actually want to find the users username so it looks nicer in the
	// metadata description
	if strings.HasPrefix(username, "1x") {
		user, _ := findUser(w.database, username)
		if user.Telegram.Username != "" {
			username = user.Telegram.Username
		}
	}

	return lnurl.Metadata{
		Description:      fmt.Sprintf("Pay to %s@%s", username, w.callbackHostname.Hostname()),
		LightningAddress: fmt.Sprintf("%s@%s", username, w.callbackHostname.Hostname()),
	}
}

// addImageMetaData add images an image to the LNURL metadata
func addImageToMetaData(tb *tb.Bot, metadata *lnurl.Metadata, username string, user *tb.User) {
	metadata.Image.Ext = "jpeg"

	// if the username is anonymous, add the bot's avatar
	if isAnonUsername(username) {
		metadata.Image.Bytes = telegram.BotProfilePicture
		return
	}

	// if the user has a profile picture, add it
	picture, err := telegram.DownloadProfilePicture(tb, user)
	if err != nil {
		log.Errorf("[LNURL] Couldn't download user %s's profile picture: %v", username, err)
		return
	}
	metadata.Image.Bytes = picture
}

func isAnonUsername(username string) bool {
	if _, err := strconv.ParseInt(username, 10, 64); err == nil {
		return true
	} else {
		return strings.HasPrefix(username, "0x")
	}
}

func findUser(database *gorm.DB, username string) (*lnbits.User, *gorm.DB) {
	// now check for the user
	user := &lnbits.User{}
	// check if "username" is actually the user ID
	tx := database
	if _, err := strconv.ParseInt(username, 10, 64); err == nil {
		// asume it's anon_id
		tx = database.Where("anon_id = ?", username).First(user)
	} else if strings.HasPrefix(username, "0x") {
		// asume it's anon_id_sha256
		tx = database.Where("anon_id_sha256 = ?", username).First(user)
	} else if strings.HasPrefix(username, "1x") {
		// asume it's uuid
		tx = database.Where("uuid = ?", username).First(user)
	} else {
		// assume it's a string @username
		tx = database.Where("telegram_username = ? COLLATE NOCASE", username).First(user)
	}
	return user, tx
}
