package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/LightningTipBot/LightningTipBot/internal/runtime"
	log "github.com/sirupsen/logrus"
	tb "gopkg.in/tucnak/telebot.v2"
)

const (
	inlineFaucetMessage                     = "Press ✅ to collect.\n\n🏅 Balance: %d/%d sat (%d users)"
	inlineFaucetAppendMemo                  = "\n✉️ %s"
	inlineFaucetUpdateMessageAccept         = "💸 %d sat sent from %s to %s."
	inlineFaucetCreateWalletMessage         = "Chat with %s 👈 to manage your wallet."
	inlineFaucetYourselfMessage             = "📖 You can't pay to yourself."
	inlineFaucetFailedMessage               = "🚫 Send failed."
	inlineFaucetCancelledMessage            = "🚫 Faucet cancelled."
	inlineFaucetInvalidPeruserAmountMessage = "🚫 Peruser amount not divisor of capacity."
)

var (
	inlineQueryFaucetTitle        = "🚰 Create a faucet."
	inlineQueryFaucetDescription  = "Usage: @%s faucet <capacity> <per_user>"
	inlineResultFaucetTitle       = "💸 Create a %d sat faucet."
	inlineResultFaucetDescription = "👉 Click here to create a faucet worth %d sat in this chat."
	inlineFaucetMenu              = &tb.ReplyMarkup{ResizeReplyKeyboard: true}
	btnCancelInlineFaucet         = inlineFaucetMenu.Data("🚫 Cancel", "cancel_faucet_inline")
	btnAcceptInlineFaucet         = inlineFaucetMenu.Data("✅ Receive", "confirm_faucet_inline")
)

type InlineFaucet struct {
	Message         string     `json:"inline_faucet_message"`
	Amount          int        `json:"inline_faucet_amount"`
	RemainingAmount int        `json:"inline_faucet_remainingamount"`
	PerUserAmount   int        `json:"inline_faucet_peruseramount"`
	From            *tb.User   `json:"inline_faucet_from"`
	To              []*tb.User `json:"inline_faucet_to"`
	Memo            string
	ID              string `json:"inline_faucet_id"`
	Active          bool   `json:"inline_faucet_active"`
	NTotal          int    `json:"inline_faucet_ntotal"`
	NTaken          int    `json:"inline_faucet_ntaken"`
}

func NewInlineFaucet(m string) *InlineFaucet {
	inlineFaucet := &InlineFaucet{
		Message: m,
		NTaken:  0,
	}
	return inlineFaucet

}

func (msg InlineFaucet) Key() string {
	return msg.ID
}

// tipTooltipExists checks if this tip is already known
func (bot *TipBot) getInlineFaucet(c *tb.Callback) (*InlineFaucet, error) {
	inlineFaucet := NewInlineFaucet("")
	inlineFaucet.ID = c.Data
	err := bot.bunt.Get(inlineFaucet)
	if err != nil {
		return nil, fmt.Errorf("could not get inline faucet")
	}
	return inlineFaucet, nil

}

func (bot TipBot) handleInlineFaucetQuery(q *tb.Query) {
	amount, err := decodeAmountFromCommand(q.Text)
	if err != nil {
		bot.inlineQueryReplyWithError(q, inlineQueryFaucetTitle, fmt.Sprintf(inlineQueryFaucetDescription, bot.telegram.Me.Username))
		return
	}
	if amount < 1 {
		bot.inlineQueryReplyWithError(q, inlineSendInvalidAmountMessage, fmt.Sprintf(inlineQueryFaucetDescription, bot.telegram.Me.Username))
		return
	}

	peruserStr, err := getArgumentFromCommand(q.Text, 2)
	if err != nil {
		return
	}
	peruser, err := strconv.Atoi(peruserStr)
	if err != nil {
		bot.inlineQueryReplyWithError(q, inlineQuerySendTitle, fmt.Sprintf(inlineQueryFaucetDescription, bot.telegram.Me.Username))
		return
	}
	// peruser amount must be >1 and a divisor of amount
	if peruser < 1 || amount%peruser != 0 {
		bot.inlineQueryReplyWithError(q, inlineFaucetInvalidPeruserAmountMessage, fmt.Sprintf(inlineQueryFaucetDescription, bot.telegram.Me.Username))
		return
	}
	ntotal := amount / peruser

	fromUserStr := GetUserStr(&q.From)
	balance, err := bot.GetUserBalance(&q.From)
	if err != nil {
		errmsg := fmt.Sprintf("could not get balance of user %s", fromUserStr)
		log.Errorln(errmsg)
		return
	}
	// check if fromUser has balance
	if balance < amount {
		log.Errorln("Balance of user %s too low", fromUserStr)
		bot.inlineQueryReplyWithError(q, fmt.Sprintf(inlineSendBalanceLowMessage, balance), fmt.Sprintf(inlineQueryFaucetDescription, bot.telegram.Me.Username))
		return
	}

	// check for memo in command
	memo := ""
	if len(strings.Split(q.Text, " ")) > 3 {
		memo = strings.SplitN(q.Text, " ", 4)[3]
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

		inlineMessage := fmt.Sprintf(inlineFaucetMessage, amount, amount, 0)

		if len(memo) > 0 {
			inlineMessage = inlineMessage + fmt.Sprintf(inlineFaucetAppendMemo, memo)
		}

		result := &tb.ArticleResult{
			// URL:         url,
			Text:        inlineMessage,
			Title:       fmt.Sprintf(inlineResultFaucetTitle, amount),
			Description: fmt.Sprintf(inlineResultFaucetDescription, amount),
			// required for photos
			ThumbURL: url,
		}
		id := fmt.Sprintf("inl-faucet-%d-%d-%s", q.From.ID, amount, RandStringRunes(5))
		btnAcceptInlineFaucet.Data = id
		btnCancelInlineFaucet.Data = id
		inlineFaucetMenu.Inline(inlineFaucetMenu.Row(btnAcceptInlineFaucet, btnCancelInlineFaucet))
		result.ReplyMarkup = &tb.InlineKeyboardMarkup{InlineKeyboard: inlineFaucetMenu.InlineKeyboard}

		results[i] = result

		// needed to set a unique string ID for each result
		results[i].SetResultID(id)

		// create persistend inline send struct
		inlineFaucet := NewInlineFaucet(inlineMessage)
		// add data to persistent object
		inlineFaucet.ID = id
		inlineFaucet.From = &q.From
		// add result to persistent struct
		inlineFaucet.Amount = amount
		inlineFaucet.PerUserAmount = peruser
		inlineFaucet.RemainingAmount = amount
		inlineFaucet.NTotal = ntotal
		inlineFaucet.NTaken = 0

		inlineFaucet.Memo = memo
		inlineFaucet.Active = true
		runtime.IgnoreError(bot.bunt.Set(inlineFaucet))
	}

	err = bot.telegram.Answer(q, &tb.QueryResponse{
		Results:   results,
		CacheTime: 1, // 60 == 1 minute, todo: make higher than 1 s in production

	})

	if err != nil {
		log.Errorln(err)
	}
}

func (bot *TipBot) accpetInlineFaucetHandler(c *tb.Callback) {
	inlineFaucet, err := bot.getInlineFaucet(c)
	if err != nil {
		log.Errorf("[acceptInlineSendHandler] %s", err)
		return
	}
	if !inlineFaucet.Active {
		log.Errorf("[acceptInlineSendHandler] inline send not active anymore")
		return
	}
	amount := inlineFaucet.Amount
	peruser := inlineFaucet.PerUserAmount
	to := c.Sender
	from := inlineFaucet.From
	remaining := inlineFaucet.RemainingAmount
	ntaken := inlineFaucet.NTaken

	// if from.ID == to.ID {
	// 	bot.trySendMessage(from, sendYourselfMessage)
	// 	return
	// }

	// check if to user has already taken from the faucet
	for _, a := range inlineFaucet.To {
		if a.ID == to.ID {
			// to user is already in To slice, has taken from facuet
			return
		}
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

	// todo: user new get username function to get userStrings
	transactionMemo := fmt.Sprintf("Faucet from %s to %s (%d sat).", fromUserStr, toUserStr, peruser)
	t := NewTransaction(bot, from, to, peruser, TransactionType("inline faucet"))
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
		// bot.tryEditMessage(c.Message, inlineFaucetFailedMessage, &tb.ReplyMarkup{})
		return
	}

	log.Infof("[sendInline] %d sat from %s to %s", peruser, fromUserStr, toUserStr)
	ntaken += 1
	inlineFaucet.Message = fmt.Sprintf("%s", fmt.Sprintf(inlineFaucetMessage, remaining, amount, ntaken))
	memo := inlineFaucet.Memo
	if len(memo) > 0 {
		inlineFaucet.Message = inlineFaucet.Message + fmt.Sprintf(inlineFaucetAppendMemo, memo)
	}

	// if !bot.UserInitializedWallet(to) {
	// 	inlineFaucet.Message += "\n\n" + fmt.Sprintf(inlineSendCreateWalletMessage, GetUserStrMd(bot.telegram.Me))
	// }
	// btnAcceptInlineFaucet = inlineFaucetMenu.Data("✅ Receive asdasd", "confirm_faucet_inline_2")
	// btnAcceptInlineFaucet.Data = inlineFaucet.ID
	// btnCancelInlineFaucet.Data = inlineFaucet.ID
	// bot.telegram.Handle(&btnAcceptInlineFaucet, bot.accpetInlineFaucetHandler)
	// bot.telegram.Handle(&btnCancelInlineFaucet, bot.cancelInlineFaucetHandler)
	inlineFaucetMenu.Inline(inlineFaucetMenu.Row(btnAcceptInlineFaucet, btnCancelInlineFaucet))

	log.Infoln(inlineFaucet.Message)
	bot.tryEditMessage(c.Message, inlineFaucet.Message, inlineFaucetMenu)
	// notify users
	_, err = bot.telegram.Send(to, fmt.Sprintf(sendReceivedMessage, fromUserStrMd, peruser))
	_, err = bot.telegram.Send(from, fmt.Sprintf(tipSentMessage, peruser, toUserStrMd))
	if err != nil {
		errmsg := fmt.Errorf("[sendInline] Error: Send message to %s: %s", toUserStr, err)
		log.Errorln(errmsg)
		return
	}

	// edit persistent object and store it
	inlineFaucet.To = append(inlineFaucet.To, to)
	inlineFaucet.RemainingAmount = inlineFaucet.RemainingAmount - peruser
	inlineFaucet.NTaken = ntaken

	runtime.IgnoreError(bot.bunt.Set(inlineFaucet))

}

func (bot *TipBot) cancelInlineFaucetHandler(c *tb.Callback) {
	inlineFaucet, err := bot.getInlineFaucet(c)
	if err != nil {
		log.Errorf("[cancelInlineSendHandler] %s", err)
		return
	}
	if c.Sender.ID == inlineFaucet.From.ID {
		bot.tryEditMessage(c.Message, inlineFaucetCancelledMessage, &tb.ReplyMarkup{})
		// set the inlineFaucet inactive
		inlineFaucet.Active = false
		runtime.IgnoreError(bot.bunt.Set(inlineFaucet))
	}
	return
}
