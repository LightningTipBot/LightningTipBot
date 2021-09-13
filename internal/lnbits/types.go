package lnbits

import (
	"github.com/imroc/req"
	tb "gopkg.in/tucnak/telebot.v2"
)

type Client struct {
	header     req.Header
	url        string
	AdminKey   string
	InvoiceKey string
}

type User struct {
	ID          string       `json:"id"`
	Name        string       `json:"name" gorm:"primaryKey"`
	Initialized bool         `json:"initialized"`
	Telegram    *tb.User     `gorm:"embedded;embeddedPrefix:telegram_"`
	Wallet      *Wallet      `gorm:"embedded;embeddedPrefix:wallet_"`
	StateKey    UserStateKey `json:"stateKey"`
	StateData   string       `json:"stateData"`
}

const (
	UserStateConfirmPayment = iota + 1
	UserStateConfirmSend
	UserStateLNURLEnterAmount
	UserStateConfirmLNURLPay
)

type UserStateKey int

func (u *User) ResetState() {
	u.StateData = ""
	u.StateKey = 0
}

type InvoiceParams struct {
	Out             bool   `json:"out"`                        // must be True if invoice is payed, False if invoice is received
	Amount          int64  `json:"amount"`                     // amount in MilliSatoshi
	Memo            string `json:"memo,omitempty"`             // the invoice memo.
	Webhook         string `json:"webhook,omitempty"`          // the webhook to fire back to when payment is received.
	DescriptionHash string `json:"description_hash,omitempty"` // the invoice description hash.
}

type PaymentParams struct {
	Out    bool   `json:"out"`
	Bolt11 string `json:"bolt11"`
}
type PayParams struct {
	// the BOLT11 payment request you want to pay.
	PaymentRequest string `json:"payment_request"`

	// custom data you may want to associate with this invoice. optional.
	PassThru map[string]interface{} `json:"passThru"`
}

type TransferParams struct {
	Memo         string `json:"memo"`           // the transfer description.
	NumSatoshis  int64  `json:"num_satoshis"`   // the transfer amount.
	DestWalletId string `json:"dest_wallet_id"` // the key or id of the destination
}

type Error struct {
	Name    string `json:"name"`
	Message string `json:"message"`
	Code    int    `json:"code"`
	Status  int    `json:"status"`
}

func (err Error) Error() string {
	return err.Message
}

type Wallet struct {
	*Client  `gorm:"-"`
	ID       string `json:"id" gorm:"id"`
	Adminkey string `json:"adminkey"`
	Inkey    string `json:"inkey"`
	Balance  int64  `json:"balance"`
	Name     string `json:"name"`
	User     string `json:"user"`
}
type BitInvoice struct {
	PaymentHash    string `json:"payment_hash"`
	PaymentRequest string `json:"payment_request"`
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
