package telegram

import (
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/LightningTipBot/LightningTipBot/internal/lnbits"
	tb "gopkg.in/lightningtipbot/telebot.v3"
)

type Transaction struct {
	ID           uint           `gorm:"primarykey"`
	Time         time.Time      `json:"time"`
	Bot          *TipBot        `gorm:"-"`
	From         *lnbits.User   `json:"from" gorm:"-"`
	To           *lnbits.User   `json:"to" gorm:"-"`
	FromId       int64          `json:"from_id" `
	ToId         int64          `json:"to_id" `
	FromUser     string         `json:"from_user"`
	ToUser       string         `json:"to_user"`
	Type         string         `json:"type"`
	Amount       int64          `json:"amount"`
	ChatID       int64          `json:"chat_id"`
	ChatName     string         `json:"chat_name"`
	Memo         string         `json:"memo"`
	Success      bool           `json:"success"`
	FromWallet   string         `json:"from_wallet"`
	ToWallet     string         `json:"to_wallet"`
	FromLNbitsID string         `json:"from_lnbits"`
	ToLNbitsID   string         `json:"to_lnbits"`
	Invoice      lnbits.Invoice `gorm:"embedded;embeddedPrefix:invoice_"`
}

type TransactionOption func(t *Transaction)

func TransactionChat(chat *tb.Chat) TransactionOption {
	return func(t *Transaction) {
		t.ChatID = chat.ID
		t.ChatName = chat.Title
	}
}

func TransactionType(transactionType string) TransactionOption {
	return func(t *Transaction) {
		t.Type = transactionType
	}
}

func NewTransaction(bot *TipBot, from *lnbits.User, to *lnbits.User, amount int64, opts ...TransactionOption) *Transaction {
	t := &Transaction{
		Bot:      bot,
		From:     from,
		To:       to,
		FromUser: GetUserStr(from.Telegram),
		ToUser:   GetUserStr(to.Telegram),
		FromId:   from.Telegram.ID,
		ToId:     to.Telegram.ID,
		Amount:   amount,
		Memo:     "Powered by @LightningTipBot",
		Time:     time.Now(),
		Success:  false,
	}
	for _, opt := range opts {
		opt(t)
	}
	return t

}

func (t *Transaction) Send() (success bool, err error) {
	success, err = t.SendTransaction(t.Bot, t.From, t.To, t.Amount, t.Memo)
	if success {
		t.Success = success
	}

	// save transaction to db
	tx := t.Bot.DB.Transactions.Save(t)
	if tx.Error != nil {
		log.WithFields(log.Fields{
			"module":       "telegram",
			"func":         "SendTransaction",
			"user":         t.FromUser,
			"user_id":      t.From.ID,
			"wallet_id":    t.From.Wallet.ID,
			"telegram_id":  t.From.Telegram.ID,
			"to_user":      t.ToUser,
			"to_wallet_id": t.To.Wallet.ID,
			"to_user_id":   t.To.ID,
			"error":        tx.Error}).Errorln("could not log transaction")
	}
	return success, err
}

func (t *Transaction) SendTransaction(bot *TipBot, from *lnbits.User, to *lnbits.User, amount int64, memo string) (bool, error) {
	fromUserStr := GetUserStr(from.Telegram)
	toUserStr := GetUserStr(to.Telegram)

	t.FromWallet = from.Wallet.ID
	t.FromLNbitsID = from.ID

	// check if fromUser has balance
	balance, err := bot.GetUserBalance(from)
	if err != nil {
		log.WithFields(log.Fields{
			"module":       "telegram",
			"func":         "SendTransaction",
			"to_user":      toUserStr,
			"to_user_id":   to.ID,
			"to_wallet_id": to.Wallet.ID,
			"user":         fromUserStr,
			"user_id":      from.ID,
			"wallet_id":    from.Wallet.ID,
			"amount":       amount},
		).Errorln("error checking balance")
		return false, err
	}
	// check if fromUser has balance
	if balance < amount {
		errmsg := fmt.Sprintf("balance too low.")
		log.WithFields(log.Fields{
			"module":      "telegram",
			"func":        "SendTransaction",
			"to_user":     to.ID,
			"user":        fromUserStr,
			"user_id":     from.ID,
			"wallet_id":   from.Wallet.ID,
			"telegram_id": from.Telegram.ID}).Warn("balance of too low")
		return false, fmt.Errorf(errmsg)
	}

	t.ToWallet = to.ID
	t.ToLNbitsID = to.ID

	// generate invoice
	invoice, err := to.Wallet.Invoice(
		lnbits.InvoiceParams{
			Amount: int64(amount),
			Out:    false,
			Memo:   memo},
		bot.Client)
	if err != nil {
		log.WithFields(log.Fields{
			"module":      "telegram",
			"func":        "SendTransaction",
			"user":        toUserStr,
			"user_id":     to.ID,
			"wallet_id":   to.Wallet.ID,
			"telegram_id": to.Telegram.ID,
			"error":       err.Error()}).Errorln("could not create invoice")
		return false, err
	}
	t.Invoice = invoice
	// pay invoice
	_, err = from.Wallet.Pay(lnbits.PaymentParams{Out: true, Bolt11: invoice.PaymentRequest}, bot.Client)
	if err != nil {
		log.WithFields(log.Fields{
			"module":       "telegram",
			"func":         "SendTransaction",
			"user":         fromUserStr,
			"user_id":      from.ID,
			"wallet_id":    from.Wallet.ID,
			"telegram_id":  from.Telegram.ID,
			"to_user":      toUserStr,
			"to_wallet_id": to.Wallet.ID,
			"to_user_id":   to.ID,
			"error":        err.Error()}).Warnf("Payment failed")
		return false, err
	}

	// check if fromUser has balance
	_, err = bot.GetUserBalance(from)
	if err != nil {
		log.WithFields(log.Fields{
			"module":       "telegram",
			"func":         "SendTransaction",
			"user":         fromUserStr,
			"user_id":      from.ID,
			"wallet_id":    from.Wallet.ID,
			"telegram_id":  from.Telegram.ID,
			"to_user":      toUserStr,
			"to_wallet_id": to.Wallet.ID,
			"to_user_id":   to.ID,
			"error":        err.Error()}).Errorln("could not get sender balance")
		return false, err
	}
	_, err = bot.GetUserBalance(to)
	if err != nil {
		log.WithFields(log.Fields{
			"module":       "telegram",
			"func":         "SendTransaction",
			"user":         fromUserStr,
			"user_id":      from.ID,
			"wallet_id":    from.Wallet.ID,
			"telegram_id":  from.Telegram.ID,
			"to_user":      toUserStr,
			"to_wallet_id": to.Wallet.ID,
			"to_user_id":   to.ID,
			"error":        err.Error()}).Errorln("could not get receiver balance")
		return false, err
	}

	return true, err
}
