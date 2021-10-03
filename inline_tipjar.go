package main

import (
	"context"
	"fmt"
	"time"

	"github.com/LightningTipBot/LightningTipBot/internal/lnbits"
	"github.com/LightningTipBot/LightningTipBot/internal/storage"

	"github.com/LightningTipBot/LightningTipBot/internal/runtime"
	log "github.com/sirupsen/logrus"
	tb "gopkg.in/tucnak/telebot.v2"
)

var (
	inlineTipjarMenu      = &tb.ReplyMarkup{ResizeReplyKeyboard: true}
	btnCancelInlineTipjar = inlineTipjarMenu.Data("🚫", "cancel_tipjar_inline")
	btnAcceptInlineTipjar = inlineTipjarMenu.Data("💸 Pay", "confirm_tipjar_inline")
)

type InlineTipjar struct {
	*storage.BaseTransaction
	Message         string         `json:"inline_tipjar_message"`
	Amount          int            `json:"inline_tipjar_amount"`
	RemainingAmount int            `json:"inline_tipjar_remainingamount"`
	PerUserAmount   int            `json:"inline_tipjar_peruseramount"`
	To              *lnbits.User   `json:"inline_tipjar_to"`
	From            []*lnbits.User `json:"inline_tipjar_from"`
	Memo            string         `json:"inline_tipjar_memo"`
	NTotal          int            `json:"inline_tipjar_ntotal"`
	NGiven          int            `json:"inline_tipjar_ngiven"`
	LanguageCode    string         `json:"languagecode"`
}

func NewInlineTipjar() *InlineTipjar {
	inlineTipjar := &InlineTipjar{
		Message: "",
		NGiven:  0,
		BaseTransaction: &storage.BaseTransaction{
			InTransaction: false,
			Active:        true,
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		},
	}
	return inlineTipjar

}

func (bot TipBot) tipjarHandler(ctx context.Context, m *tb.Message) {
	bot.anyTextHandler(ctx, m)
	if m.Private() {
		bot.trySendMessage(m.Sender, fmt.Sprintf(Translate(ctx, "inlineTipjarHelpText"), Translate(ctx, "inlineTipjarHelpTipjarInGroup")))
		return
	}
	inlineTipjar := NewInlineTipjar()
	var err error
	inlineTipjar.Amount, err = decodeAmountFromCommand(m.Text)
	if err != nil {
		bot.trySendMessage(m.Sender, fmt.Sprintf(Translate(ctx, "inlineTipjarHelpText"), Translate(ctx, "inlineTipjarInvalidAmountMessage")))
		bot.tryDeleteMessage(m)
		return
	}
	peruserStr, err := getArgumentFromCommand(m.Text, 2)
	if err != nil {
		bot.trySendMessage(m.Sender, fmt.Sprintf(Translate(ctx, "inlineTipjarHelpText"), ""))
		bot.tryDeleteMessage(m)
		return
	}
	inlineTipjar.PerUserAmount, err = getAmount(peruserStr)
	if err != nil {
		bot.trySendMessage(m.Sender, fmt.Sprintf(Translate(ctx, "inlineTipjarHelpText"), Translate(ctx, "inlineTipjarInvalidAmountMessage")))
		bot.tryDeleteMessage(m)
		return
	}
	// peruser amount must be >1 and a divisor of amount
	if inlineTipjar.PerUserAmount < 1 || inlineTipjar.Amount%inlineTipjar.PerUserAmount != 0 {
		bot.trySendMessage(m.Sender, fmt.Sprintf(Translate(ctx, "inlineTipjarHelpText"), Translate(ctx, "inlineTipjarInvalidPeruserAmountMessage")))
		bot.tryDeleteMessage(m)
		return
	}
	inlineTipjar.NTotal = inlineTipjar.Amount / inlineTipjar.PerUserAmount
	toUser := LoadUser(ctx)
	toUserStr := GetUserStr(m.Sender)

	// // check for memo in command
	memo := GetMemoFromCommand(m.Text, 3)

	inlineMessage := fmt.Sprintf(Translate(ctx, "inlineTipjarMessage"), inlineTipjar.PerUserAmount, inlineTipjar.Amount, inlineTipjar.Amount, 0, inlineTipjar.NTotal, MakeProgressbar(inlineTipjar.Amount, inlineTipjar.Amount))
	if len(memo) > 0 {
		inlineMessage = inlineMessage + fmt.Sprintf(Translate(ctx, "inlineTipjarAppendMemo"), memo)
	}

	inlineTipjar.ID = fmt.Sprintf("inl-tipjar-%d-%d-%s", m.Sender.ID, inlineTipjar.Amount, RandStringRunes(5))
	acceptInlineTipjarButton := inlineTipjarMenu.Data(Translate(ctx, "payReceiveButtonMessage"), "confirm_tipjar_inline")
	cancelInlineTipjarButton := inlineTipjarMenu.Data(Translate(ctx, "cancelButtonMessage"), "cancel_tipjar_inline")
	acceptInlineTipjarButton.Data = inlineTipjar.ID
	cancelInlineTipjarButton.Data = inlineTipjar.ID

	inlineTipjarMenu.Inline(
		inlineTipjarMenu.Row(
			acceptInlineTipjarButton,
			cancelInlineTipjarButton),
	)
	bot.trySendMessage(m.Chat, inlineMessage, inlineTipjarMenu)
	log.Infof("[tipjar] %s created tipjar %s: %d sat (%d per user)", toUserStr, inlineTipjar.ID, inlineTipjar.Amount, inlineTipjar.PerUserAmount)
	inlineTipjar.Message = inlineMessage
	inlineTipjar.To = toUser
	inlineTipjar.Memo = memo
	inlineTipjar.RemainingAmount = inlineTipjar.Amount
	inlineTipjar.LanguageCode = ctx.Value("publicLanguageCode").(string)
	runtime.IgnoreError(bot.bunt.Set(inlineTipjar))

}

func (bot TipBot) handleInlineTipjarQuery(ctx context.Context, q *tb.Query) {
	inlineTipjar := NewInlineTipjar()
	var err error
	inlineTipjar.Amount, err = decodeAmountFromCommand(q.Text)
	if err != nil {
		bot.inlineQueryReplyWithError(q, TranslateUser(ctx, "inlineQueryTipjarTitle"), fmt.Sprintf(TranslateUser(ctx, "inlineQueryTipjarDescription"), bot.telegram.Me.Username))
		return
	}
	if inlineTipjar.Amount < 1 {
		bot.inlineQueryReplyWithError(q, TranslateUser(ctx, "inlineSendInvalidAmountMessage"), fmt.Sprintf(TranslateUser(ctx, "inlineQueryTipjarDescription"), bot.telegram.Me.Username))
		return
	}

	peruserStr, err := getArgumentFromCommand(q.Text, 2)
	if err != nil {
		bot.inlineQueryReplyWithError(q, TranslateUser(ctx, "inlineQueryTipjarTitle"), fmt.Sprintf(TranslateUser(ctx, "inlineQueryTipjarDescription"), bot.telegram.Me.Username))
		return
	}
	inlineTipjar.PerUserAmount, err = getAmount(peruserStr)
	if err != nil {
		bot.inlineQueryReplyWithError(q, TranslateUser(ctx, "inlineQueryTipjarTitle"), fmt.Sprintf(TranslateUser(ctx, "inlineQueryTipjarDescription"), bot.telegram.Me.Username))
		return
	}
	// peruser amount must be >1 and a divisor of amount
	if inlineTipjar.PerUserAmount < 1 || inlineTipjar.Amount%inlineTipjar.PerUserAmount != 0 {
		bot.inlineQueryReplyWithError(q, TranslateUser(ctx, "inlineTipjarInvalidPeruserAmountMessage"), fmt.Sprintf(TranslateUser(ctx, "inlineQueryTipjarDescription"), bot.telegram.Me.Username))
		return
	}
	inlineTipjar.NTotal = inlineTipjar.Amount / inlineTipjar.PerUserAmount
	toUser := LoadUser(ctx)
	toUserStr := GetUserStr(&q.From)
	balance, err := bot.GetUserBalance(toUser)
	if err != nil {
		errmsg := fmt.Sprintf("could not get balance of user %s", toUserStr)
		log.Errorln(errmsg)
		return
	}
	// check if toUser has balance
	if balance < inlineTipjar.Amount {
		log.Errorf("Balance of user %s too low", toUserStr)
		bot.inlineQueryReplyWithError(q, fmt.Sprintf(TranslateUser(ctx, "inlineSendBalanceLowMessage"), balance), fmt.Sprintf(TranslateUser(ctx, "inlineQueryTipjarDescription"), bot.telegram.Me.Username))
		return
	}

	// check for memo in command
	memo := GetMemoFromCommand(q.Text, 3)

	urls := []string{
		queryImage,
	}
	results := make(tb.Results, len(urls)) // []tb.Result
	for i, url := range urls {
		inlineMessage := fmt.Sprintf(Translate(ctx, "inlineTipjarMessage"), inlineTipjar.PerUserAmount, inlineTipjar.Amount, inlineTipjar.Amount, 0, inlineTipjar.NTotal, MakeProgressbar(inlineTipjar.Amount, inlineTipjar.Amount))
		if len(memo) > 0 {
			inlineMessage = inlineMessage + fmt.Sprintf(Translate(ctx, "inlineTipjarAppendMemo"), memo)
		}
		result := &tb.ArticleResult{
			// URL:         url,
			Text:        inlineMessage,
			Title:       fmt.Sprintf(TranslateUser(ctx, "inlineResultTipjarTitle"), inlineTipjar.Amount),
			Description: TranslateUser(ctx, "inlineResultTipjarDescription"),
			// required for photos
			ThumbURL: url,
		}
		id := fmt.Sprintf("inl-tipjar-%d-%d-%s", q.From.ID, inlineTipjar.Amount, RandStringRunes(5))
		acceptInlineTipjarButton := inlineTipjarMenu.Data(Translate(ctx, "payReceiveButtonMessage"), "confirm_tipjar_inline")
		cancelInlineTipjarButton := inlineTipjarMenu.Data(Translate(ctx, "cancelButtonMessage"), "cancel_tipjar_inline")
		acceptInlineTipjarButton.Data = id
		cancelInlineTipjarButton.Data = id

		inlineTipjarMenu.Inline(
			inlineTipjarMenu.Row(
				acceptInlineTipjarButton,
				cancelInlineTipjarButton),
		)
		result.ReplyMarkup = &tb.InlineKeyboardMarkup{InlineKeyboard: inlineTipjarMenu.InlineKeyboard}
		results[i] = result

		// needed to set a unique string ID for each result
		results[i].SetResultID(id)

		// create persistend inline send struct
		inlineTipjar.Message = inlineMessage
		inlineTipjar.ID = id
		inlineTipjar.To = toUser
		inlineTipjar.RemainingAmount = inlineTipjar.Amount
		inlineTipjar.Memo = memo
		inlineTipjar.LanguageCode = ctx.Value("publicLanguageCode").(string)
		runtime.IgnoreError(bot.bunt.Set(inlineTipjar))
	}

	err = bot.telegram.Answer(q, &tb.QueryResponse{
		Results:   results,
		CacheTime: 1,
	})
	log.Infof("[tipjar] %s created inline tipjar %s: %d sat (%d per user)", toUserStr, inlineTipjar.ID, inlineTipjar.Amount, inlineTipjar.PerUserAmount)
	if err != nil {
		log.Errorln(err)
	}
}

func (bot *TipBot) acceptInlineTipjarHandler(ctx context.Context, c *tb.Callback) {
	from := LoadUser(ctx)
	// todo: immediatelly check if user has wallet

	fromUserStr := GetUserStr(from.Telegram)
	tx := NewInlineTipjar()
	tx.ID = c.Data
	fn, err := storage.GetTransaction(tx, tx.BaseTransaction, bot.bunt)
	if err != nil {
		log.Errorf("[tipjar] %s", err)
		return
	}
	inlineTipjar := fn.(*InlineTipjar)
	to := inlineTipjar.To
	err = storage.Lock(inlineTipjar, inlineTipjar.BaseTransaction, bot.bunt)
	if err != nil {
		log.Errorf("[tipjar] LockTipjar %s error: %s", inlineTipjar.ID, err)
		return
	}
	if !inlineTipjar.Active {
		log.Errorf(fmt.Sprintf("[tipjar] tipjar %s inactive.", inlineTipjar.ID))
		return
	}
	// release tipjar no matter what
	defer storage.Lock(inlineTipjar, inlineTipjar.BaseTransaction, bot.bunt)

	if from.Telegram.ID == to.Telegram.ID {
		bot.trySendMessage(from.Telegram, Translate(ctx, "sendYourselfMessage"))
		return
	}
	// // check if to user has already taken from the tipjar
	// for _, a := range inlineTipjar.From {
	// 	if a.Telegram.ID == to.Telegram.ID {
	// 		// to user is already in To slice, has taken from facuet
	// 		log.Infof("[tipjar] %s already  tipjar %s", GetUserStr(to.Telegram), inlineTipjar.ID)
	// 		return
	// 	}
	// }

	// check if the fromUser has enough balance
	balance, err := bot.GetUserBalance(from)
	if err != nil {
		errmsg := fmt.Sprintf("could not get balance of user %s", fromUserStr)
		log.Errorln(errmsg)
		return
	}
	// check if fromUser has balance
	if balance < inlineTipjar.PerUserAmount {
		log.Errorf("[tipjar] Balance of user %s too low", fromUserStr)
		bot.trySendMessage(from.Telegram, fmt.Sprintf(Translate(ctx, "inlineSendBalanceLowMessage"), balance))
		return
	}

	if inlineTipjar.RemainingAmount >= inlineTipjar.PerUserAmount {
		toUserStrMd := GetUserStrMd(to.Telegram)
		fromUserStrMd := GetUserStrMd(from.Telegram)
		toUserStr := GetUserStr(to.Telegram)
		fromUserStr := GetUserStr(from.Telegram)
		// check if user exists and create a wallet if not
		_, exists := bot.UserExists(to.Telegram)
		if !exists {
			log.Infof("[tipjar] User %s has no wallet.", toUserStr)
			to, err = bot.CreateWalletForTelegramUser(to.Telegram)
			if err != nil {
				errmsg := fmt.Errorf("[tipjar] Error: Could not create wallet for %s", toUserStr)
				log.Errorln(errmsg)
				return
			}
		}

		// todo: user new get username function to get userStrings
		transactionMemo := fmt.Sprintf("Tipjar from %s to %s (%d sat).", fromUserStr, toUserStr, inlineTipjar.PerUserAmount)
		t := NewTransaction(bot, from, to, inlineTipjar.PerUserAmount, TransactionType("tipjar"))
		t.Memo = transactionMemo

		success, err := t.Send()
		if !success {
			bot.trySendMessage(from.Telegram, Translate(ctx, "sendErrorMessage"))
			errMsg := fmt.Sprintf("[tipjar] Transaction failed: %s", err)
			log.Errorln(errMsg)
			return
		}

		log.Infof("[tipjar] tipjar %s: %d sat from %s to %s ", inlineTipjar.ID, inlineTipjar.PerUserAmount, fromUserStr, toUserStr)
		inlineTipjar.NGiven += 1
		inlineTipjar.From = append(inlineTipjar.From, from)
		inlineTipjar.RemainingAmount = inlineTipjar.RemainingAmount - inlineTipjar.PerUserAmount

		_, err = bot.telegram.Send(to.Telegram, fmt.Sprintf(bot.Translate(to.Telegram.LanguageCode, "inlineTipjarReceivedMessage"), fromUserStrMd, inlineTipjar.PerUserAmount))
		_, err = bot.telegram.Send(from.Telegram, fmt.Sprintf(bot.Translate(from.Telegram.LanguageCode, "inlineTipjarSentMessage"), inlineTipjar.PerUserAmount, toUserStrMd))
		if err != nil {
			errmsg := fmt.Errorf("[tipjar] Error: Send message to %s: %s", toUserStr, err)
			log.Errorln(errmsg)
			return
		}

		// build tipjar message
		inlineTipjar.Message = fmt.Sprintf(bot.Translate(inlineTipjar.LanguageCode, "inlineTipjarMessage"), inlineTipjar.PerUserAmount, inlineTipjar.RemainingAmount, inlineTipjar.Amount, inlineTipjar.NGiven, inlineTipjar.NTotal, MakeProgressbar(inlineTipjar.RemainingAmount, inlineTipjar.Amount))
		memo := inlineTipjar.Memo
		if len(memo) > 0 {
			inlineTipjar.Message = inlineTipjar.Message + fmt.Sprintf(bot.Translate(inlineTipjar.LanguageCode, "inlineTipjarAppendMemo"), memo)
		}

		// register new inline buttons
		inlineTipjarMenu = &tb.ReplyMarkup{ResizeReplyKeyboard: true}
		acceptInlineTipjarButton := inlineTipjarMenu.Data(Translate(ctx, "payReceiveButtonMessage"), "confirm_tipjar_inline")
		cancelInlineTipjarButton := inlineTipjarMenu.Data(Translate(ctx, "cancelButtonMessage"), "cancel_tipjar_inline")
		acceptInlineTipjarButton.Data = inlineTipjar.ID
		cancelInlineTipjarButton.Data = inlineTipjar.ID

		inlineTipjarMenu.Inline(
			inlineTipjarMenu.Row(
				acceptInlineTipjarButton,
				cancelInlineTipjarButton),
		)
		// update message
		log.Infoln(inlineTipjar.Message)
		bot.tryEditMessage(c.Message, inlineTipjar.Message, inlineTipjarMenu)
	}
	if inlineTipjar.RemainingAmount < inlineTipjar.PerUserAmount {
		// tipjar is depleted
		inlineTipjar.Message = fmt.Sprintf(bot.Translate(inlineTipjar.LanguageCode, "inlineTipjarEndedMessage"), inlineTipjar.Amount, inlineTipjar.NGiven)
		bot.tryEditMessage(c.Message, inlineTipjar.Message)
		inlineTipjar.Active = false
	}

}

func (bot *TipBot) cancelInlineTipjarHandler(ctx context.Context, c *tb.Callback) {
	tx := NewInlineTipjar()
	tx.ID = c.Data
	fn, err := storage.GetTransaction(tx, tx.BaseTransaction, bot.bunt)

	if err != nil {
		log.Errorf("[cancelInlineSendHandler] %s", err)
		return
	}
	inlineTipjar := fn.(*InlineTipjar)
	if c.Sender.ID == inlineTipjar.To.Telegram.ID {
		bot.tryEditMessage(c.Message, bot.Translate(inlineTipjar.LanguageCode, "inlineTipjarCancelledMessage"), &tb.ReplyMarkup{})
		// set the inlineTipjar inactive
		inlineTipjar.Active = false
		inlineTipjar.InTransaction = false
		runtime.IgnoreError(bot.bunt.Set(inlineTipjar))
	}
	return
}
