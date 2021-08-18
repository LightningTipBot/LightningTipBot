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

// NewClient is the first function you must call. Pass your main API key here.
// It will return a client you can later use to access wallets and transactions.
// You can find it at https://lnpay.co/developers/dashboard
func NewClient(key, url string) *Client {
	return &Client{
		url: url,
		header: req.Header{
			"Content-Type": "application/json",
			"Accept":       "application/json",
			"X-Api-Key":    key,
		},
	}
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
type UserStateKey int

const (
	UserStateConfirmPayment = iota + 1
	UserStateConfirmSend
)

func (u *User) ResetState() {
	u.StateData = ""
	u.StateKey = 0
}

// CreateWallet creates a new wallet with a given descriptive label.
// It will return the wallet object which you can use to create invoices and payments.
// https://docs.lnpay.co/wallet/create-wallet
func (c *Client) GetUser(userId string) (user User, err error) {
	resp, err := req.Post(c.url+"/usermanager/api/v1/users/"+userId, c.header, nil)
	if err != nil {
		return
	}

	if resp.Response().StatusCode >= 300 {
		var reqErr Error
		resp.ToJSON(&reqErr)
		err = reqErr
		return
	}

	err = resp.ToJSON(&user)
	return
}

// CreateWallet creates a new wallet with a given descriptive label.
// It will return the wallet object which you can use to create invoices and payments.
// https://docs.lnpay.co/wallet/create-wallet
func (c *Client) CreateUserWithInitialWallet(userName, walletName, adminId string, email string) (wal User, err error) {
	resp, err := req.Post(c.url+"/usermanager/api/v1/users", c.header, req.BodyJSON(struct {
		WalletName string `json:"wallet_name"`
		AdminId    string `json:"admin_id"`
		UserName   string `json:"user_name"`
		Email      string `json:"email"`
	}{walletName, adminId, userName, email}))
	if err != nil {
		return
	}

	if resp.Response().StatusCode >= 300 {
		var reqErr Error
		resp.ToJSON(&reqErr)
		err = reqErr
		return
	}
	err = resp.ToJSON(&wal)
	return
}

// CreateWallet creates a new wallet with a given descriptive label.
// It will return the wallet object which you can use to create invoices and payments.
// https://docs.lnpay.co/wallet/create-wallet
func (c *Client) CreateWallet(userId, walletName, adminId string) (wal Wallet, err error) {
	resp, err := req.Post(c.url+"/usermanager/api/v1/wallets", c.header, req.BodyJSON(struct {
		UserId     string `json:"user_id"`
		WalletName string `json:"wallet_name"`
		AdminId    string `json:"admin_id"`
	}{userId, walletName, adminId}))
	if err != nil {
		return
	}

	if resp.Response().StatusCode >= 300 {
		var reqErr Error
		resp.ToJSON(&reqErr)
		err = reqErr
		return
	}
	err = resp.ToJSON(&wal)
	wal.Client = c
	return
}

type InvoiceParams struct {
	Out     bool   `json:"out"`
	Amount  int64  `json:"amount"`
	Memo    string `json:"memo"` // the invoice description.
	Webhook string `json:"webhook,omitempty"`
}

// Invoice creates an invoice associated with this wallet.
// https://docs.lnpay.co/wallet/generate-invoice
func (c Client) Invoice(params InvoiceParams, w Wallet) (lntx BitInvoice, err error) {
	c.header["X-Api-Key"] = w.Adminkey
	resp, err := req.Post(c.url+"/api/v1/payments", w.header, req.BodyJSON(&params))
	if err != nil {
		return
	}

	if resp.Response().StatusCode >= 300 {
		var reqErr Error
		resp.ToJSON(&reqErr)
		err = reqErr
		return
	}

	err = resp.ToJSON(&lntx)
	return
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

func (c Client) Info(w Wallet) (wtx Wallet, err error) {
	c.header["X-Api-Key"] = w.Adminkey
	resp, err := req.Get(w.url+"/api/v1/wallet", w.header, nil)
	if err != nil {
		return
	}

	if resp.Response().StatusCode >= 300 {
		var reqErr Error
		resp.ToJSON(&reqErr)
		err = reqErr
		return
	}

	err = resp.ToJSON(&wtx)
	return
}
func (c Client) Wallets(w User) (wtx []Wallet, err error) {
	resp, err := req.Get(c.url+"/usermanager/api/v1/wallets/"+w.ID, c.header, nil)
	if err != nil {
		return
	}

	if resp.Response().StatusCode >= 300 {
		var reqErr Error
		resp.ToJSON(&reqErr)
		err = reqErr
		return
	}

	err = resp.ToJSON(&wtx)
	return
}

// Pay pays a given invoice with funds from the wallet.
// https://docs.lnpay.co/wallet/pay-invoice
func (c Client) Pay(params PaymentParams, w Wallet) (wtx BitInvoice, err error) {
	c.header["X-Api-Key"] = w.Adminkey
	resp, err := req.Post(c.url+"/api/v1/payments", w.header, req.BodyJSON(&params))
	if err != nil {
		return
	}

	if resp.Response().StatusCode >= 300 {
		var reqErr Error
		resp.ToJSON(&reqErr)
		err = reqErr
		return
	}

	err = resp.ToJSON(&wtx)
	return
}

type TransferParams struct {
	Memo         string `json:"memo"`           // the transfer description.
	NumSatoshis  int64  `json:"num_satoshis"`   // the transfer amount.
	DestWalletId string `json:"dest_wallet_id"` // the key or id of the destination
}
