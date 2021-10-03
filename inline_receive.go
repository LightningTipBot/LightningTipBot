package main

import (
	"context"
	"fmt"

	"github.com/LightningTipBot/LightningTipBot/internal/lnbits"
	"github.com/LightningTipBot/LightningTipBot/internal/storage/transaction"

	"github.com/LightningTipBot/LightningTipBot/internal/runtime"
	log "github.com/sirupsen/logrus"
	tb "gopkg.in/tucnak/telebot.v2"
)

var (
	inlineReceiveMenu      = &tb.ReplyMarkup{ResizeReplyKeyboard: true}
	btnCancelInlineReceive = inlineReceiveMenu.Data("🚫 Cancel", "cancel_receive_inline")
	btnAcceptInlineReceive = inlineReceiveMenu.Data("💸 Pay", "confirm_receive_inline")
)

type InlineReceive struct {
	*transaction.Base
	Message      string       `json:"inline_receive_message"`
	Amount       int          `json:"inline_receive_amount"`
	From         *lnbits.User `json:"inline_receive_from"`
	To           *lnbits.User `json:"inline_receive_to"`
	Memo         string
	LanguageCode string `json:"languagecode"`
}

func (bot TipBot) makeReceiveKeyboard(ctx context.Context, id string) *tb.ReplyMarkup {
	acceptInlineReceiveButton := inlineReceiveMenu.Data(Translate(ctx, "payReceiveButtonMessage"), "confirm_receive_inline")
	cancelInlineReceiveButton := inlineReceiveMenu.Data(Translate(ctx, "cancelButtonMessage"), "cancel_receive_inline")
	acceptInlineReceiveButton.Data = id
	cancelInlineReceiveButton.Data = id
	inlineReceiveMenu.Inline(
		inlineReceiveMenu.Row(
			acceptInlineReceiveButton,
			cancelInlineReceiveButton),
	)
	return inlineFaucetMenu
}

func (bot TipBot) handleInlineReceiveQuery(ctx context.Context, q *tb.Query) {
	from := LoadUser(ctx)
	amount, err := decodeAmountFromCommand(q.Text)
	if err != nil {
		bot.inlineQueryReplyWithError(q, Translate(ctx, "inlineQueryReceiveTitle"), fmt.Sprintf(Translate(ctx, "inlineQueryReceiveDescription"), bot.telegram.Me.Username))
		return
	}
	if amount < 1 {
		bot.inlineQueryReplyWithError(q, Translate(ctx, "inlineSendInvalidAmountMessage"), fmt.Sprintf(Translate(ctx, "inlineQueryReceiveDescription"), bot.telegram.Me.Username))
		return
	}
	fromUserStr := GetUserStr(&q.From)
	// check for memo in command
	memo := GetMemoFromCommand(q.Text, 2)
	urls := []string{
		queryImage,
	}
	results := make(tb.Results, len(urls)) // []tb.Result
	for i, url := range urls {
		inlineMessage := fmt.Sprintf(Translate(ctx, "inlineReceiveMessage"), fromUserStr, amount)
		if len(memo) > 0 {
			inlineMessage = inlineMessage + fmt.Sprintf(Translate(ctx, "inlineReceiveAppendMemo"), amount)
		}
		result := &tb.ArticleResult{
			// URL:         url,
			Text:        inlineMessage,
			Title:       fmt.Sprintf(TranslateUser(ctx, "inlineResultReceiveTitle"), amount),
			Description: fmt.Sprintf(TranslateUser(ctx, "inlineResultReceiveDescription"), amount),
			// required for photos
			ThumbURL: url,
		}
		id := fmt.Sprintf("inl-receive-%d-%d-%s", q.From.ID, amount, RandStringRunes(5))
		result.ReplyMarkup = &tb.InlineKeyboardMarkup{InlineKeyboard: bot.makeReceiveKeyboard(ctx, id).InlineKeyboard}
		results[i] = result
		// needed to set a unique string ID for each result
		results[i].SetResultID(id)
		// create persistend inline send struct
		inlineReceive := InlineReceive{
			Base:         transaction.New(transaction.ID(id)),
			Message:      inlineMessage,
			To:           from,
			Memo:         memo,
			LanguageCode: ctx.Value("publicLanguageCode").(string),
		}
		runtime.IgnoreError(inlineReceive.Set(inlineReceive, bot.bunt))
	}
	err = bot.telegram.Answer(q, &tb.QueryResponse{
		Results:   results,
		CacheTime: 1, // 60 == 1 minute, todo: make higher than 1 s in production

	})
	if err != nil {
		log.Errorln(err)
	}
}

func (bot *TipBot) acceptInlineReceiveHandler(ctx context.Context, c *tb.Callback) {
	tx := &InlineReceive{Base: transaction.New(transaction.ID(c.Data))}
	rn, err := tx.Get(tx, bot.bunt)
	// immediatelly set intransaction to block duplicate calls
	if err != nil {
		log.Errorf("[getInlineReceive] %s", err)
		return
	}
	inlineReceive := rn.(*InlineReceive)
	err = inlineReceive.Lock(inlineReceive, bot.bunt)
	if err != nil {
		log.Errorf("[acceptInlineReceiveHandler] %s", err)
		return
	}

	if !inlineReceive.Active {
		log.Errorf("[acceptInlineReceiveHandler] inline receive not active anymore")
		return
	}

	defer inlineReceive.Release(inlineReceive, bot.bunt)

	// user `from` is the one who is SENDING
	// user `to` is the one who is RECEIVING
	from := LoadUser(ctx)
	to := inlineReceive.To
	toUserStrMd := GetUserStrMd(to.Telegram)
	fromUserStrMd := GetUserStrMd(from.Telegram)
	toUserStr := GetUserStr(to.Telegram)
	fromUserStr := GetUserStr(from.Telegram)

	if from.Telegram.ID == to.Telegram.ID {
		bot.trySendMessage(from.Telegram, Translate(ctx, "sendYourselfMessage"))
		return
	}
	// balance check of the user
	balance, err := bot.GetUserBalance(from)
	if err != nil {
		errmsg := fmt.Sprintf("could not get balance of user %s", fromUserStr)
		log.Errorln(errmsg)
		return
	}
	// check if fromUser has balance
	if balance < inlineReceive.Amount {
		log.Errorf("[acceptInlineReceiveHandler] balance of user %s too low", fromUserStr)
		bot.trySendMessage(from.Telegram, Translate(ctx, "inlineSendBalanceLowMessage"))
		return
	}

	// set inactive to avoid double-sends
	inlineReceive.Inactivate(inlineReceive, bot.bunt)

	// todo: user new get username function to get userStrings
	transactionMemo := fmt.Sprintf("InlineReceive from %s to %s (%d sat).", fromUserStr, toUserStr, inlineReceive.Amount)
	t := NewTransaction(bot, from, to, inlineReceive.Amount, TransactionType("inline receive"))
	t.Memo = transactionMemo
	success, err := t.Send()
	if !success {
		errMsg := fmt.Sprintf("[acceptInlineReceiveHandler] Transaction failed: %s", err)
		log.Errorln(errMsg)
		bot.tryEditMessage(c.Message, bot.Translate(inlineReceive.LanguageCode, "inlineReceiveFailedMessage"), &tb.ReplyMarkup{})
		return
	}

	log.Infof("[acceptInlineReceiveHandler] %d sat from %s to %s", inlineReceive.Amount, fromUserStr, toUserStr)

	inlineReceive.Message = fmt.Sprintf("%s", fmt.Sprintf(bot.Translate(inlineReceive.LanguageCode, "inlineSendUpdateMessageAccept"), inlineReceive.Amount, fromUserStrMd, toUserStrMd))
	memo := inlineReceive.Memo
	if len(memo) > 0 {
		inlineReceive.Message = inlineReceive.Message + fmt.Sprintf(bot.Translate(inlineReceive.LanguageCode, "inlineReceiveAppendMemo"), memo)
	}

	if !to.Initialized {
		inlineReceive.Message += "\n\n" + fmt.Sprintf(bot.Translate(inlineReceive.LanguageCode, "inlineSendCreateWalletMessage"), GetUserStrMd(bot.telegram.Me))
	}

	bot.tryEditMessage(c.Message, inlineReceive.Message, &tb.ReplyMarkup{})
	// notify users
	_, err = bot.telegram.Send(to.Telegram, fmt.Sprintf(bot.Translate(to.Telegram.LanguageCode, "sendReceivedMessage"), fromUserStrMd, inlineReceive.Amount))
	_, err = bot.telegram.Send(from.Telegram, fmt.Sprintf(bot.Translate(from.Telegram.LanguageCode, "sendSentMessage"), inlineReceive.Amount, toUserStrMd))
	if err != nil {
		errmsg := fmt.Errorf("[acceptInlineReceiveHandler] Error: Receive message to %s: %s", toUserStr, err)
		log.Errorln(errmsg)
		return
	}
}

func (bot *TipBot) cancelInlineReceiveHandler(ctx context.Context, c *tb.Callback) {
	tx := &InlineReceive{Base: transaction.New(transaction.ID(c.Data))}
	rn, err := tx.Get(tx, bot.bunt)
	// immediatelly set intransaction to block duplicate calls
	if err != nil {
		log.Errorf("[cancelInlineReceiveHandler] %s", err)
		return
	}
	inlineReceive := rn.(*InlineReceive)
	if c.Sender.ID == inlineReceive.To.Telegram.ID {
		bot.tryEditMessage(c.Message, bot.Translate(inlineReceive.LanguageCode, "inlineReceiveCancelledMessage"), &tb.ReplyMarkup{})
		// set the inlineReceive inactive
		inlineReceive.Active = false
		inlineReceive.InTransaction = false
		runtime.IgnoreError(inlineReceive.Set(inlineReceive, bot.bunt))
	}
	return
}
