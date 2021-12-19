package telegram

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/eko/gocache/store"
	"github.com/skip2/go-qrcode"

	"github.com/LightningTipBot/LightningTipBot/internal/i18n"
	"github.com/LightningTipBot/LightningTipBot/internal/lnbits"
	"github.com/LightningTipBot/LightningTipBot/internal/storage/transaction"

	"github.com/LightningTipBot/LightningTipBot/internal/runtime"
	log "github.com/sirupsen/logrus"
	tb "gopkg.in/lightningtipbot/telebot.v2"
)

var (
	inlineReceiveMenu      = &tb.ReplyMarkup{ResizeReplyKeyboard: true}
	btnCancelInlineReceive = inlineReceiveMenu.Data("🚫 Cancel", "cancel_receive_inline")
	btnAcceptInlineReceive = inlineReceiveMenu.Data("💸 Pay", "confirm_receive_inline")
)

type InlineReceive struct {
	*transaction.Base
	MessageText       string       `json:"inline_receive_messagetext"`
	Message           *tb.Message  `json:"inline_receive_message"`
	Amount            int64        `json:"inline_receive_amount"`
	From              *lnbits.User `json:"inline_receive_from"`
	To                *lnbits.User `json:"inline_receive_to"`
	From_SpecificUser bool         `json:"from_specific_user"`
	Memo              string       `json:"inline_receive_memo"`
	LanguageCode      string       `json:"languagecode"`
}

func (bot TipBot) makeReceiveKeyboard(ctx context.Context, id string) *tb.ReplyMarkup {
	acceptInlineReceiveButton := inlineReceiveMenu.Data(Translate(ctx, "payReceiveButtonMessage"), "confirm_receive_inline")
	cancelInlineReceiveButton := inlineReceiveMenu.Data(Translate(ctx, "cancelButtonMessage"), "cancel_receive_inline")
	acceptInlineReceiveButton.Data = id
	cancelInlineReceiveButton.Data = id
	inlineReceiveMenu.Inline(
		inlineReceiveMenu.Row(
			cancelInlineReceiveButton,
			acceptInlineReceiveButton,
		),
	)
	return inlineReceiveMenu
}

func (bot TipBot) handleInlineReceiveQuery(ctx context.Context, q *tb.Query) {
	to := LoadUser(ctx)
	amount, err := decodeAmountFromCommand(q.Text)
	if err != nil {
		bot.inlineQueryReplyWithError(q, Translate(ctx, "inlineQueryReceiveTitle"), fmt.Sprintf(Translate(ctx, "inlineQueryReceiveDescription"), bot.Telegram.Me.Username))
		return
	}
	if amount < 1 {
		bot.inlineQueryReplyWithError(q, Translate(ctx, "inlineSendInvalidAmountMessage"), fmt.Sprintf(Translate(ctx, "inlineQueryReceiveDescription"), bot.Telegram.Me.Username))
		return
	}
	toUserStr := GetUserStr(&q.From)

	// check whether the 3rd argument is a username
	// command is "@LightningTipBot receive 123 @from_user This is the memo"
	memo_argn := 2 // argument index at which the memo starts, will be 3 if there is a from_username in command
	fromUserDb := &lnbits.User{}
	from_SpecificUser := false
	if len(strings.Split(q.Text, " ")) > 2 {
		from_username := strings.Split(q.Text, " ")[2]
		if strings.HasPrefix(from_username, "@") {
			fromUserDb, err = GetUserByTelegramUsername(from_username[1:], bot) // must be without the @
			if err != nil {
				//bot.tryDeleteMessage(m)
				//bot.trySendMessage(m.Sender, fmt.Sprintf(Translate(ctx, "sendUserHasNoWalletMessage"), toUserStrMention))
				bot.inlineQueryReplyWithError(q,
					fmt.Sprintf(TranslateUser(ctx, "sendUserHasNoWalletMessage"), from_username),
					fmt.Sprintf(TranslateUser(ctx, "inlineQueryReceiveDescription"),
						bot.Telegram.Me.Username))
				return
			}
			memo_argn = 3 // assume that memo starts after the from_username
			from_SpecificUser = true
		}
	}

	// check for memo in command
	memo := GetMemoFromCommand(q.Text, memo_argn)
	urls := []string{
		queryImage,
	}
	results := make(tb.Results, len(urls)) // []tb.Result
	for i, url := range urls {
		inlineMessage := fmt.Sprintf(Translate(ctx, "inlineReceiveMessage"), toUserStr, amount)

		// modify message if payment is to specific user
		if from_SpecificUser {
			inlineMessage = fmt.Sprintf("@%s: %s", fromUserDb.Telegram.Username, inlineMessage)
		}

		if len(memo) > 0 {
			inlineMessage = inlineMessage + fmt.Sprintf(Translate(ctx, "inlineReceiveAppendMemo"), memo)
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
			Base:              transaction.New(transaction.ID(id)),
			MessageText:       inlineMessage,
			To:                to,
			Memo:              memo,
			Amount:            amount,
			From:              fromUserDb,
			From_SpecificUser: from_SpecificUser,
			LanguageCode:      ctx.Value("publicLanguageCode").(string),
		}
		bot.Cache.Set(inlineReceive.ID, inlineReceive, &store.Options{Expiration: 5 * time.Minute})
	}

	err = bot.Telegram.Answer(q, &tb.QueryResponse{
		Results:   results,
		CacheTime: 1, // 60 == 1 minute, todo: make higher than 1 s in production

	})
	if err != nil {
		log.Errorln(err)
	}
}

func (bot *TipBot) acceptInlineReceiveHandler(ctx context.Context, c *tb.Callback) {
	tx := &InlineReceive{Base: transaction.New(transaction.ID(c.Data))}
	rn, err := tx.Get(tx, bot.Bunt)
	// immediatelly set intransaction to block duplicate calls
	if err != nil {
		log.Errorf("[getInlineReceive] %s", err)
		return
	}
	inlineReceive := rn.(*InlineReceive)
	if !inlineReceive.Active {
		log.Errorf("[acceptInlineReceiveHandler] inline receive not active anymore")
		return
	}

	// user `from` is the one who is SENDING
	// user `to` is the one who is RECEIVING
	from := LoadUser(ctx)
	// check if this payment is requested from a specific user
	if inlineReceive.From_SpecificUser {
		if inlineReceive.From.Telegram.ID != from.Telegram.ID {
			// log.Infof("User %d is not User %d", inlineReceive.From.Telegram.ID, from.Telegram.ID)
			return
		}
	} else {
		// otherwise, we just set it to the user who has clicked
		inlineReceive.From = from

	}
	inlineReceive.Message = c.Message
	runtime.IgnoreError(inlineReceive.Set(inlineReceive, bot.Bunt))

	to := inlineReceive.To
	if from.Telegram.ID == to.Telegram.ID {
		bot.trySendMessage(from.Telegram, Translate(ctx, "sendYourselfMessage"))
		return
	}

	balance, err := bot.GetUserBalance(from)
	if err != nil {
		errmsg := fmt.Sprintf("[inlineReceive] Error: Could not get user balance: %s", err)
		log.Warnln(errmsg)
	}

	if from.Wallet == nil || balance < inlineReceive.Amount {
		// if user has no wallet, show invoice
		bot.tryEditMessage(inlineReceive.Message, inlineReceive.MessageText, &tb.ReplyMarkup{})
		// runtime.IgnoreError(inlineReceive.Set(inlineReceive, bot.Bunt))
		bot.inlineReceiveInvoice(ctx, c, inlineReceive)
		return
	} else {
		// else, do an internal transaction
		bot.sendInlineReceiveHandler(ctx, c)
		return
	}
}

func (bot *TipBot) sendInlineReceiveHandler(ctx context.Context, c *tb.Callback) {
	tx := &InlineReceive{Base: transaction.New(transaction.ID(c.Data))}
	rn, err := tx.Get(tx, bot.Bunt)
	// immediatelly set intransaction to block duplicate calls
	if err != nil {
		log.Errorf("[getInlineReceive] %s", err)
		return
	}
	inlineReceive := rn.(*InlineReceive)
	err = inlineReceive.Lock(inlineReceive, bot.Bunt)
	if err != nil {
		log.Errorf("[acceptInlineReceiveHandler] %s", err)
		return
	}

	if !inlineReceive.Active {
		log.Errorf("[acceptInlineReceiveHandler] inline receive not active anymore")
		return
	}

	// defer inlineReceive.Release(inlineReceive, bot.Bunt)

	// from := inlineReceive.From
	from := LoadUser(ctx)
	to := inlineReceive.To
	toUserStr := GetUserStr(to.Telegram)
	fromUserStr := GetUserStr(from.Telegram)
	// balance check of the user
	balance, err := bot.GetUserBalanceCached(from)
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
	inlineReceive.Inactivate(inlineReceive, bot.Bunt)

	// todo: user new get username function to get userStrings
	transactionMemo := fmt.Sprintf("InlineReceive from %s to %s (%d sat).", fromUserStr, toUserStr, inlineReceive.Amount)
	t := NewTransaction(bot, from, to, inlineReceive.Amount, TransactionType("inline receive"))
	t.Memo = transactionMemo
	success, err := t.Send()
	if !success {
		errMsg := fmt.Sprintf("[acceptInlineReceiveHandler] Transaction failed: %s", err)
		log.Errorln(errMsg)
		bot.tryEditMessage(c.Message, i18n.Translate(inlineReceive.LanguageCode, "inlineReceiveFailedMessage"), &tb.ReplyMarkup{})
		return
	}

	log.Infof("[💸 inlineReceive] Send from %s to %s (%d sat).", fromUserStr, toUserStr, inlineReceive.Amount)
	inlineReceive.Release(inlineReceive, bot.Bunt)
	bot.finishInlineReceiveHandler(ctx, c)
}

func (bot *TipBot) inlineReceiveInvoice(ctx context.Context, c *tb.Callback, inlineReceive *InlineReceive) {
	if !inlineReceive.Active {
		log.Errorf("[acceptInlineReceiveHandler] inline receive not active anymore")
		return
	}
	invoice, err := bot.createInvoiceWithEvent(ctx, inlineReceive.To, inlineReceive.Amount, fmt.Sprintf("Pay to %s", GetUserStr(inlineReceive.To.Telegram)), InvoiceCallbackInlineReceive, inlineReceive.ID)
	if err != nil {
		errmsg := fmt.Sprintf("[/invoice] Could not create an invoice: %s", err)
		bot.tryEditMessage(inlineReceive.Message, Translate(ctx, "errorTryLaterMessage"))
		log.Errorln(errmsg)
		return
	}

	// create qr code
	qr, err := qrcode.Encode(invoice.PaymentRequest, qrcode.Medium, 256)
	if err != nil {
		errmsg := fmt.Sprintf("[/invoice] Failed to create QR code for invoice: %s", err)
		bot.tryEditMessage(inlineReceive.Message, Translate(ctx, "errorTryLaterMessage"))
		log.Errorln(errmsg)
		return
	}

	// send the invoice data to user
	var msg *tb.Message
	if inlineReceive.Message.Chat != nil {
		msg = bot.trySendMessage(inlineReceive.Message.Chat, &tb.Photo{File: tb.File{FileReader: bytes.NewReader(qr)}, Caption: fmt.Sprintf("`%s`", invoice.PaymentRequest)})
	} else {
		msg = bot.trySendMessage(c.Sender, &tb.Photo{File: tb.File{FileReader: bytes.NewReader(qr)}, Caption: fmt.Sprintf("`%s`", invoice.PaymentRequest)})
		bot.tryEditMessage(inlineReceive.Message, fmt.Sprintf("%s\n\nPay this invoice:\n```%s```", inlineReceive.MessageText, invoice.PaymentRequest))
	}
	invoice.InvoiceMessage = msg
	runtime.IgnoreError(bot.Bunt.Set(invoice))
	log.Printf("[/invoice] Incvoice created. User: %s, amount: %d sat.", GetUserStr(inlineReceive.To.Telegram), inlineReceive.Amount)

}
func (bot *TipBot) inlineReceiveEvent(invoiceEvent *InvoiceEvent) {
	bot.tryDeleteMessage(invoiceEvent.InvoiceMessage)
	bot.notifyInvoiceReceivedEvent(invoiceEvent)
	bot.finishInlineReceiveHandler(nil, &tb.Callback{Data: string(invoiceEvent.CallbackData)})
}

func (bot *TipBot) finishInlineReceiveHandler(ctx context.Context, c *tb.Callback) {
	tx := &InlineReceive{Base: transaction.New(transaction.ID(c.Data))}
	rn, err := tx.Get(tx, bot.Bunt)
	// immediatelly set intransaction to block duplicate calls
	if err != nil {
		log.Errorf("[getInlineReceive] %s", err)
		return
	}
	inlineReceive := rn.(*InlineReceive)

	from := inlineReceive.From
	to := inlineReceive.To
	toUserStrMd := GetUserStrMd(to.Telegram)
	fromUserStrMd := GetUserStrMd(from.Telegram)
	toUserStr := GetUserStr(to.Telegram)
	inlineReceive.MessageText = fmt.Sprintf("%s", fmt.Sprintf(i18n.Translate(inlineReceive.LanguageCode, "inlineSendUpdateMessageAccept"), inlineReceive.Amount, fromUserStrMd, toUserStrMd))
	memo := inlineReceive.Memo
	if len(memo) > 0 {
		inlineReceive.MessageText += fmt.Sprintf(i18n.Translate(inlineReceive.LanguageCode, "inlineReceiveAppendMemo"), memo)
	}

	if !to.Initialized {
		inlineReceive.MessageText += "\n\n" + fmt.Sprintf(i18n.Translate(inlineReceive.LanguageCode, "inlineSendCreateWalletMessage"), GetUserStrMd(bot.Telegram.Me))
	}

	bot.tryEditMessage(inlineReceive.Message, inlineReceive.MessageText, &tb.ReplyMarkup{})
	// notify users
	_, err = bot.Telegram.Send(to.Telegram, fmt.Sprintf(i18n.Translate(to.Telegram.LanguageCode, "sendReceivedMessage"), fromUserStrMd, inlineReceive.Amount))
	_, err = bot.Telegram.Send(from.Telegram, fmt.Sprintf(i18n.Translate(from.Telegram.LanguageCode, "sendSentMessage"), inlineReceive.Amount, toUserStrMd))
	if err != nil {
		errmsg := fmt.Errorf("[acceptInlineReceiveHandler] Error: Receive message to %s: %s", toUserStr, err)
		log.Warnln(errmsg)
	}
	// inlineReceive.Release(inlineReceive, bot.Bunt)
}

func (bot *TipBot) cancelInlineReceiveHandler(ctx context.Context, c *tb.Callback) {
	tx := &InlineReceive{Base: transaction.New(transaction.ID(c.Data))}
	rn, err := tx.Get(tx, bot.Bunt)
	// immediatelly set intransaction to block duplicate calls
	if err != nil {
		log.Errorf("[cancelInlineReceiveHandler] %s", err)
		return
	}
	inlineReceive := rn.(*InlineReceive)
	if c.Sender.ID == inlineReceive.To.Telegram.ID {
		bot.tryEditMessage(c.Message, i18n.Translate(inlineReceive.LanguageCode, "inlineReceiveCancelledMessage"), &tb.ReplyMarkup{})
		// set the inlineReceive inactive
		inlineReceive.Active = false
		inlineReceive.InTransaction = false
		runtime.IgnoreError(inlineReceive.Set(inlineReceive, bot.Bunt))
	}
	return
}
