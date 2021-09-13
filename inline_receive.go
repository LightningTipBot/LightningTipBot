package main

import (
	"fmt"
	"strings"

	"github.com/LightningTipBot/LightningTipBot/internal/runtime"
	log "github.com/sirupsen/logrus"
	tb "gopkg.in/tucnak/telebot.v2"
)

const (
	inlineReceiveMessage             = "Press ✅ to pay to %s.\n\n💸 Amount: %d sat"
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
	btnAcceptInlineReceive         = inlineReceiveMenu.Data("✅ Pay", "confirm_receive_inline")
)

type InlineReceive struct {
	Message string   `json:"inline_receive_message"`
	Amount  int      `json:"inline_receive_amount"`
	From    *tb.User `json:"inline_receive_from"`
	To      *tb.User `json:"inline_receive_to"`
	Memo    string
	ID      string `json:"inline_receive_id"`
	Active  bool   `json:"inline_receive_active"`
}

func NewInlineReceive(m string) *InlineReceive {
	inlineReceive := &InlineReceive{
		Message: m,
	}
	return inlineReceive

}

func (msg InlineReceive) Key() string {
	return msg.ID
}

// tipTooltipExists checks if this tip is already known
func (bot *TipBot) getInlineReceive(c *tb.Callback) (*InlineReceive, error) {
	inlineReceive := NewInlineReceive("")
	inlineReceive.ID = c.Data
	err := bot.bunt.Get(inlineReceive)
	if err != nil {
		return nil, fmt.Errorf("could not get inline receive message")
	}

	return inlineReceive, nil

}

func (bot *TipBot) inactivateInlineReceive(c *tb.Callback) error {
	inlineReceive, err := bot.getInlineReceive(c)
	if err != nil {
		log.Errorf("[inactivateInlineReceive] %s", err)
		return err
	}
	// immediatelly set inactive to avoid double-sends
	inlineReceive.Active = false
	runtime.IgnoreError(bot.bunt.Set(inlineReceive))
	return nil
}

func (bot TipBot) handleInlineReceiveQuery(q *tb.Query) {
	amount, err := decodeAmountFromCommand(q.Text)
	if err != nil {
		bot.inlineQueryReplyWithError(q, inlineQueryReceiveTitle, fmt.Sprintf(inlineQueryReceiveDescription, bot.telegram.Me.Username))
		return
	}
	if amount < 1 {
		bot.inlineQueryReplyWithError(q, inlineSendInvalidAmountMessage, fmt.Sprintf(inlineQueryReceiveDescription, bot.telegram.Me.Username))
		return
	}

	fromUserStr := GetUserStr(&q.From)

	// check for memo in command
	memo := ""
	if len(strings.Split(q.Text, " ")) > 2 {
		memo = strings.SplitN(q.Text, " ", 3)[2]
		memoMaxLen := 159
		if len(memo) > memoMaxLen {
			memo = memo[:memoMaxLen]
		}
	}

	urls := []string{
		queryImage,
	}
	results := make(tb.Results, len(urls)) // []tb.Result
	for i, url := range urls {

		inlineMessage := fmt.Sprintf(inlineReceiveMessage, fromUserStr, amount)

		if len(memo) > 0 {
			inlineMessage = inlineMessage + fmt.Sprintf(inlineReceiveAppendMemo, memo)
		}

		result := &tb.ArticleResult{
			// URL:         url,
			Text:        inlineMessage,
			Title:       fmt.Sprintf(inlineResultReceiveTitle, amount),
			Description: fmt.Sprintf(inlineResultReceiveDescription, amount),
			// required for photos
			ThumbURL: url,
		}
		id := fmt.Sprintf("inl-receive-%d-%d-%s", q.From.ID, amount, RandStringRunes(5))
		btnAcceptInlineReceive.Data = id
		btnCancelInlineReceive.Data = id
		inlineReceiveMenu.Inline(inlineReceiveMenu.Row(btnAcceptInlineReceive, btnCancelInlineReceive))
		result.ReplyMarkup = &tb.InlineKeyboardMarkup{InlineKeyboard: inlineReceiveMenu.InlineKeyboard}

		results[i] = result

		// needed to set a unique string ID for each result
		results[i].SetResultID(id)

		// create persistend inline send struct
		inlineReceive := NewInlineReceive(inlineMessage)
		// add data to persistent object
		inlineReceive.ID = id
		inlineReceive.To = &q.From // The user who wants to receive
		// add result to persistent struct
		inlineReceive.Amount = amount
		inlineReceive.Memo = memo
		inlineReceive.Active = true
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

	if err != nil {
		log.Errorf("[acceptInlineReceiveHandler] %s", err)
		return
	}
	if !inlineReceive.Active {
		log.Errorf("[acceptInlineReceiveHandler] inline receive not active anymore")
		return
	}
	amount := inlineReceive.Amount

	// user `from` is the one who is SENDING
	// user `to` is the one who is RECEIVING
	from := c.Sender
	to := inlineReceive.To
	toUserStrMd := GetUserStrMd(to)
	fromUserStrMd := GetUserStrMd(from)
	toUserStr := GetUserStr(to)
	fromUserStr := GetUserStr(from)

	// if from.ID == to.ID {
	// 	bot.trySendMessage(from, sendYourselfMessage)
	// 	return
	// }

	// balance check of the user
	balance, err := bot.GetUserBalance(to)
	if err != nil {
		errmsg := fmt.Sprintf("could not get balance of user %s", toUserStrMd)
		log.Errorln(errmsg)
		return
	}
	// check if fromUser has balance
	if balance < amount {
		log.Errorln("[acceptInlineReceiveHandler] balance of user %s too low", toUserStrMd)
		bot.trySendMessage(from, fmt.Sprintf(inlineSendBalanceLowMessage, balance))
		return
	}

	// set inactive to avoid double-sends
	bot.inactivateInlineReceive(c)

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
		errMsg := fmt.Sprintf("[acceptInlineReceiveHandler] Transaction failed: %s", err)
		log.Errorln(errMsg)
		bot.tryEditMessage(c.Message, inlineReceiveFailedMessage, &tb.ReplyMarkup{})
		return
	}

	log.Infof("[acceptInlineReceiveHandler] %d sat from %s to %s", amount, fromUserStr, toUserStr)

	inlineReceive.Message = fmt.Sprintf("%s", fmt.Sprintf(inlineSendUpdateMessageAccept, amount, fromUserStrMd, toUserStrMd))
	memo := inlineReceive.Memo
	if len(memo) > 0 {
		inlineReceive.Message = inlineReceive.Message + fmt.Sprintf(inlineReceiveAppendMemo, memo)
	}

	if !bot.UserInitializedWallet(to) {
		inlineReceive.Message += "\n\n" + fmt.Sprintf(inlineSendCreateWalletMessage, GetUserStrMd(bot.telegram.Me))
	}

	bot.tryEditMessage(c.Message, inlineReceive.Message, &tb.ReplyMarkup{})
	// notify users
	_, err = bot.telegram.Send(to, fmt.Sprintf(sendReceivedMessage, fromUserStrMd, amount))
	_, err = bot.telegram.Send(from, fmt.Sprintf(tipSentMessage, amount, toUserStrMd))
	if err != nil {
		errmsg := fmt.Errorf("[acceptInlineReceiveHandler] Error: Receive message to %s: %s", toUserStr, err)
		log.Errorln(errmsg)
		return
	}

	// edit persistent object and store it
	inlineReceive.To = to
	runtime.IgnoreError(bot.bunt.Set(inlineReceive))

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
		runtime.IgnoreError(bot.bunt.Set(inlineReceive))
	}
	return
}
