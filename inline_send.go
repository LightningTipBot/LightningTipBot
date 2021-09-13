package main

import (
	"fmt"
	"time"

	"github.com/LightningTipBot/LightningTipBot/internal/runtime"
	log "github.com/sirupsen/logrus"
	tb "gopkg.in/tucnak/telebot.v2"
)

const (
	inlineSendMessage              = "Press ✅ to receive payment from %s.\n\n💸 Amount: %d sat"
	inlineSendAppendMemo           = "\n✉️ %s"
	inlineSendUpdateMessageAccept  = "💸 %d sat sent from %s to %s."
	inlineSendCreateWalletMessage  = "Chat with %s 👈 to manage your wallet."
	sendYourselfMessage            = "📖 You can't pay to yourself."
	inlineSendFailedMessage        = "🚫 Send failed."
	inlineSendInvalidAmountMessage = "🚫 Amount must be larger than 0."
	inlineSendBalanceLowMessage    = "🚫 Your balance is too low (👑 %d sat)."
)

var (
	inlineQuerySendTitle        = "💸 Send payment to a chat."
	inlineQuerySendDescription  = "Usage: @%s send <amount> [<memo>]"
	inlineResultSendTitle       = "💸 Send %d sat."
	inlineResultSendDescription = "👉 Click to send %d sat to this chat."
	inlineSendMenu              = &tb.ReplyMarkup{ResizeReplyKeyboard: true}
	btnCancelInlineSend         = inlineSendMenu.Data("🚫 Cancel", "cancel_send_inline")
	btnAcceptInlineSend         = inlineSendMenu.Data("✅ Receive", "confirm_send_inline")
)

type InlineSend struct {
	Message       string   `json:"inline_send_message"`
	Amount        int      `json:"inline_send_amount"`
	From          *tb.User `json:"inline_send_from"`
	To            *tb.User `json:"inline_send_to"`
	Memo          string   `json:"inline_send_memo"`
	ID            string   `json:"inline_send_id"`
	Active        bool     `json:"inline_send_active"`
	InTransaction bool     `json:"inline_send_intransaction"`
}

func NewInlineSend() *InlineSend {
	inlineSend := &InlineSend{
		Message:       "",
		Active:        true,
		InTransaction: false,
	}
	return inlineSend

}

func (msg InlineSend) Key() string {
	return msg.ID
}

func (bot *TipBot) LockSend(tx *InlineSend) error {
	// immediatelly set intransaction to block duplicate calls
	tx.InTransaction = true
	err := bot.bunt.Set(tx)
	if err != nil {
		return err
	}
	return nil
}

func (bot *TipBot) ReleaseSend(tx *InlineSend) error {
	// immediatelly set intransaction to block duplicate calls
	tx.InTransaction = false
	err := bot.bunt.Set(tx)
	if err != nil {
		return err
	}
	return nil
}

func (bot *TipBot) inactivateSend(tx *InlineSend) error {
	tx.Active = false
	err := bot.bunt.Set(tx)
	if err != nil {
		return err
	}
	return nil
}

func (bot *TipBot) getInlineSend(c *tb.Callback) (*InlineSend, error) {
	inlineSend := NewInlineSend()
	inlineSend.ID = c.Data

	err := bot.bunt.Get(inlineSend)

	// to avoid race conditions, we block the call if there is
	// already an active transaction by loop until InTransaction is false
	ticker := time.NewTicker(time.Second * 10)

	for inlineSend.InTransaction {
		select {
		case <-ticker.C:
			return nil, fmt.Errorf("inline send timeout")
		default:
			log.Infoln("in transaction")
			time.Sleep(time.Duration(500) * time.Millisecond)
			err = bot.bunt.Get(inlineSend)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("could not get inline send message")
	}

	return inlineSend, nil

}

func (bot TipBot) handleInlineSendQuery(q *tb.Query) {
	inlineSend := NewInlineSend()
	var err error
	inlineSend.Amount, err = decodeAmountFromCommand(q.Text)
	if err != nil {
		bot.inlineQueryReplyWithError(q, inlineQuerySendTitle, fmt.Sprintf(inlineQuerySendDescription, bot.telegram.Me.Username))
		return
	}
	if inlineSend.Amount < 1 {
		bot.inlineQueryReplyWithError(q, inlineSendInvalidAmountMessage, fmt.Sprintf(inlineQuerySendDescription, bot.telegram.Me.Username))
		return
	}
	fromUserStr := GetUserStr(&q.From)
	balance, err := bot.GetUserBalance(&q.From)
	if err != nil {
		errmsg := fmt.Sprintf("could not get balance of user %s", fromUserStr)
		log.Errorln(errmsg)
		return
	}
	// check if fromUser has balance
	if balance < inlineSend.Amount {
		log.Errorln("Balance of user %s too low", fromUserStr)
		bot.inlineQueryReplyWithError(q, fmt.Sprintf(inlineSendBalanceLowMessage, balance), fmt.Sprintf(inlineQuerySendDescription, bot.telegram.Me.Username))
		return
	}

	// check for memo in command
	inlineSend.Memo = GetMemoFromCommand(q.Text, 2)

	urls := []string{
		queryImage,
	}
	results := make(tb.Results, len(urls)) // []tb.Result
	for i, url := range urls {

		inlineMessage := fmt.Sprintf(inlineSendMessage, fromUserStr, inlineSend.Amount)

		if len(inlineSend.Memo) > 0 {
			inlineMessage = inlineMessage + fmt.Sprintf(inlineSendAppendMemo, inlineSend.Memo)
		}

		result := &tb.ArticleResult{
			// URL:         url,
			Text:        inlineMessage,
			Title:       fmt.Sprintf(inlineResultSendTitle, inlineSend.Amount),
			Description: fmt.Sprintf(inlineResultSendDescription, inlineSend.Amount),
			// required for photos
			ThumbURL: url,
		}
		id := fmt.Sprintf("inl-send-%d-%d-%s", q.From.ID, inlineSend.Amount, RandStringRunes(5))
		btnAcceptInlineSend.Data = id
		btnCancelInlineSend.Data = id
		inlineSendMenu.Inline(inlineSendMenu.Row(btnAcceptInlineSend, btnCancelInlineSend))
		result.ReplyMarkup = &tb.InlineKeyboardMarkup{InlineKeyboard: inlineSendMenu.InlineKeyboard}

		results[i] = result

		// needed to set a unique string ID for each result
		results[i].SetResultID(id)

		// add data to persistent object
		inlineSend.Message = inlineMessage
		inlineSend.ID = id
		inlineSend.From = &q.From
		// add result to persistent struct
		runtime.IgnoreError(bot.bunt.Set(inlineSend))
	}

	err = bot.telegram.Answer(q, &tb.QueryResponse{
		Results:   results,
		CacheTime: 1, // 60 == 1 minute, todo: make higher than 1 s in production

	})
	if err != nil {
		log.Errorln(err)
	}
}

func (bot *TipBot) acceptInlineSendHandler(c *tb.Callback) {
	inlineSend, err := bot.getInlineSend(c)
	if err != nil {
		log.Errorf("[acceptInlineSendHandler] %s", err)
		return
	}
	// immediatelly set intransaction to block duplicate calls
	err = bot.LockSend(inlineSend)
	if err != nil {
		log.Errorf("[getInlineSend] %s", err)
		return
	}
	if !inlineSend.Active {
		log.Errorf("[acceptInlineSendHandler] inline send not active anymore")
		return
	}

	defer bot.ReleaseSend(inlineSend)

	amount := inlineSend.Amount
	to := c.Sender
	from := inlineSend.From

	if from.ID == to.ID {
		bot.trySendMessage(from, sendYourselfMessage)
		return
	}

	toUserStrMd := GetUserStrMd(to)
	fromUserStrMd := GetUserStrMd(from)
	toUserStr := GetUserStr(to)
	fromUserStr := GetUserStr(from)

	// check if user exists and create a wallet if not
	_, exists := bot.UserExists(to)
	if !exists {
		log.Infof("[sendInline] User %s has no wallet.", toUserStr)
		err = bot.CreateWalletForTelegramUser(to)
		if err != nil {
			errmsg := fmt.Errorf("[sendInline] Error: Could not create wallet for %s", toUserStr)
			log.Errorln(errmsg)
			return
		}
	}
	// set inactive to avoid double-sends
	bot.inactivateSend(inlineSend)

	// todo: user new get username function to get userStrings
	transactionMemo := fmt.Sprintf("Send from %s to %s (%d sat).", fromUserStr, toUserStr, amount)
	t := NewTransaction(bot, from, to, amount, TransactionType("inline send"))
	t.Memo = transactionMemo
	success, err := t.Send()
	if !success {
		if err != nil {
			bot.trySendMessage(from, fmt.Sprintf(tipErrorMessage, err))
		} else {
			bot.trySendMessage(from, fmt.Sprintf(tipErrorMessage, tipUndefinedErrorMsg))
		}
		errMsg := fmt.Sprintf("[sendInline] Transaction failed: %s", err)
		log.Errorln(errMsg)
		bot.tryEditMessage(c.Message, inlineSendFailedMessage, &tb.ReplyMarkup{})
		return
	}

	log.Infof("[sendInline] %d sat from %s to %s", amount, fromUserStr, toUserStr)

	inlineSend.Message = fmt.Sprintf("%s", fmt.Sprintf(inlineSendUpdateMessageAccept, amount, fromUserStrMd, toUserStrMd))
	memo := inlineSend.Memo
	if len(memo) > 0 {
		inlineSend.Message = inlineSend.Message + fmt.Sprintf(inlineSendAppendMemo, memo)
	}

	if !bot.UserInitializedWallet(to) {
		inlineSend.Message += "\n\n" + fmt.Sprintf(inlineSendCreateWalletMessage, GetUserStrMd(bot.telegram.Me))
	}

	bot.tryEditMessage(c.Message, inlineSend.Message, &tb.ReplyMarkup{})
	// notify users
	_, err = bot.telegram.Send(to, fmt.Sprintf(sendReceivedMessage, fromUserStrMd, amount))
	_, err = bot.telegram.Send(from, fmt.Sprintf(tipSentMessage, amount, toUserStrMd))
	if err != nil {
		errmsg := fmt.Errorf("[sendInline] Error: Send message to %s: %s", toUserStr, err)
		log.Errorln(errmsg)
		return
	}

	// edit persistent object and store it
	inlineSend.To = to
	// complete this transaction
	inlineSend.InTransaction = false
	runtime.IgnoreError(bot.bunt.Set(inlineSend))

}

func (bot *TipBot) cancelInlineSendHandler(c *tb.Callback) {
	inlineSend, err := bot.getInlineSend(c)
	if err != nil {
		log.Errorf("[cancelInlineSendHandler] %s", err)
		return
	}
	if c.Sender.ID == inlineSend.From.ID {
		bot.tryEditMessage(c.Message, sendCancelledMessage, &tb.ReplyMarkup{})
		// set the inlineSend inactive
		inlineSend.Active = false
		inlineSend.InTransaction = false
		runtime.IgnoreError(bot.bunt.Set(inlineSend))
	}
	return
}
