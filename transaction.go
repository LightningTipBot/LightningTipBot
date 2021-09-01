package main

import (
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/LightningTipBot/LightningTipBot/internal/lnbits"
	tb "gopkg.in/tucnak/telebot.v2"
)

type Transaction struct {
	ID           uint      `gorm:"primarykey"`
	Time         time.Time `json:"time"`
	Bot          *TipBot   `gorm:"-"`
	From         *tb.User  `json:"from" gorm:"-"`
	To           *tb.User  `json:"to" gorm:"-"`
	FromId       int       `json:"from_id" `
	ToId         int       `json:"to_id" `
	FromUser     string    `json:"from_user"`
	ToUser       string    `json:"to_user"`
	Type         string    `json:"type"`
	Amount       int       `json:"amount"`
	ChatID       int64     `json:"chat_id"`
	ChatName     string    `json:"chat_name"`
	Memo         string    `json:"memo"`
	Success      bool      `json:"success"`
	FromWallet   string    `json:"from_wallet"`
	ToWallet     string    `json:"to_wallet"`
	FromLNbitsID string    `json:"from_lnbits"`
	ToLNbitsID   string    `json:"to_lnbits"`
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

func NewTransaction(bot *TipBot, from *tb.User, to *tb.User, amount int, opts ...TransactionOption) *Transaction {
	t := &Transaction{
		Bot:      bot,
		From:     from,
		To:       to,
		FromUser: GetUserStr(from),
		ToUser:   GetUserStr(to),
		FromId:   from.ID,
		ToId:     to.ID,
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
	// maybe remove comments, GTP-3 dreamed this up but it's nice:
	// if t.From.ID == t.To.ID {
	// 	err = fmt.Errorf("Can not send transaction to yourself.")
	// 	return false, err
	// }

	// todo: remove this commend if the backend is back up
	success, err = t.SendTransaction(t.Bot, t.From, t.To, t.Amount, t.Memo)
	// success = true
	if success {
		t.Success = success
		// TODO: call post-send methods
	}

	// save transaction to db
	tx := t.Bot.logger.Save(t)
	if tx.Error != nil {
		errMsg := fmt.Sprintf("Error: Could not log transaction: %s", err)
		log.Errorln(errMsg)
	}

	return success, err
}

func (t *Transaction) SendTransaction(bot *TipBot, from *tb.User, to *tb.User, amount int, memo string) (bool, error) {
	fromUserStr := GetUserStr(from)
	toUserStr := GetUserStr(to)

	// from := m.Sender
	fromUser, err := GetUser(from, *bot)
	if err != nil {
		errmsg := fmt.Sprintf("[SendTransaction] Error: Could not get user %s", fromUserStr)
		log.Errorln(errmsg)
		return false, err
	}
	t.FromWallet = fromUser.Wallet.ID
	t.FromLNbitsID = fromUser.ID
	// check if fromUser has balance
	balance, err := bot.GetUserBalance(from)
	if err != nil {
		errmsg := fmt.Sprintf("[SendTransaction] Error: Could not get balance of user %s", fromUserStr)
		log.Errorln(errmsg)
		return false, err
	}
	// check if fromUser has balance
	if balance < amount {
		errmsg := fmt.Sprintf("[SendTransaction] Error: User %s has not enough balance", fromUserStr)
		bot.trySendMessage(from, fmt.Sprintf("🚫 Your balance is too low for that."))
		log.Errorln(errmsg)
		return false, fmt.Errorf(errmsg)
	}

	toUser, err := GetUser(to, *bot)
	if err != nil {
		errmsg := fmt.Sprintf("[SendTransaction] Error: ToUser %s not found: %s", toUserStr, err)
		log.Errorln(errmsg)
		return false, err
	}
	t.ToWallet = toUser.Wallet.ID
	t.ToLNbitsID = toUser.ID

	// generate invoice
	invoice, err := toUser.Wallet.Invoice(
		lnbits.InvoiceParams{
			Amount: int64(amount),
			Out:    false,
			Memo:   memo},
		*toUser.Wallet)
	if err != nil {
		errmsg := fmt.Sprintf("[SendTransaction] Error: Could not create invoice for user %s", toUserStr)
		log.Errorln(errmsg)
		return false, err
	}
	// pay invoice
	_, err = fromUser.Wallet.Pay(lnbits.PaymentParams{Out: true, Bolt11: invoice.PaymentRequest}, *fromUser.Wallet)
	if err != nil {
		errmsg := fmt.Sprintf("[SendTransaction] Error: Payment from %s to %s of %d sat failed", fromUserStr, toUserStr, amount)
		log.Errorln(errmsg)
		return false, err
	}
	return true, err
}
