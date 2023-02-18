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
	"time"

	"github.com/eko/gocache/store"
	tb "gopkg.in/lightningtipbot/telebot.v3"

	"github.com/LightningTipBot/LightningTipBot/internal"
	"github.com/LightningTipBot/LightningTipBot/internal/api"
	"github.com/LightningTipBot/LightningTipBot/internal/storage"
	"gorm.io/gorm"

	db "github.com/LightningTipBot/LightningTipBot/internal/database"
	"github.com/LightningTipBot/LightningTipBot/internal/lnbits"
	"github.com/LightningTipBot/LightningTipBot/internal/runtime"
	"github.com/LightningTipBot/LightningTipBot/internal/telegram"
	"github.com/fiatjaf/go-lnurl"
	"github.com/gorilla/mux"
	"github.com/nbd-wtf/go-nostr"
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
	Comment            string       `json:"comment"`
	User               *lnbits.User `json:"user"`
	CreatedAt          time.Time    `json:"created_at"`
	Paid               bool         `json:"paid"`
	PaidAt             time.Time    `json:"paid_at"`
	From               string       `json:"from"`
	Nip57Receipt       nostr.Event  `json:"nip57_receipt"`
	Nip57ReceiptRelays []string     `json:"nip57_receipt_relays"`
}
type Lnurl struct {
	telegram         *tb.Bot
	c                *lnbits.Client
	database         *gorm.DB
	callbackHostname *url.URL
	buntdb           *storage.DB
	WebhookServer    string
	cache            telegram.Cache
	bot              *telegram.TipBot
}

func New(bot *telegram.TipBot) Lnurl {
	return Lnurl{
		c:                bot.Client,
		database:         bot.DB.Users,
		callbackHostname: internal.Configuration.Bot.LNURLHostUrl,
		WebhookServer:    internal.Configuration.Lnbits.WebhookServer,
		buntdb:           bot.Bunt,
		telegram:         bot.Telegram,
		cache:            bot.Cache,
		bot:              bot,
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

		var amount int64
		if amount, err = strconv.ParseInt(stringAmount, 10, 64); err != nil {
			// if the value wasn't a clean msat denomination, parse it
			amount, err = telegram.GetAmount(stringAmount)
			if err != nil {
				api.NotFoundHandler(writer, fmt.Errorf("[handleLnUrl] Couldn't cast amount to int: %v", err))
				return
			}
			// GetAmount returns sat, we need msat
			amount *= 1000
		}

		comment := request.FormValue("comment")
		if len(comment) > CommentAllowed {
			api.NotFoundHandler(writer, fmt.Errorf("[handleLnUrl] Comment is too long"))
			return
		}

		// payer data
		payerdata := request.FormValue("payerdata")
		var payerData lnurl.PayerDataValues
		if len(payerdata) > 0 {
			err = json.Unmarshal([]byte(payerdata), &payerData)
			if err != nil {
				// api.NotFoundHandler(writer, fmt.Errorf("[handleLnUrl] Couldn't parse payerdata: %v", err))
				log.Errorf("[handleLnUrl] Couldn't parse payerdata: %v", err)
				// log.Errorf("[handleLnUrl] payerdata: %v", payerdata)
			}

		}

		// nostr NIP-57
		// the "nostr" query param has a zap request which is a nostr event
		// that specifies which nostr note has been zapped.
		// here we check wheter its present, the event signature is valid
		// and whether the event has the necessary tags that we need (p and relays are necessary, e is optional)
		zapEventQuery := request.FormValue("nostr")
		var zapEvent nostr.Event
		if len(zapEventQuery) > 0 {
			err = json.Unmarshal([]byte(zapEventQuery), &zapEvent)
			if err != nil {
				log.Errorf("[handleLnUrl] Couldn't parse nostr event: %v", err)
			} else {
				valid, err := zapEvent.CheckSignature()
				if !valid || err != nil {
					log.Errorf("[handleLnUrl] Nostr NIP-57 zap event signature invalid: %v", err)
					return
				}
				if len(zapEvent.Tags) == 0 || zapEvent.Tags.GetFirst([]string{"p"}) == nil ||
					zapEvent.Tags.GetFirst([]string{"relays"}) == nil {
					// zapEvent.Tags.GetFirst([]string{"e"}) == nil {
					log.Errorf("[handleLnUrl] Nostr NIP-57 zap event validation error")
					return
				}

			}
		}

		response, err = w.serveLNURLpSecond(username, int64(amount), comment, payerData, zapEvent)
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
		user, tx := db.FindUser(w.database, username)
		if tx.Error == nil && user.Telegram != nil {
			addImageToMetaData(w.telegram, &metadata, username, user.Telegram)
		}
	}

	// save into cache
	runtime.IgnoreError(w.cache.Set(key, metadata, &store.Options{Expiration: 30 * time.Minute}))
	return metadata
}

// we have our custom LNURLPayParams response object here because we want to
// add nostr nip57 fields to it
type LNURLPayParamsCustom struct {
	lnurl.LNURLResponse
	Callback        string               `json:"callback"`
	Tag             string               `json:"tag"`
	MaxSendable     int64                `json:"maxSendable"`
	MinSendable     int64                `json:"minSendable"`
	EncodedMetadata string               `json:"metadata"`
	CommentAllowed  int64                `json:"commentAllowed"`
	PayerData       *lnurl.PayerDataSpec `json:"payerData,omitempty"`
	AllowNostr      bool                 `json:"allowsNostr,omitempty"`
	NostrPubKey     string               `json:"nostrPubkey,omitempty"`
	Metadata        lnurl.Metadata       `json:"-"`
}

// serveLNURLpFirst serves the first part of the LNURLp protocol with the endpoint
// to call and the metadata that matches the description hash of the second response
func (w Lnurl) serveLNURLpFirst(username string) (*LNURLPayParamsCustom, error) {
	log.Infof("[LNURL] Serving endpoint for user %s", username)
	callbackURL, err := url.Parse(fmt.Sprintf("%s/%s/%s", w.callbackHostname.String(), Endpoint, username))
	if err != nil {
		return nil, err
	}

	// produce the metadata including the image
	metadata := w.getMetaDataCached(username)

	// check if the user has added a nostr key for nip57
	var allowNostr bool = false
	var nostrPubkey string = ""
	user, tx := db.FindUser(w.database, username)
	// if the bot has a nostr private key
	if len(internal.Configuration.Nostr.PrivateKey) > 0 &&
		tx.Error == nil && user.Telegram != nil {
		user, err = db.FindUserSettings(user, w.bot.DB.Users.Preload("Settings"))
		if err != nil {
			return &LNURLPayParamsCustom{}, err
		}
		// if the user has a nostr public key
		if user.Settings.Nostr.PubKey != "" {
			allowNostr = true
			pk := internal.Configuration.Nostr.PrivateKey
			pub, _ := nostr.GetPublicKey(pk)
			nostrPubkey = pub
		}
	} else {
		log.Errorf("[serveLNURLpFirst] user not found")
	}

	return &LNURLPayParamsCustom{
		LNURLResponse:   lnurl.LNURLResponse{Status: api.StatusOk},
		Tag:             PayRequestTag,
		Callback:        callbackURL.String(),
		MinSendable:     MinSendable,
		MaxSendable:     MaxSendable,
		EncodedMetadata: metadata.Encode(),
		CommentAllowed:  CommentAllowed,
		PayerData: &lnurl.PayerDataSpec{
			FreeName:         &lnurl.PayerDataItemSpec{},
			LightningAddress: &lnurl.PayerDataItemSpec{},
			Email:            &lnurl.PayerDataItemSpec{},
		},
		AllowNostr:  allowNostr,
		NostrPubKey: nostrPubkey,
	}, nil
}

// serveLNURLpSecond serves the second LNURL response with the payment request with the correct description hash
func (w Lnurl) serveLNURLpSecond(username string, amount_msat int64, comment string, payerData lnurl.PayerDataValues, zapEvent nostr.Event) (*lnurl.LNURLPayValues, error) {
	log.Infof("[LNURL] Serving invoice for user %s", username)
	if amount_msat < MinSendable || amount_msat > MaxSendable {
		// amount is not ok
		return &lnurl.LNURLPayValues{
			LNURLResponse: lnurl.LNURLResponse{
				Status: api.StatusError,
				Reason: fmt.Sprintf("Amount out of bounds (min: %d sat, max: %d sat).", MinSendable/1000, MaxSendable/1000)},
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
	user, tx := db.FindUser(w.database, username)
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
	var descriptionHash string

	// NIP57 ZAPs
	// TODO: refactor all this into nip57.go
	var nip57Receipt nostr.Event
	var zapEventSerializedStr string
	var nip57ReceiptRelays []string
	// for nip57 use the nostr event as the descriptionHash
	if zapEvent.Sig != "" {
		log.Infof("[LNURL] nostr zap for user %s", username)
		// we calculate the descriptionHash here, create an invoice with it
		// and store the invoice in the zap receipt later down the line
		zapEventSerialized, err := json.Marshal(zapEvent)
		zapEventSerializedStr = fmt.Sprintf("%s", zapEventSerialized)
		if err != nil {
			log.Println(err)
			return &lnurl.LNURLPayValues{
				LNURLResponse: lnurl.LNURLResponse{
					Status: api.StatusError,
					Reason: "Couldn't serialize zap event."},
			}, err
		}
		// we extract the relays from the zap request
		nip57ReceiptRelaysTags := zapEvent.Tags.GetFirst([]string{"relays"})
		if len(fmt.Sprintf("%s", nip57ReceiptRelaysTags)) > 0 {
			nip57ReceiptRelays = strings.Split(fmt.Sprintf("%s", nip57ReceiptRelaysTags), " ")
			// this tirty method returns slice [ "[relays", "wss...", "wss...", "wss...]" ] – we need to clean it up
			if len(nip57ReceiptRelays) > 1 {
				// remove the first entry
				nip57ReceiptRelays = nip57ReceiptRelays[1:]
				// clean up the last entry
				len_last_entry := len(nip57ReceiptRelays[len(nip57ReceiptRelays)-1])
				nip57ReceiptRelays[len(nip57ReceiptRelays)-1] = nip57ReceiptRelays[len(nip57ReceiptRelays)-1][:len_last_entry-1]
			}
			// now the relay list is clean!
		}
		// calculate description hash from the serialized nostr event
		descriptionHash = w.Nip57DescriptionHash(zapEventSerializedStr)
	} else {
		// calculate normal LNURL descriptionhash
		// the same description_hash needs to be built in the second request
		metadata := w.getMetaDataCached(username)

		var payerDataByte []byte
		var err error
		if payerData.Email != "" || payerData.LightningAddress != "" || payerData.FreeName != "" {
			payerDataByte, err = json.Marshal(payerData)
			if err != nil {
				return nil, err
			}
		} else {
			payerDataByte = []byte("")
		}

		descriptionHash, err = w.DescriptionHash(metadata, string(payerDataByte))
		if err != nil {
			return nil, err
		}
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

	// nip57 - we need to store the newly created invoice in the zap receipt
	if zapEvent.Sig != "" {
		pk := internal.Configuration.Nostr.PrivateKey
		pub, _ := nostr.GetPublicKey(pk)
		nip57Receipt = nostr.Event{
			PubKey:    pub,
			CreatedAt: time.Now(),
			Kind:      9735,
			Tags: nostr.Tags{
				*zapEvent.Tags.GetFirst([]string{"p"}),
				[]string{"bolt11", invoice.PaymentRequest},
				[]string{"description", zapEventSerializedStr},
			},
		}
		if zapEvent.Tags.GetFirst([]string{"e"}) != nil {
			nip57Receipt.Tags = nip57Receipt.Tags.AppendUnique(*zapEvent.Tags.GetFirst([]string{"e"}))
		}
		nip57Receipt.Sign(pk)
	}

	// save lnurl invoice struct for later use (will hold the comment or other metadata for a notification when paid)
	// also holds Nip57 Zap receipt to send to nostr when invoice is paid
	runtime.IgnoreError(w.buntdb.Set(
		Invoice{
			Invoice:            invoiceStruct,
			User:               user,
			Comment:            comment,
			CreatedAt:          time.Now(),
			From:               extractSenderFromPayerdata(payerData),
			Nip57Receipt:       nip57Receipt,
			Nip57ReceiptRelays: nip57ReceiptRelays,
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

// DescriptionHash is the SHA256 hash of the metadata
func (w Lnurl) DescriptionHash(metadata lnurl.Metadata, payerData string) (string, error) {
	var hashString string
	var hash [32]byte
	if len(payerData) == 0 {
		hash = sha256.Sum256([]byte(metadata.Encode()))
		hashString = hex.EncodeToString(hash[:])
	} else {
		hash = sha256.Sum256([]byte(metadata.Encode() + payerData))
		hashString = hex.EncodeToString(hash[:])
	}
	return hashString, nil
}

// metaData returns the metadata that is sent in the first response
// and is used again in the second response to verify the description hash
func (w Lnurl) metaData(username string) lnurl.Metadata {
	// this is a bit stupid but if the address is a UUID starting with 1x...
	// we actually want to find the users username so it looks nicer in the
	// metadata description
	if strings.HasPrefix(username, "1x") {
		user, _ := db.FindUser(w.database, username)
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

	// if the username is anonymous, add the bot's picture
	if isAnonUsername(username) {
		metadata.Image.Bytes = telegram.BotProfilePicture
		return
	}

	// if the user has a profile picture, add it
	picture, err := telegram.DownloadProfilePicture(tb, user)
	if err != nil {
		log.Debugf("[LNURL] Couldn't download user %s's profile picture: %v", username, err)
		// in case the user has no image, use bot's picture
		metadata.Image.Bytes = telegram.BotProfilePicture
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

func extractSenderFromPayerdata(payer lnurl.PayerDataValues) string {
	if payer.LightningAddress != "" {
		return payer.LightningAddress
	}
	if payer.Email != "" {
		return payer.Email
	}
	if payer.FreeName != "" {
		return payer.FreeName
	}
	return ""
}
