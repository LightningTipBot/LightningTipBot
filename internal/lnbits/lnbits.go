package lnbits

import (
	"github.com/imroc/req"
)

// NewClient returns a new lnbits api client. Pass your API key and url here.
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

// GetUser returns user information
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

// CreateUserWithInitialWallet creates new user with initial wallet
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

// CreateWallet creates a new wallet.
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

// Invoice creates an invoice associated with this wallet.
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

// Info returns wallet information
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

// Wallets returns all wallets belonging to an user
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
