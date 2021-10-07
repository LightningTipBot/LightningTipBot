package telegram

import (
	"context"
	"fmt"
	"github.com/LightningTipBot/LightningTipBot/internal/i18n"

	"github.com/LightningTipBot/LightningTipBot/internal/lnbits"
	"github.com/LightningTipBot/LightningTipBot/internal/storage/transaction"

	"github.com/LightningTipBot/LightningTipBot/internal/runtime"
	log "github.com/sirupsen/logrus"
	tb "gopkg.in/tucnak/telebot.v2"
)

var (
	inlineSendMenu      = &tb.ReplyMarkup{ResizeReplyKeyboard: true}
	btnCancelInlineSend = inlineSendMenu.Data("🚫 Cancel", "cancel_send_inline")
	btnAcceptInlineSend = inlineSendMenu.Data("✅ Receive", "confirm_send_inline")
)

type InlineSend struct {
	*transaction.Base
	Message      string       `json:"inline_send_message"`
	Amount       int          `json:"inline_send_amount"`
	From         *lnbits.User `json:"inline_send_from"`
	To           *tb.User     `json:"inline_send_to"`
	Memo         string       `json:"inline_send_memo"`
	LanguageCode string       `json:"languagecode"`
}

func (bot TipBot) makeSendKeyboard(ctx context.Context, id string) *tb.ReplyMarkup {
	acceptInlineSendButton := inlineSendMenu.Data(Translate(ctx, "receiveButtonMessage"), "confirm_send_inline")
	cancelInlineSendButton := inlineSendMenu.Data(Translate(ctx, "cancelButtonMessage"), "cancel_send_inline")
	acceptInlineSendButton.Data = id
	cancelInlineSendButton.Data = id

	inlineSendMenu.Inline(
		inlineSendMenu.Row(
			acceptInlineSendButton,
			cancelInlineSendButton),
	)
	return inlineSendMenu
}

func (bot TipBot) handleInlineSendQuery(ctx context.Context, q *tb.Query) {
	// inlineSend := NewInlineSend()
	// var err error
	amount, err := decodeAmountFromCommand(q.Text)
	if err != nil {
		bot.inlineQueryReplyWithError(q, TranslateUser(ctx, "inlineQuerySendTitle"), fmt.Sprintf(TranslateUser(ctx, "inlineQuerySendDescription"), bot.Telegram.Me.Username))
		return
	}
	if amount < 1 {
		bot.inlineQueryReplyWithError(q, TranslateUser(ctx, "inlineSendInvalidAmountMessage"), fmt.Sprintf(Translate(ctx, "inlineQuerySendDescription"), bot.Telegram.Me.Username))
		return
	}
	fromUser := LoadUser(ctx)
	fromUserStr := GetUserStr(&q.From)
	balance, err := bot.GetUserBalance(fromUser)
	if err != nil {
		errmsg := fmt.Sprintf("could not get balance of user %s", fromUserStr)
		log.Errorln(errmsg)
		return
	}
	// check if fromUser has balance
	if balance < amount {
		log.Errorf("Balance of user %s too low", fromUserStr)
		bot.inlineQueryReplyWithError(q, TranslateUser(ctx, "inlineSendBalanceLowMessage"), fmt.Sprintf(TranslateUser(ctx, "inlineQuerySendDescription"), bot.Telegram.Me.Username))
		return
	}
	// check for memo in command
	memo := GetMemoFromCommand(q.Text, 2)
	urls := []string{
		queryImage,
	}
	results := make(tb.Results, len(urls)) // []tb.Result
	for i, url := range urls {
		inlineMessage := fmt.Sprintf(Translate(ctx, "inlineSendMessage"), fromUserStr, amount)
		if len(memo) > 0 {
			inlineMessage = inlineMessage + fmt.Sprintf(Translate(ctx, "inlineSendAppendMemo"), memo)
		}
		result := &tb.ArticleResult{
			// URL:         url,
			Text:        inlineMessage,
			Title:       fmt.Sprintf(TranslateUser(ctx, "inlineResultSendTitle"), amount),
			Description: fmt.Sprintf(TranslateUser(ctx, "inlineResultSendDescription"), amount),
			// required for photos
			ThumbURL: url,
		}
		id := fmt.Sprintf("inl-send-%d-%d-%s", q.From.ID, amount, RandStringRunes(5))
		result.ReplyMarkup = &tb.InlineKeyboardMarkup{InlineKeyboard: bot.makeSendKeyboard(ctx, id).InlineKeyboard}
		results[i] = result
		// needed to set a unique string ID for each result
		results[i].SetResultID(id)

		// add data to persistent object
		inlineSend := InlineSend{
			Base:         transaction.New(transaction.ID(id)),
			Message:      inlineMessage,
			From:         fromUser,
			Memo:         memo,
			Amount:       amount,
			LanguageCode: ctx.Value("publicLanguageCode").(string),
		}

		// add result to persistent struct
		runtime.IgnoreError(inlineSend.Set(inlineSend, bot.Bunt))
	}

	err = bot.Telegram.Answer(q, &tb.QueryResponse{
		Results:   results,
		CacheTime: 1, // 60 == 1 minute, todo: make higher than 1 s in production

	})
	if err != nil {
		log.Errorln(err)
	}
}

func (bot *TipBot) acceptInlineSendHandler(ctx context.Context, c *tb.Callback) {
	to := LoadUser(ctx)
	tx := &InlineSend{Base: transaction.New(transaction.ID(c.Data))}
	sn, err := tx.Get(tx, bot.Bunt)
	// immediatelly set intransaction to block duplicate calls
	if err != nil {
		log.Errorf("[acceptInlineSendHandler] %s", err)
		return
	}
	inlineSend := sn.(*InlineSend)

	fromUser := inlineSend.From
	// immediatelly set intransaction to block duplicate calls
	err = inlineSend.Lock(inlineSend, bot.Bunt)
	if err != nil {
		log.Errorf("[getInlineSend] %s", err)
		return
	}
	if !inlineSend.Active {
		log.Errorf("[acceptInlineSendHandler] inline send not active anymore")
		return
	}

	defer inlineSend.Release(inlineSend, bot.Bunt)

	amount := inlineSend.Amount

	inlineSend.To = to.Telegram

	if fromUser.Telegram.ID == to.Telegram.ID {
		bot.trySendMessage(fromUser.Telegram, Translate(ctx, "sendYourselfMessage"))
		return
	}

	toUserStrMd := GetUserStrMd(to.Telegram)
	fromUserStrMd := GetUserStrMd(fromUser.Telegram)
	toUserStr := GetUserStr(to.Telegram)
	fromUserStr := GetUserStr(fromUser.Telegram)

	// check if user exists and create a wallet if not
	_, exists := bot.UserExists(to.Telegram)
	if !exists {
		log.Infof("[sendInline] User %s has no wallet.", toUserStr)
		to, err = bot.CreateWalletForTelegramUser(to.Telegram)
		if err != nil {
			errmsg := fmt.Errorf("[sendInline] Error: Could not create wallet for %s", toUserStr)
			log.Errorln(errmsg)
			return
		}
	}
	// set inactive to avoid double-sends
	inlineSend.Inactivate(inlineSend, bot.Bunt)

	// todo: user new get username function to get userStrings
	transactionMemo := fmt.Sprintf("InlineSend from %s to %s (%d sat).", fromUserStr, toUserStr, amount)
	t := NewTransaction(bot, fromUser, to, amount, TransactionType("inline send"))
	t.Memo = transactionMemo
	success, err := t.Send()
	if !success {
		errMsg := fmt.Sprintf("[sendInline] Transaction failed: %s", err)
		log.Errorln(errMsg)
		bot.tryEditMessage(c.Message, i18n.Translate(inlineSend.LanguageCode, "inlineSendFailedMessage"), &tb.ReplyMarkup{})
		return
	}

	log.Infof("[sendInline] %d sat from %s to %s", amount, fromUserStr, toUserStr)

	inlineSend.Message = fmt.Sprintf("%s", fmt.Sprintf(i18n.Translate(inlineSend.LanguageCode, "inlineSendUpdateMessageAccept"), amount, fromUserStrMd, toUserStrMd))
	memo := inlineSend.Memo
	if len(memo) > 0 {
		inlineSend.Message = inlineSend.Message + fmt.Sprintf(i18n.Translate(inlineSend.LanguageCode, "inlineSendAppendMemo"), memo)
	}
	if !to.Initialized {
		inlineSend.Message += "\n\n" + fmt.Sprintf(i18n.Translate(inlineSend.LanguageCode, "inlineSendCreateWalletMessage"), GetUserStrMd(bot.Telegram.Me))
	}
	bot.tryEditMessage(c.Message, inlineSend.Message, &tb.ReplyMarkup{})
	// notify users
	_, err = bot.Telegram.Send(to.Telegram, fmt.Sprintf(i18n.Translate(to.Telegram.LanguageCode, "sendReceivedMessage"), fromUserStrMd, amount))
	_, err = bot.Telegram.Send(fromUser.Telegram, fmt.Sprintf(i18n.Translate(fromUser.Telegram.LanguageCode, "sendSentMessage"), amount, toUserStrMd))
	if err != nil {
		errmsg := fmt.Errorf("[sendInline] Error: Send message to %s: %s", toUserStr, err)
		log.Errorln(errmsg)
		return
	}
}

func (bot *TipBot) cancelInlineSendHandler(ctx context.Context, c *tb.Callback) {
	tx := &InlineSend{Base: transaction.New(transaction.ID(c.Data))}
	sn, err := tx.Get(tx, bot.Bunt)
	// immediatelly set intransaction to block duplicate calls
	if err != nil {
		log.Errorf("[cancelInlineSendHandler] %s", err)
		return
	}
	inlineSend := sn.(*InlineSend)
	if c.Sender.ID == inlineSend.From.Telegram.ID {
		bot.tryEditMessage(c.Message, i18n.Translate(inlineSend.LanguageCode, "sendCancelledMessage"), &tb.ReplyMarkup{})
		// set the inlineSend inactive
		inlineSend.Active = false
		inlineSend.InTransaction = false
		runtime.IgnoreError(inlineSend.Set(inlineSend, bot.Bunt))
	}
	return
}
