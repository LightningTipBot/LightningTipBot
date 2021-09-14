package main

import (
	"fmt"
	"time"

	"github.com/LightningTipBot/LightningTipBot/internal/runtime"
	log "github.com/sirupsen/logrus"
	tb "gopkg.in/tucnak/telebot.v2"
)

const (
	inlineReceiveMessage             = "Press 💸 to pay to %s.\n\n💸 Amount: %d sat"
	inlineReceiveAppendMemo          = "\n✉️ %s"
	inlineReceiveUpdateMessageAccept = "💸 %d sat sent from %s to %s."
	inlineReceiveCreateWalletMessage = "Chat with %s 👈 to manage your wallet."
	inlineReceiveYourselfMessage     = "📖 You can't pay to yourself."
	inlineReceiveFailedMessage       = "🚫 Receive failed."
)

var (
	inlineQueryReceiveTitle        = "🏅 Request a payment in a chat."
	inlineQueryReceiveDescription  = "Usage: @%s receive <amount> [<memo>]"
	inlineResultReceiveTitle       = "🏅 Receive %d sat."
	inlineResultReceiveDescription = "👉 Click to request a payment of %d sat."
	inlineReceiveMenu              = &tb.ReplyMarkup{ResizeReplyKeyboard: true}
	btnCancelInlineReceive         = inlineReceiveMenu.Data("🚫 Cancel", "cancel_receive_inline")
	btnAcceptInlineReceive         = inlineReceiveMenu.Data("💸 Pay", "confirm_receive_inline")
)

type InlineReceive struct {
	Message       string   `json:"inline_receive_message"`
	Amount        int      `json:"inline_receive_amount"`
	From          *tb.User `json:"inline_receive_from"`
	To            *tb.User `json:"inline_receive_to"`
	Memo          string
	ID            string `json:"inline_receive_id"`
	Active        bool   `json:"inline_receive_active"`
	InTransaction bool   `json:"inline_receive_intransaction"`
}

func NewInlineReceive() *InlineReceive {
	inlineReceive := &InlineReceive{
		Message:       "",
		Active:        true,
		InTransaction: false,
	}
	return inlineReceive

}

func (msg InlineReceive) Key() string {
	return msg.ID
}

func (bot *TipBot) LockReceive(tx *InlineReceive) error {
	// immediatelly set intransaction to block duplicate calls
	tx.InTransaction = true
	err := bot.bunt.Set(tx)
	if err != nil {
		return err
	}
	return nil
}

func (bot *TipBot) ReleaseReceive(tx *InlineReceive) error {
	// immediatelly set intransaction to block duplicate calls
	tx.InTransaction = false
	err := bot.bunt.Set(tx)
	if err != nil {
		return err
	}
	return nil
}

func (bot *TipBot) inactivateReceive(tx *InlineReceive) error {
	tx.Active = false
	err := bot.bunt.Set(tx)
	if err != nil {
		return err
	}
	return nil
}

// tipTooltipExists checks if this tip is already known
func (bot *TipBot) getInlineReceive(c *tb.Callback) (*InlineReceive, error) {
	inlineReceive := NewInlineReceive()
	inlineReceive.ID = c.Data
	err := bot.bunt.Get(inlineReceive)
	// to avoid race conditions, we block the call if there is
	// already an active transaction by loop until InTransaction is false
	ticker := time.NewTicker(time.Second * 10)

	for inlineReceive.InTransaction {
		select {
		case <-ticker.C:
			return nil, fmt.Errorf("inline send timeout")
		default:
			log.Infoln("in transaction")
			time.Sleep(time.Duration(500) * time.Millisecond)
			err = bot.bunt.Get(inlineReceive)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("could not get inline receive message")
	}
	return inlineReceive, nil

}

func (bot TipBot) handleInlineReceiveQuery(q *tb.Query) {
	inlineReceive := NewInlineReceive()
	var err error
	inlineReceive.Amount, err = decodeAmountFromCommand(q.Text)
	if err != nil {
		bot.inlineQueryReplyWithError(q, inlineQueryReceiveTitle, fmt.Sprintf(inlineQueryReceiveDescription, bot.telegram.Me.Username))
		return
	}
	if inlineReceive.Amount < 1 {
		bot.inlineQueryReplyWithError(q, inlineSendInvalidAmountMessage, fmt.Sprintf(inlineQueryReceiveDescription, bot.telegram.Me.Username))
		return
	}

	fromUserStr := GetUserStr(&q.From)

	// check for memo in command
	inlineReceive.Memo = GetMemoFromCommand(q.Text, 2)

	urls := []string{
		queryImage,
	}
	results := make(tb.Results, len(urls)) // []tb.Result
	for i, url := range urls {

		inlineMessage := fmt.Sprintf(inlineReceiveMessage, fromUserStr, inlineReceive.Amount)

		if len(inlineReceive.Memo) > 0 {
			inlineMessage = inlineMessage + fmt.Sprintf(inlineReceiveAppendMemo, inlineReceive.Memo)
		}

		result := &tb.ArticleResult{
			// URL:         url,
			Text:        inlineMessage,
			Title:       fmt.Sprintf(inlineResultReceiveTitle, inlineReceive.Amount),
			Description: fmt.Sprintf(inlineResultReceiveDescription, inlineReceive.Amount),
			// required for photos
			ThumbURL: url,
		}
		id := fmt.Sprintf("inl-receive-%d-%d-%s", q.From.ID, inlineReceive.Amount, RandStringRunes(5))
		btnAcceptInlineReceive.Data = id
		btnCancelInlineReceive.Data = id
		inlineReceiveMenu.Inline(inlineReceiveMenu.Row(btnAcceptInlineReceive, btnCancelInlineReceive))
		result.ReplyMarkup = &tb.InlineKeyboardMarkup{InlineKeyboard: inlineReceiveMenu.InlineKeyboard}

		results[i] = result

		// needed to set a unique string ID for each result
		results[i].SetResultID(id)

		// create persistend inline send struct
		// add data to persistent object
		inlineReceive.ID = id
		inlineReceive.To = &q.From // The user who wants to receive
		// add result to persistent struct
		inlineReceive.Message = inlineMessage
		runtime.IgnoreError(bot.bunt.Set(inlineReceive))
	}

	err = bot.telegram.Answer(q, &tb.QueryResponse{
		Results:   results,
		CacheTime: 1, // 60 == 1 minute, todo: make higher than 1 s in production

	})

	if err != nil {
		log.Errorln(err)
	}
}

func (bot *TipBot) acceptInlineReceiveHandler(c *tb.Callback) {
	inlineReceive, err := bot.getInlineReceive(c)
	// immediatelly set intransaction to block duplicate calls
	if err != nil {
		log.Errorf("[getInlineReceive] %s", err)
		return
	}
	err = bot.LockReceive(inlineReceive)
	if err != nil {
		log.Errorf("[acceptInlineReceiveHandler] %s", err)
		return
	}

	if !inlineReceive.Active {
		log.Errorf("[acceptInlineReceiveHandler] inline receive not active anymore")
		return
	}

	defer bot.ReleaseReceive(inlineReceive)

	// user `from` is the one who is SENDING
	// user `to` is the one who is RECEIVING
	from := c.Sender
	to := inlineReceive.To
	toUserStrMd := GetUserStrMd(to)
	fromUserStrMd := GetUserStrMd(from)
	toUserStr := GetUserStr(to)
	fromUserStr := GetUserStr(from)

	if from.ID == to.ID {
		bot.trySendMessage(from, sendYourselfMessage)
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
		log.Errorln("[acceptInlineReceiveHandler] balance of user %s too low", fromUserStr)
		bot.trySendMessage(from, fmt.Sprintf(inlineSendBalanceLowMessage, balance))
		return
	}

	// set inactive to avoid double-sends
	bot.inactivateReceive(inlineReceive)

	// todo: user new get username function to get userStrings
	transactionMemo := fmt.Sprintf("Send from %s to %s (%d sat).", fromUserStr, toUserStr, inlineReceive.Amount)
	t := NewTransaction(bot, from, to, inlineReceive.Amount, TransactionType("inline send"))
	t.Memo = transactionMemo
	success, err := t.Send()
	if !success {
		if err != nil {
			bot.trySendMessage(from, fmt.Sprintf(tipErrorMessage, err))
		} else {
			bot.trySendMessage(from, fmt.Sprintf(tipErrorMessage, tipUndefinedErrorMsg))
		}
		errMsg := fmt.Sprintf("[acceptInlineReceiveHandler] Transaction failed: %s", err)
		log.Errorln(errMsg)
		bot.tryEditMessage(c.Message, inlineReceiveFailedMessage, &tb.ReplyMarkup{})
		return
	}

	log.Infof("[acceptInlineReceiveHandler] %d sat from %s to %s", inlineReceive.Amount, fromUserStr, toUserStr)

	inlineReceive.Message = fmt.Sprintf("%s", fmt.Sprintf(inlineSendUpdateMessageAccept, inlineReceive.Amount, fromUserStrMd, toUserStrMd))
	memo := inlineReceive.Memo
	if len(memo) > 0 {
		inlineReceive.Message = inlineReceive.Message + fmt.Sprintf(inlineReceiveAppendMemo, memo)
	}

	if !bot.UserInitializedWallet(to) {
		inlineReceive.Message += "\n\n" + fmt.Sprintf(inlineSendCreateWalletMessage, GetUserStrMd(bot.telegram.Me))
	}

	bot.tryEditMessage(c.Message, inlineReceive.Message, &tb.ReplyMarkup{})
	// notify users
	_, err = bot.telegram.Send(to, fmt.Sprintf(sendReceivedMessage, fromUserStrMd, inlineReceive.Amount))
	_, err = bot.telegram.Send(from, fmt.Sprintf(tipSentMessage, inlineReceive.Amount, toUserStrMd))
	if err != nil {
		errmsg := fmt.Errorf("[acceptInlineReceiveHandler] Error: Receive message to %s: %s", toUserStr, err)
		log.Errorln(errmsg)
		return
	}
}

func (bot *TipBot) cancelInlineReceiveHandler(c *tb.Callback) {
	inlineReceive, err := bot.getInlineReceive(c)
	if err != nil {
		log.Errorf("[cancelInlineReceiveHandler] %s", err)
		return
	}
	if c.Sender.ID == inlineReceive.To.ID {
		bot.tryEditMessage(c.Message, sendCancelledMessage, &tb.ReplyMarkup{})
		// set the inlineReceive inactive
		inlineReceive.Active = false
		inlineReceive.InTransaction = false
		runtime.IgnoreError(bot.bunt.Set(inlineReceive))
	}
	return
}
