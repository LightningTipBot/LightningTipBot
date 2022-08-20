package api

import (
	"encoding/json"
	log "github.com/sirupsen/logrus"
	"net/http"

	"github.com/LightningTipBot/LightningTipBot/internal"
	"github.com/LightningTipBot/LightningTipBot/internal/lnbits"
	"github.com/LightningTipBot/LightningTipBot/internal/telegram"
	"github.com/gorilla/mux"
)

type Service struct {
	Bot *telegram.TipBot
}

type ErrorResponse struct {
	Message string `json:"error"`
}

func RespondError(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	log.WithFields(log.Fields{"module": "api", "func": "RespondError"}).Error(message)
	json.NewEncoder(w).Encode(ErrorResponse{Message: message})
}

func (s Service) Balance(w http.ResponseWriter, r *http.Request) {
	user := telegram.LoadUser(r.Context())
	balance, err := s.Bot.GetUserBalance(user)
	if err != nil {
		RespondError(w, "balance check failed")
		return
	}

	balanceResponse := BalanceResponse{
		Balance: balance,
	}
	log.WithFields(log.Fields{
		"module":      "api",
		"func":        "PayInvoice",
		"user":        telegram.GetUserStr(user.Telegram),
		"user_id":     user.ID,
		"amount":      balance,
		"wallet_id":   user.Wallet.ID,
		"telegram_id": user.Telegram.ID}).Info("checked balance")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(balanceResponse)
}

func (s Service) CreateInvoice(w http.ResponseWriter, r *http.Request) {
	user := telegram.LoadUser(r.Context())
	var createInvoiceRequest CreateInvoiceRequest
	err := json.NewDecoder(r.Body).Decode(&createInvoiceRequest)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	invoice, err := user.Wallet.Invoice(
		lnbits.InvoiceParams{
			Amount:              createInvoiceRequest.Amount,
			Out:                 false,
			DescriptionHash:     createInvoiceRequest.DescriptionHash,
			UnhashedDescription: createInvoiceRequest.UnhashedDescription,
			Webhook:             internal.Configuration.Lnbits.WebhookServer},
		s.Bot.Client)
	if err != nil {
		RespondError(w, "could not create invoice")
		return
	}

	log.WithFields(log.Fields{
		"module":      "api",
		"func":        "CreateInvoice",
		"user":        telegram.GetUserStr(user.Telegram),
		"user_id":     user.ID,
		"wallet_id":   user.Wallet.ID,
		"amount":      createInvoiceRequest.Amount,
		"invoice":     invoice.PaymentRequest,
		"telegram_id": user.Telegram.ID}).Info("created invoice")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(invoice)
}

func (s Service) PayInvoice(w http.ResponseWriter, r *http.Request) {
	user := telegram.LoadUser(r.Context())
	var payInvoiceRequest PayInvoiceRequest
	err := json.NewDecoder(r.Body).Decode(&payInvoiceRequest)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	invoice, err := user.Wallet.Pay(lnbits.PaymentParams{Out: true, Bolt11: payInvoiceRequest.PayRequest}, s.Bot.Client)
	if err != nil {
		RespondError(w, "could not pay invoice: "+err.Error())
		return
	}

	payment, _ := s.Bot.Client.Payment(*user.Wallet, invoice.PaymentHash)
	if err != nil {
		// we assume that it's paid since thre was no error earlier
		payment.Paid = true
	}
	log.WithFields(log.Fields{
		"module":      "api",
		"func":        "PayInvoice",
		"user":        telegram.GetUserStr(user.Telegram),
		"user_id":     user.ID,
		"wallet_id":   user.Wallet.ID,
		"invoice":     invoice.PaymentRequest,
		"telegram_id": user.Telegram.ID}).Info("payed invoice")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(payment)
}

func (s Service) PaymentStatus(w http.ResponseWriter, r *http.Request) {
	user := telegram.LoadUser(r.Context())
	payment_hash := mux.Vars(r)["payment_hash"]
	payment, err := s.Bot.Client.Payment(*user.Wallet, payment_hash)
	if err != nil {
		RespondError(w, "could not get payment")
		return
	}
	log.WithFields(log.Fields{
		"module":      "api",
		"func":        "PayInvoice",
		"user":        telegram.GetUserStr(user.Telegram),
		"user_id":     user.ID,
		"status":      payment.Paid,
		"wallet_id":   user.Wallet.ID,
		"telegram_id": user.Telegram.ID}).Info("returning payment status")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(payment)
}

// InvoiceStatus
func (s Service) InvoiceStatus(w http.ResponseWriter, r *http.Request) {
	user := telegram.LoadUser(r.Context())
	payment_hash := mux.Vars(r)["payment_hash"]
	user.Wallet = &lnbits.Wallet{}
	payment, err := s.Bot.Client.Payment(*user.Wallet, payment_hash)
	if err != nil {
		RespondError(w, "could not get invoice")
		return
	}
	log.WithFields(log.Fields{
		"module":      "api",
		"func":        "PayInvoice",
		"user":        telegram.GetUserStr(user.Telegram),
		"user_id":     user.ID,
		"status":      payment.Paid,
		"wallet_id":   user.Wallet.ID,
		"telegram_id": user.Telegram.ID}).Info("returning invoice status")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(payment)
}
