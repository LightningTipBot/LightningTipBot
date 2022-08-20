package telegram

import (
	"fmt"
	"time"

	"github.com/LightningTipBot/LightningTipBot/internal/lnbits"
	"github.com/LightningTipBot/LightningTipBot/internal/str"
	"github.com/LightningTipBot/LightningTipBot/internal/telegram/intercept"
	"github.com/eko/gocache/store"
	log "github.com/sirupsen/logrus"
	tb "gopkg.in/lightningtipbot/telebot.v3"
)

type TransactionsList struct {
	ID           string          `json:"id"`
	User         *lnbits.User    `json:"from"`
	Payments     lnbits.Payments `json:"payments"`
	LanguageCode string          `json:"languagecode"`
	CurrentPage  int             `json:"currentpage"`
	MaxPages     int             `json:"maxpages"`
	TxPerPage    int             `json:"txperpage"`
}

func (txlist *TransactionsList) printTransactions(ctx intercept.Context) string {
	txstr := ""
	// for _, p := range payments {
	payments := txlist.Payments
	pagenr := txlist.CurrentPage
	tx_per_page := txlist.TxPerPage
	if pagenr > (len(payments)+1)/tx_per_page {
		pagenr = 0
	}
	if len(payments) < tx_per_page {
		tx_per_page = len(payments)
	}
	start := pagenr * (tx_per_page - 1)
	end := start + tx_per_page
	if end >= len(payments) {
		end = len(payments) - 1
	}
	for i := start; i <= end; i++ {
		p := payments[i]
		if p.Pending {
			txstr += "🔄"
		} else {
			if p.Amount < 0 {
				txstr += "🔴"
			} else {
				txstr += "🟢"
			}
		}
		timestr := time.Unix(int64(p.Time), 0).UTC().Format("2 Jan 06 15:04")
		txstr += fmt.Sprintf("` %s`", timestr)
		txstr += fmt.Sprintf("` %+d sat`", p.Amount/1000)
		if p.Fee > 0 {
			fee := p.Fee
			if fee < 1000 {
				fee = 1000
			}
			txstr += fmt.Sprintf(" _(fee: %d sat)_", fee/1000)
		}
		memo := p.Memo
		memo_maxlen := 50
		if len(memo) > memo_maxlen {
			memo = memo[:memo_maxlen] + "..."
		}
		if len(memo) > 0 {
			txstr += fmt.Sprintf("\n✉️ %s", str.MarkdownEscape(memo))
		}
		txstr += "\n"
	}
	txstr += fmt.Sprintf("\nShowing %d transactions. Page %d of %d.", len(payments), txlist.CurrentPage+1, txlist.MaxPages)
	return txstr
}

var (
	transactionsMeno           = &tb.ReplyMarkup{ResizeKeyboard: true}
	btnLeftTransactionsButton  = inlineTipjarMenu.Data("◀️", "left_transactions")
	btnRightTransactionsButton = inlineTipjarMenu.Data("▶️", "right_transactions")
)

func (bot *TipBot) makeTransactionsKeyboard(ctx intercept.Context, txlist TransactionsList) *tb.ReplyMarkup {
	leftTransactionsButton := transactionsMeno.Data("←", "left_transactions", txlist.ID)
	rightTransactionsButton := transactionsMeno.Data("→", "right_transactions", txlist.ID)

	if txlist.CurrentPage == 0 {
		transactionsMeno.Inline(
			transactionsMeno.Row(
				leftTransactionsButton),
		)
	} else if txlist.CurrentPage == txlist.MaxPages-1 {
		transactionsMeno.Inline(
			transactionsMeno.Row(
				rightTransactionsButton),
		)
	} else {
		transactionsMeno.Inline(
			transactionsMeno.Row(
				leftTransactionsButton,
				rightTransactionsButton),
		)
	}
	return transactionsMeno
}

func (bot *TipBot) transactionsHandler(ctx intercept.Context) (intercept.Context, error) {
	m := ctx.Message()
	user := LoadUser(ctx)
	payments, err := bot.Client.Payments(*user.Wallet)
	if err != nil {
		log.Errorf("[transactions] Error: %s", err.Error())
		return ctx, err
	}
	tx_per_page := 10
	transactionsList := TransactionsList{
		ID:           fmt.Sprintf("txlist:%d:%s", user.Telegram.ID, RandStringRunes(5)),
		User:         user,
		Payments:     payments,
		LanguageCode: ctx.Value("userLanguageCode").(string),
		CurrentPage:  0,
		TxPerPage:    tx_per_page,
		MaxPages:     (len(payments)+1)/tx_per_page + 1,
	}
	bot.Cache.Set(fmt.Sprintf("%s_transactions", user.Name), transactionsList, &store.Options{Expiration: 1 * time.Minute})
	txstr := transactionsList.printTransactions(ctx)
	bot.trySendMessage(m.Sender, txstr, bot.makeTransactionsKeyboard(ctx, transactionsList))
	return ctx, nil
}

func (bot *TipBot) transactionsScrollLeftHandler(ctx intercept.Context) (intercept.Context, error) {
	c := ctx.Callback()
	user := LoadUser(ctx)
	transactionsListInterface, err := bot.Cache.Get(fmt.Sprintf("%s_transactions", user.Name))
	if err != nil {
		log.WithFields(log.Fields{
			"module": "telegram",
			"func":   "transactionsScrollLeftHandler",
		}).Info("Transactions not in cache anymore")
		return ctx, err
	}
	transactionsList := transactionsListInterface.(TransactionsList)

	if c.Sender.ID == transactionsList.User.Telegram.ID {
		if transactionsList.CurrentPage < transactionsList.MaxPages-1 {
			transactionsList.CurrentPage++
		} else {
			return ctx, err
		}
		bot.Cache.Set(fmt.Sprintf("%s_transactions", user.Name), transactionsList, &store.Options{Expiration: 1 * time.Minute})
		bot.tryEditMessage(c.Message, transactionsList.printTransactions(ctx), bot.makeTransactionsKeyboard(ctx, transactionsList))
	}
	return ctx, nil
}

func (bot *TipBot) transactionsScrollRightHandler(ctx intercept.Context) (intercept.Context, error) {
	c := ctx.Callback()
	user := LoadUser(ctx)
	transactionsListInterface, err := bot.Cache.Get(fmt.Sprintf("%s_transactions", user.Name))
	if err != nil {
		log.WithFields(log.Fields{
			"module": "telegram",
			"func":   "transactionsScrollRightHandler"}).Info("Transactions not in cache anymore")
		return ctx, err
	}
	transactionsList := transactionsListInterface.(TransactionsList)

	if c.Sender.ID == transactionsList.User.Telegram.ID {
		if transactionsList.CurrentPage > 0 {
			transactionsList.CurrentPage--
		} else {
			return ctx, nil
		}
		bot.Cache.Set(fmt.Sprintf("%s_transactions", user.Name), transactionsList, &store.Options{Expiration: 1 * time.Minute})
		bot.tryEditMessage(c.Message, transactionsList.printTransactions(ctx), bot.makeTransactionsKeyboard(ctx, transactionsList))
	}
	return ctx, nil
}
