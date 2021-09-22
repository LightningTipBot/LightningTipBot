package main

import (
	"context"
	"fmt"
	"time"

	"github.com/LightningTipBot/LightningTipBot/internal/lnbits"

	"github.com/LightningTipBot/LightningTipBot/internal/runtime"
	log "github.com/sirupsen/logrus"
	tb "gopkg.in/tucnak/telebot.v2"
)

var (
	inlineFaucetMenu      = &tb.ReplyMarkup{ResizeReplyKeyboard: true}
	btnCancelInlineFaucet = inlineFaucetMenu.Data("🚫 Cancel", "cancel_faucet_inline")
	btnAcceptInlineFaucet = inlineFaucetMenu.Data("✅ Collect", "confirm_faucet_inline")
)

type InlineFaucet struct {
	*storage.Transaction
	Message         string       `json:"inline_faucet_message"`
	Amount          int          `json:"inline_faucet_amount"`
	RemainingAmount int          `json:"inline_faucet_remainingamount"`
	PerUserAmount   int          `json:"inline_faucet_peruseramount"`
	From            *lnbits.User `json:"inline_faucet_from"`
	To              []*tb.User   `json:"inline_faucet_to"`
	Memo            string       `json:"inline_faucet_memo"`
	NTotal          int          `json:"inline_faucet_ntotal"`
	NTaken          int          `json:"inline_faucet_ntaken"`
	UserNeedsWallet bool         `json:"inline_faucet_userneedswallet"`
	LanguageCode    string       `json:"languagecode"`
}

func NewInlineFaucet() *InlineFaucet {
	inlineFaucet := &InlineFaucet{
		Message:         "",
		NTaken:          0,
		UserNeedsWallet: false,
		Transaction: &storage.Transaction{
			InTransaction: false,
			Active:        true,
		},
	}
	return inlineFaucet

}

func (bot TipBot) faucetHandler(ctx context.Context, m *tb.Message) {
	bot.anyTextHandler(ctx, m)
	if m.Private() {
		bot.trySendMessage(m.Sender, fmt.Sprintf(Translate(ctx, "inlineFaucetHelpText"), Translate(ctx, "inlineFaucetHelpFaucetInGroup")))
		return
	}
	ctx = bot.mapFaucetLanguage(ctx, m.Text)
	inlineFaucet := NewInlineFaucet()
	var err error
	inlineFaucet.Amount, err = decodeAmountFromCommand(m.Text)
	if err != nil {
		bot.trySendMessage(m.Sender, fmt.Sprintf(Translate(ctx, "inlineFaucetHelpText"), Translate(ctx, "inlineFaucetInvalidAmountMessage")))
		bot.tryDeleteMessage(m)
		return
	}
	peruserStr, err := getArgumentFromCommand(m.Text, 2)
	if err != nil {
		bot.trySendMessage(m.Sender, fmt.Sprintf(Translate(ctx, "inlineFaucetHelpText"), ""))
		bot.tryDeleteMessage(m)
		return
	}
	inlineFaucet.PerUserAmount, err = getAmount(peruserStr)
	if err != nil {
		bot.trySendMessage(m.Sender, fmt.Sprintf(Translate(ctx, "inlineFaucetHelpText"), Translate(ctx, "inlineFaucetInvalidAmountMessage")))
		bot.tryDeleteMessage(m)
		return
	}
	// peruser amount must be >1 and a divisor of amount
	if inlineFaucet.PerUserAmount < 1 || inlineFaucet.Amount%inlineFaucet.PerUserAmount != 0 {
		bot.trySendMessage(m.Sender, fmt.Sprintf(Translate(ctx, "inlineFaucetHelpText"), Translate(ctx, "inlineFaucetInvalidPeruserAmountMessage")))
		bot.tryDeleteMessage(m)
		return
	}
	inlineFaucet.NTotal = inlineFaucet.Amount / inlineFaucet.PerUserAmount
	fromUser := LoadUser(ctx)
	fromUserStr := GetUserStr(m.Sender)
	balance, err := bot.GetUserBalance(fromUser)
	if err != nil {
		errmsg := fmt.Sprintf("could not get balance of user %s", fromUserStr)
		log.Errorln(errmsg)
		bot.tryDeleteMessage(m)
		return
	}
	// check if fromUser has balance
	if balance < inlineFaucet.Amount {
		log.Errorf("[faucet] Balance of user %s too low", fromUserStr)
		bot.trySendMessage(m.Sender, fmt.Sprintf(Translate(ctx, "inlineSendBalanceLowMessage"), balance))
		bot.tryDeleteMessage(m)
		return
	}

	// // check for memo in command
	memo := GetMemoFromCommand(m.Text, 3)

	inlineMessage := fmt.Sprintf(Translate(ctx, "inlineFaucetMessage"), inlineFaucet.PerUserAmount, inlineFaucet.Amount, inlineFaucet.Amount, 0, inlineFaucet.NTotal, MakeProgressbar(inlineFaucet.Amount, inlineFaucet.Amount))
	if len(memo) > 0 {
		inlineMessage = inlineMessage + fmt.Sprintf(Translate(ctx, "inlineFaucetAppendMemo"), memo)
	}

	inlineFaucet.ID = fmt.Sprintf("inl-faucet-%d-%d-%s", m.Sender.ID, inlineFaucet.Amount, RandStringRunes(5))
	acceptInlineFaucetButton := inlineFaucetMenu.Data(Translate(ctx, "collectButtonMessage"), "confirm_faucet_inline")
	cancelInlineFaucetButton := inlineFaucetMenu.Data(Translate(ctx, "cancelButtonMessage"), "cancel_faucet_inline")
	acceptInlineFaucetButton.Data = inlineFaucet.ID
	cancelInlineFaucetButton.Data = inlineFaucet.ID

	inlineFaucetMenu.Inline(
		inlineFaucetMenu.Row(
			acceptInlineFaucetButton,
			cancelInlineFaucetButton),
	)
	bot.trySendMessage(m.Chat, inlineMessage, inlineFaucetMenu)
	log.Infof("[faucet] %s created faucet %s: %d sat (%d per user)", fromUserStr, inlineFaucet.ID, inlineFaucet.Amount, inlineFaucet.PerUserAmount)
	inlineFaucet.Message = inlineMessage
	inlineFaucet.From = fromUser
	inlineFaucet.Memo = memo
	inlineFaucet.RemainingAmount = inlineFaucet.Amount
	inlineFaucet.LanguageCode = ctx.Value("publicLanguageCode").(string)
	runtime.IgnoreError(bot.Bunt.Set(inlineFaucet))

}

func (bot TipBot) handleInlineFaucetQuery(ctx context.Context, q *tb.Query) {
	inlineFaucet := NewInlineFaucet()
	var err error
	inlineFaucet.Amount, err = decodeAmountFromCommand(q.Text)
	if err != nil {
		bot.inlineQueryReplyWithError(q, TranslateUser(ctx, "inlineQueryFaucetTitle"), fmt.Sprintf(TranslateUser(ctx, "inlineQueryFaucetDescription"), bot.Telegram.Me.Username))
		return
	}
	if inlineFaucet.Amount < 1 {
		bot.inlineQueryReplyWithError(q, TranslateUser(ctx, "inlineSendInvalidAmountMessage"), fmt.Sprintf(TranslateUser(ctx, "inlineQueryFaucetDescription"), bot.Telegram.Me.Username))
		return
	}

	peruserStr, err := getArgumentFromCommand(q.Text, 2)
	if err != nil {
		bot.inlineQueryReplyWithError(q, TranslateUser(ctx, "inlineQueryFaucetTitle"), fmt.Sprintf(TranslateUser(ctx, "inlineQueryFaucetDescription"), bot.Telegram.Me.Username))
		return
	}
	inlineFaucet.PerUserAmount, err = getAmount(peruserStr)
	if err != nil {
		bot.inlineQueryReplyWithError(q, TranslateUser(ctx, "inlineQueryFaucetTitle"), fmt.Sprintf(TranslateUser(ctx, "inlineQueryFaucetDescription"), bot.Telegram.Me.Username))
		return
	}
	// peruser amount must be >1 and a divisor of amount
	if inlineFaucet.PerUserAmount < 1 || inlineFaucet.Amount%inlineFaucet.PerUserAmount != 0 {
		bot.inlineQueryReplyWithError(q, TranslateUser(ctx, "inlineFaucetInvalidPeruserAmountMessage"), fmt.Sprintf(TranslateUser(ctx, "inlineQueryFaucetDescription"), bot.Telegram.Me.Username))
		return
	}
	inlineFaucet.NTotal = inlineFaucet.Amount / inlineFaucet.PerUserAmount
	fromUser := LoadUser(ctx)
	fromUserStr := GetUserStr(&q.From)
	balance, err := bot.GetUserBalance(fromUser)
	if err != nil {
		errmsg := fmt.Sprintf("could not get balance of user %s", fromUserStr)
		log.Errorln(errmsg)
		return
	}
	// check if fromUser has balance
	if balance < inlineFaucet.Amount {
		log.Errorf("Balance of user %s too low", fromUserStr)
		bot.inlineQueryReplyWithError(q, fmt.Sprintf(TranslateUser(ctx, "inlineSendBalanceLowMessage"), balance), fmt.Sprintf(TranslateUser(ctx, "inlineQueryFaucetDescription"), bot.Telegram.Me.Username))
		return
	}

	// check for memo in command
	memo := GetMemoFromCommand(q.Text, 3)

	urls := []string{
		queryImage,
	}
	results := make(tb.Results, len(urls)) // []tb.Result
	for i, url := range urls {
		inlineMessage := fmt.Sprintf(Translate(ctx, "inlineFaucetMessage"), inlineFaucet.PerUserAmount, inlineFaucet.Amount, inlineFaucet.Amount, 0, inlineFaucet.NTotal, MakeProgressbar(inlineFaucet.Amount, inlineFaucet.Amount))
		if len(memo) > 0 {
			inlineMessage = inlineMessage + fmt.Sprintf(Translate(ctx, "inlineFaucetAppendMemo"), memo)
		}
		result := &tb.ArticleResult{
			// URL:         url,
			Text:        inlineMessage,
			Title:       fmt.Sprintf(TranslateUser(ctx, "inlineResultFaucetTitle"), inlineFaucet.Amount),
			Description: TranslateUser(ctx, "inlineResultFaucetDescription"),
			// required for photos
			ThumbURL: url,
		}
		id := fmt.Sprintf("inl-faucet-%d-%d-%s", q.From.ID, inlineFaucet.Amount, RandStringRunes(5))
		acceptInlineFaucetButton := inlineFaucetMenu.Data(Translate(ctx, "collectButtonMessage"), "confirm_faucet_inline")
		cancelInlineFaucetButton := inlineFaucetMenu.Data(Translate(ctx, "cancelButtonMessage"), "cancel_faucet_inline")
		acceptInlineFaucetButton.Data = id
		cancelInlineFaucetButton.Data = id

		inlineFaucetMenu.Inline(
			inlineFaucetMenu.Row(
				acceptInlineFaucetButton,
				cancelInlineFaucetButton),
		)
		result.ReplyMarkup = &tb.InlineKeyboardMarkup{InlineKeyboard: inlineFaucetMenu.InlineKeyboard}
		results[i] = result

		// needed to set a unique string ID for each result
		results[i].SetResultID(id)

		// create persistend inline send struct
		inlineFaucet.Message = inlineMessage
		inlineFaucet.ID = id
		inlineFaucet.From = fromUser
		inlineFaucet.RemainingAmount = inlineFaucet.Amount
		inlineFaucet.Memo = memo
		inlineFaucet.LanguageCode = ctx.Value("publicLanguageCode").(string)
		runtime.IgnoreError(bot.Bunt.Set(inlineFaucet))
	}

	err = bot.Telegram.Answer(q, &tb.QueryResponse{
		Results:   results,
		CacheTime: 1,
	})
	log.Infof("[faucet] %s created inline faucet %s: %d sat (%d per user)", fromUserStr, inlineFaucet.ID, inlineFaucet.Amount, inlineFaucet.PerUserAmount)
	if err != nil {
		log.Errorln(err)
	}
}

func (bot *TipBot) acceptInlineFaucetHandler(ctx context.Context, c *tb.Callback) {
	to := LoadUser(ctx)
	tx := NewInlineFaucet()
	tx.ID = c.Data
	fn, err := storage.GetTransaction(tx, tx.Transaction, bot.bunt)
	if err != nil {
		log.Errorf("[faucet] %s", err)
		return
	}
	inlineFaucet := fn.(*InlineFaucet)
	from := inlineFaucet.From
	err = storage.Lock(inlineFaucet, inlineFaucet.Transaction, bot.bunt)
	if err != nil {
		log.Errorf("[faucet] LockFaucet %s error: %s", inlineFaucet.ID, err)
		return
	}
	if !inlineFaucet.Active {
		log.Errorf(fmt.Sprintf("[faucet] faucet %s inactive.", inlineFaucet.ID))
		return
	}
	// release faucet no matter what
	defer storage.Lock(inlineFaucet, inlineFaucet.Transaction, bot.bunt)

	if from.Telegram.ID == to.Telegram.ID {
		bot.trySendMessage(from.Telegram, Translate(ctx, "sendYourselfMessage"))
		return
	}
	// check if to user has already taken from the faucet
	for _, a := range inlineFaucet.To {
		if a.ID == to.Telegram.ID {
			// to user is already in To slice, has taken from facuet
			log.Infof("[faucet] %s already took from faucet %s", GetUserStr(to.Telegram), inlineFaucet.ID)
			return
		}
	}

	if inlineFaucet.RemainingAmount >= inlineFaucet.PerUserAmount {
		toUserStrMd := GetUserStrMd(to.Telegram)
		fromUserStrMd := GetUserStrMd(from.Telegram)
		toUserStr := GetUserStr(to.Telegram)
		fromUserStr := GetUserStr(from.Telegram)
		// check if user exists and create a wallet if not
		_, exists := bot.UserExists(to.Telegram)
		if !exists {
			log.Infof("[faucet] User %s has no wallet.", toUserStr)
			to, err = bot.CreateWalletForTelegramUser(to.Telegram)
			if err != nil {
				errmsg := fmt.Errorf("[faucet] Error: Could not create wallet for %s", toUserStr)
				log.Errorln(errmsg)
				return
			}
		}

		if !to.Initialized {
			inlineFaucet.UserNeedsWallet = true
		}

		// todo: user new get username function to get userStrings
		transactionMemo := fmt.Sprintf("Faucet from %s to %s (%d sat).", fromUserStr, toUserStr, inlineFaucet.PerUserAmount)
		t := NewTransaction(bot, from, to, inlineFaucet.PerUserAmount, TransactionType("faucet"))
		t.Memo = transactionMemo

		success, err := t.Send()
		if !success {
			bot.trySendMessage(from.Telegram, Translate(ctx, "sendErrorMessage"))
			errMsg := fmt.Sprintf("[faucet] Transaction failed: %s", err)
			log.Errorln(errMsg)
			return
		}

		log.Infof("[faucet] faucet %s: %d sat from %s to %s ", inlineFaucet.ID, inlineFaucet.PerUserAmount, fromUserStr, toUserStr)
		inlineFaucet.NTaken += 1
		inlineFaucet.To = append(inlineFaucet.To, to.Telegram)
		inlineFaucet.RemainingAmount = inlineFaucet.RemainingAmount - inlineFaucet.PerUserAmount

		_, err = bot.Telegram.Send(to.Telegram, fmt.Sprintf(i18n.Translate(to.Telegram.LanguageCode, "inlineFaucetReceivedMessage"), fromUserStrMd, inlineFaucet.PerUserAmount))
		_, err = bot.Telegram.Send(from.Telegram, fmt.Sprintf(i18n.Translate(from.Telegram.LanguageCode, "inlineFaucetSentMessage"), inlineFaucet.PerUserAmount, toUserStrMd))
		if err != nil {
			errmsg := fmt.Errorf("[faucet] Error: Send message to %s: %s", toUserStr, err)
			log.Errorln(errmsg)
			return
		}

		// build faucet message
		inlineFaucet.Message = fmt.Sprintf(i18n.Translate(inlineFaucet.LanguageCode, "inlineFaucetMessage"), inlineFaucet.PerUserAmount, inlineFaucet.RemainingAmount, inlineFaucet.Amount, inlineFaucet.NTaken, inlineFaucet.NTotal, MakeProgressbar(inlineFaucet.RemainingAmount, inlineFaucet.Amount))
		memo := inlineFaucet.Memo
		if len(memo) > 0 {
			inlineFaucet.Message = inlineFaucet.Message + fmt.Sprintf(i18n.Translate(inlineFaucet.LanguageCode, "inlineFaucetAppendMemo"), memo)
		}
		if inlineFaucet.UserNeedsWallet {
			inlineFaucet.Message += "\n\n" + fmt.Sprintf(i18n.Translate(inlineFaucet.LanguageCode, "inlineFaucetCreateWalletMessage"), GetUserStrMd(bot.Telegram.Me))
		}

		// register new inline buttons
		inlineFaucetMenu = &tb.ReplyMarkup{ResizeReplyKeyboard: true}
		acceptInlineFaucetButton := inlineFaucetMenu.Data(i18n.Translate(inlineFaucet.LanguageCode, "collectButtonMessage"), "confirm_faucet_inline")
		cancelInlineFaucetButton := inlineFaucetMenu.Data(i18n.Translate(inlineFaucet.LanguageCode, "cancelButtonMessage"), "cancel_faucet_inline")
		acceptInlineFaucetButton.Data = inlineFaucet.ID
		cancelInlineFaucetButton.Data = inlineFaucet.ID

		inlineFaucetMenu.Inline(
			inlineFaucetMenu.Row(
				acceptInlineFaucetButton,
				cancelInlineFaucetButton),
		)
		// update message
		log.Infoln(inlineFaucet.Message)
		bot.tryEditMessage(c.Message, inlineFaucet.Message, inlineFaucetMenu)
	}
	if inlineFaucet.RemainingAmount < inlineFaucet.PerUserAmount {
		// faucet is depleted
		inlineFaucet.Message = fmt.Sprintf(i18n.Translate(inlineFaucet.LanguageCode, "inlineFaucetEndedMessage"), inlineFaucet.Amount, inlineFaucet.NTaken)
		if inlineFaucet.UserNeedsWallet {
			inlineFaucet.Message += "\n\n" + fmt.Sprintf(i18n.Translate(inlineFaucet.LanguageCode, "inlineFaucetCreateWalletMessage"), GetUserStrMd(bot.Telegram.Me))
		}
		bot.tryEditMessage(c.Message, inlineFaucet.Message)
		inlineFaucet.Active = false
	}

}

func (bot *TipBot) cancelInlineFaucetHandler(ctx context.Context, c *tb.Callback) {
	tx := NewInlineFaucet()
	tx.ID = c.Data
	fn, err := storage.GetTransaction(tx, tx.Transaction, bot.bunt)

	if err != nil {
		log.Errorf("[cancelInlineSendHandler] %s", err)
		return
	}
	inlineFaucet := fn.(*InlineFaucet)
	if c.Sender.ID == inlineFaucet.From.Telegram.ID {
		bot.tryEditMessage(c.Message, i18n.Translate(inlineFaucet.LanguageCode, "inlineFaucetCancelledMessage"), &tb.ReplyMarkup{})
		// set the inlineFaucet inactive
		inlineFaucet.Active = false
		inlineFaucet.InTransaction = false
		runtime.IgnoreError(bot.Bunt.Set(inlineFaucet))
	}
	return
}
