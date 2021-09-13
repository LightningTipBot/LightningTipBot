package main

import (
	"fmt"
	"strconv"
	"time"

	"github.com/LightningTipBot/LightningTipBot/internal/runtime"
	log "github.com/sirupsen/logrus"
	tb "gopkg.in/tucnak/telebot.v2"
)

const (
	inlineFaucetMessage                     = "Press ✅ to collect from this faucet.\n\n🏅 Remaining: %d/%d sat (given to %d users)"
	inlineFaucetEndedMessage                = "🏅 Faucet empty 🏅\n\n💸 %d sat given to %d users."
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
	Memo            string     `json:"inline_faucet_memo"`
	ID              string     `json:"inline_faucet_id"`
	Active          bool       `json:"inline_faucet_active"`
	NTotal          int        `json:"inline_faucet_ntotal"`
	NTaken          int        `json:"inline_faucet_ntaken"`
	UserNeedsWallet bool       `json:"inline_faucet_userneedswallet"`
	InTransaction   bool       `json:"inline_faucet_intransaction"`
}

func NewInlineFaucet() *InlineFaucet {
	inlineFaucet := &InlineFaucet{
		Message:         "",
		NTaken:          0,
		UserNeedsWallet: false,
		InTransaction:   false,
		Active:          true,
	}
	return inlineFaucet

}

func (msg InlineFaucet) Key() string {
	return msg.ID
}

func (bot *TipBot) LockFaucet(tx *InlineFaucet) error {
	// immediatelly set intransaction to block duplicate calls
	tx.InTransaction = true
	err := bot.bunt.Set(tx)
	if err != nil {
		return err
	}
	return nil
}

func (bot *TipBot) ReleaseFaucet(tx *InlineFaucet) error {
	// immediatelly set intransaction to block duplicate calls
	tx.InTransaction = false
	err := bot.bunt.Set(tx)
	if err != nil {
		return err
	}
	return nil
}

func (bot *TipBot) inactivateFaucet(tx *InlineFaucet) error {
	tx.Active = false
	err := bot.bunt.Set(tx)
	if err != nil {
		return err
	}
	return nil
}

// tipTooltipExists checks if this tip is already known
func (bot *TipBot) getInlineFaucet(c *tb.Callback) (*InlineFaucet, error) {
	inlineFaucet := NewInlineFaucet()
	inlineFaucet.ID = c.Data
	err := bot.bunt.Get(inlineFaucet)

	// to avoid race conditions, we block the call if there is
	// already an active transaction by loop until InTransaction is false
	ticker := time.NewTicker(time.Second * 10)

	for inlineFaucet.InTransaction {
		select {
		case <-ticker.C:
			return inlineFaucet, fmt.Errorf("inline send timeout")
		default:
			log.Infoln("in transaction")
			time.Sleep(time.Duration(500) * time.Millisecond)
			err = bot.bunt.Get(inlineFaucet)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("could not get inline faucet: %s", err)
	}
	return inlineFaucet, nil

}

func (bot TipBot) faucetHandler(m *tb.Message) {
	inlineFaucet := NewInlineFaucet()
	var err error
	inlineFaucet.Amount, err = decodeAmountFromCommand(m.Text)
	if err != nil {
		return
	}
	peruserStr, err := getArgumentFromCommand(m.Text, 2)
	if err != nil {
		return
	}
	inlineFaucet.PerUserAmount, err = strconv.Atoi(peruserStr)
	if err != nil {
		return
	}
	// peruser amount must be >1 and a divisor of amount
	if inlineFaucet.PerUserAmount < 1 || inlineFaucet.Amount%inlineFaucet.PerUserAmount != 0 {
		return
	}
	inlineFaucet.NTotal = inlineFaucet.Amount / inlineFaucet.PerUserAmount

	fromUserStr := GetUserStr(m.Sender)
	balance, err := bot.GetUserBalance(m.Sender)
	if err != nil {
		errmsg := fmt.Sprintf("could not get balance of user %s", fromUserStr)
		log.Errorln(errmsg)
		return
	}
	// check if fromUser has balance
	if balance < inlineFaucet.Amount {
		log.Errorln("Balance of user %s too low", fromUserStr)
		return
	}

	// // check for memo in command
	memo := GetMemoFromCommand(m.Text, 3)

	inlineMessage := fmt.Sprintf(inlineFaucetMessage, inlineFaucet.Amount, inlineFaucet.Amount, 0)
	if len(memo) > 0 {
		inlineMessage = inlineMessage + fmt.Sprintf(inlineFaucetAppendMemo, memo)
	}

	inlineFaucet.ID = fmt.Sprintf("inl-faucet-%d-%d-%s", m.Sender.ID, inlineFaucet.Amount, RandStringRunes(5))

	btnAcceptInlineFaucet.Data = inlineFaucet.ID
	btnCancelInlineFaucet.Data = inlineFaucet.ID
	inlineFaucetMenu.Inline(inlineFaucetMenu.Row(btnAcceptInlineFaucet, btnCancelInlineFaucet))
	bot.trySendMessage(m.Chat, inlineMessage, inlineFaucetMenu)

	inlineFaucet.Message = inlineMessage
	inlineFaucet.From = m.Sender
	inlineFaucet.Memo = memo
	inlineFaucet.RemainingAmount = inlineFaucet.Amount
	runtime.IgnoreError(bot.bunt.Set(inlineFaucet))

}

func (bot TipBot) handleInlineFaucetQuery(q *tb.Query) {
	inlineFaucet := NewInlineFaucet()
	var err error
	inlineFaucet.Amount, err = decodeAmountFromCommand(q.Text)
	if err != nil {
		bot.inlineQueryReplyWithError(q, inlineQueryFaucetTitle, fmt.Sprintf(inlineQueryFaucetDescription, bot.telegram.Me.Username))
		return
	}
	if inlineFaucet.Amount < 1 {
		bot.inlineQueryReplyWithError(q, inlineSendInvalidAmountMessage, fmt.Sprintf(inlineQueryFaucetDescription, bot.telegram.Me.Username))
		return
	}

	peruserStr, err := getArgumentFromCommand(q.Text, 2)
	if err != nil {
		return
	}
	inlineFaucet.PerUserAmount, err = strconv.Atoi(peruserStr)
	if err != nil {
		bot.inlineQueryReplyWithError(q, inlineQuerySendTitle, fmt.Sprintf(inlineQueryFaucetDescription, bot.telegram.Me.Username))
		return
	}
	// peruser amount must be >1 and a divisor of amount
	if inlineFaucet.PerUserAmount < 1 || inlineFaucet.Amount%inlineFaucet.PerUserAmount != 0 {
		bot.inlineQueryReplyWithError(q, inlineFaucetInvalidPeruserAmountMessage, fmt.Sprintf(inlineQueryFaucetDescription, bot.telegram.Me.Username))
		return
	}
	inlineFaucet.NTotal = inlineFaucet.Amount / inlineFaucet.PerUserAmount

	fromUserStr := GetUserStr(&q.From)
	balance, err := bot.GetUserBalance(&q.From)
	if err != nil {
		errmsg := fmt.Sprintf("could not get balance of user %s", fromUserStr)
		log.Errorln(errmsg)
		return
	}
	// check if fromUser has balance
	if balance < inlineFaucet.Amount {
		log.Errorln("Balance of user %s too low", fromUserStr)
		bot.inlineQueryReplyWithError(q, fmt.Sprintf(inlineSendBalanceLowMessage, balance), fmt.Sprintf(inlineQueryFaucetDescription, bot.telegram.Me.Username))
		return
	}

	// check for memo in command
	memo := GetMemoFromCommand(q.Text, 3)

	urls := []string{
		queryImage,
	}
	results := make(tb.Results, len(urls)) // []tb.Result
	for i, url := range urls {
		inlineMessage := fmt.Sprintf(inlineFaucetMessage, inlineFaucet.Amount, inlineFaucet.Amount, 0)
		if len(memo) > 0 {
			inlineMessage = inlineMessage + fmt.Sprintf(inlineFaucetAppendMemo, memo)
		}
		result := &tb.ArticleResult{
			// URL:         url,
			Text:        inlineMessage,
			Title:       fmt.Sprintf(inlineResultFaucetTitle, inlineFaucet.Amount),
			Description: fmt.Sprintf(inlineResultFaucetDescription, inlineFaucet.Amount),
			// required for photos
			ThumbURL: url,
		}
		id := fmt.Sprintf("inl-faucet-%d-%d-%s", q.From.ID, inlineFaucet.Amount, RandStringRunes(5))
		btnAcceptInlineFaucet.Data = id
		btnCancelInlineFaucet.Data = id
		inlineFaucetMenu.Inline(inlineFaucetMenu.Row(btnAcceptInlineFaucet, btnCancelInlineFaucet))
		result.ReplyMarkup = &tb.InlineKeyboardMarkup{InlineKeyboard: inlineFaucetMenu.InlineKeyboard}
		results[i] = result

		// needed to set a unique string ID for each result
		results[i].SetResultID(id)

		// create persistend inline send struct
		inlineFaucet.Message = inlineMessage
		inlineFaucet.ID = id
		inlineFaucet.From = &q.From
		inlineFaucet.RemainingAmount = inlineFaucet.Amount
		inlineFaucet.Memo = memo

		// inlineFaucet.Active = true
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
		log.Errorf("[accpetInlineFaucetHandler] %s", err)
		return
	}
	err = bot.LockFaucet(inlineFaucet)
	if err != nil {
		log.Errorf("[accpetInlineFaucetHandler] %s", err)
		return
	}
	if !inlineFaucet.Active {
		log.Errorf("[accpetInlineFaucetHandler] inline send not active anymore")
		return
	}
	to := c.Sender
	from := inlineFaucet.From

	if from.ID == to.ID {
		bot.trySendMessage(from, sendYourselfMessage)
		bot.ReleaseFaucet(inlineFaucet)
		return
	}
	// check if to user has already taken from the faucet
	for _, a := range inlineFaucet.To {
		if a.ID == to.ID {
			bot.ReleaseFaucet(inlineFaucet)
			// to user is already in To slice, has taken from facuet
			return
		}
	}

	if inlineFaucet.RemainingAmount >= inlineFaucet.PerUserAmount {
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
				bot.ReleaseFaucet(inlineFaucet)
				return
			}
		}

		if !bot.UserInitializedWallet(to) {
			inlineFaucet.UserNeedsWallet = true
		}

		// todo: user new get username function to get userStrings
		transactionMemo := fmt.Sprintf("Faucet from %s to %s (%d sat).", fromUserStr, toUserStr, inlineFaucet.PerUserAmount)
		t := NewTransaction(bot, from, to, inlineFaucet.PerUserAmount, TransactionType("inline faucet"))
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
			bot.ReleaseFaucet(inlineFaucet)
			return
		}

		log.Infof("[sendInline] %d sat from %s to %s", inlineFaucet.PerUserAmount, fromUserStr, toUserStr)
		inlineFaucet.NTaken += 1
		inlineFaucet.To = append(inlineFaucet.To, to)
		inlineFaucet.RemainingAmount = inlineFaucet.RemainingAmount - inlineFaucet.PerUserAmount

		// notify users
		_, err = bot.telegram.Send(to, fmt.Sprintf(sendReceivedMessage, fromUserStrMd, inlineFaucet.PerUserAmount))
		_, err = bot.telegram.Send(from, fmt.Sprintf(tipSentMessage, inlineFaucet.PerUserAmount, toUserStrMd))
		if err != nil {
			errmsg := fmt.Errorf("[sendInline] Error: Send message to %s: %s", toUserStr, err)
			log.Errorln(errmsg)
			bot.ReleaseFaucet(inlineFaucet)
			return
		}

		// build faucet message
		inlineFaucet.Message = fmt.Sprintf(inlineFaucetMessage, inlineFaucet.RemainingAmount, inlineFaucet.Amount, inlineFaucet.NTaken)
		memo := inlineFaucet.Memo
		if len(memo) > 0 {
			inlineFaucet.Message = inlineFaucet.Message + fmt.Sprintf(inlineFaucetAppendMemo, memo)
		}
		if inlineFaucet.UserNeedsWallet {
			inlineFaucet.Message += "\n\n" + fmt.Sprintf(inlineFaucetCreateWalletMessage, GetUserStrMd(bot.telegram.Me))
		}

		// register new inline buttons
		inlineFaucetMenu = &tb.ReplyMarkup{ResizeReplyKeyboard: true}
		btnCancelInlineFaucet.Data = inlineFaucet.ID
		btnAcceptInlineFaucet.Data = inlineFaucet.ID
		inlineFaucetMenu.Inline(inlineFaucetMenu.Row(btnAcceptInlineFaucet, btnCancelInlineFaucet))
		// update message
		log.Infoln(inlineFaucet.Message)
		bot.tryEditMessage(c.Message, inlineFaucet.Message, inlineFaucetMenu)
	}
	if inlineFaucet.RemainingAmount < inlineFaucet.PerUserAmount {
		// faucet is depleted
		inlineFaucet.Message = fmt.Sprintf(inlineFaucetEndedMessage, inlineFaucet.Amount, inlineFaucet.NTaken)
		if inlineFaucet.UserNeedsWallet {
			inlineFaucet.Message += "\n\n" + fmt.Sprintf(inlineFaucetCreateWalletMessage, GetUserStrMd(bot.telegram.Me))
		}
		bot.tryEditMessage(c.Message, inlineFaucet.Message)
		inlineFaucet.Active = false
	}
	// edit persistent object and store it
	inlineFaucet.InTransaction = false
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
		inlineFaucet.InTransaction = false
		runtime.IgnoreError(bot.bunt.Set(inlineFaucet))
	}
	return
}
