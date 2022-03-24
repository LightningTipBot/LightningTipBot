package telegram

import (
	"context"
	"fmt"
	"github.com/LightningTipBot/LightningTipBot/internal/telegram/intercept"
	"strings"
	"time"

	"github.com/LightningTipBot/LightningTipBot/internal/runtime/mutex"
	"github.com/LightningTipBot/LightningTipBot/internal/storage"

	"github.com/eko/gocache/store"

	"github.com/LightningTipBot/LightningTipBot/internal/errors"
	"github.com/LightningTipBot/LightningTipBot/internal/i18n"
	"github.com/LightningTipBot/LightningTipBot/internal/lnbits"

	log "github.com/sirupsen/logrus"
	tb "gopkg.in/telebot.v3"
)

var (
	inlineTipjarMenu      = &tb.ReplyMarkup{ResizeKeyboard: false}
	btnCancelInlineTipjar = inlineTipjarMenu.Data("🚫", "cancel_tipjar_inline")
	btnAcceptInlineTipjar = inlineTipjarMenu.Data("💸 Pay", "confirm_tipjar_inline")
)

type InlineTipjar struct {
	*storage.Base
	Message       string         `json:"inline_tipjar_message"`
	Amount        int64          `json:"inline_tipjar_amount"`
	GivenAmount   int64          `json:"inline_tipjar_givenamount"`
	PerUserAmount int64          `json:"inline_tipjar_peruseramount"`
	To            *lnbits.User   `json:"inline_tipjar_to"`
	From          []*lnbits.User `json:"inline_tipjar_from"`
	Memo          string         `json:"inline_tipjar_memo"`
	NTotal        int            `json:"inline_tipjar_ntotal"`
	NGiven        int            `json:"inline_tipjar_ngiven"`
	LanguageCode  string         `json:"languagecode"`
}

func (bot TipBot) mapTipjarLanguage(ctx context.Context, command string) context.Context {
	if len(strings.Split(command, " ")) > 1 {
		c := strings.Split(command, " ")[0][1:] // cut the /
		ctx = bot.commandTranslationMap(ctx, c)
	}
	return ctx
}

func (bot TipBot) createTipjar(ctx context.Context, text string, sender *tb.User) (*InlineTipjar, error) {
	amount, err := decodeAmountFromCommand(text)
	if err != nil {
		return nil, errors.New(errors.DecodeAmountError, err)
	}
	peruserStr, err := getArgumentFromCommand(text, 2)
	if err != nil {
		return nil, errors.New(errors.DecodePerUserAmountError, err)
	}
	perUserAmount, err := getAmount(peruserStr)
	if err != nil {
		return nil, errors.New(errors.InvalidAmountError, err)
	}
	if perUserAmount < 1 || amount%perUserAmount != 0 {
		return nil, errors.New(errors.InvalidAmountPerUserError, fmt.Errorf("invalid amount per user"))
	}
	nTotal := int(amount / perUserAmount)
	toUser := LoadUser(ctx)
	// toUserStr := GetUserStr(sender)
	// // check for memo in command
	memo := GetMemoFromCommand(text, 3)

	inlineMessage := fmt.Sprintf(
		Translate(ctx, "inlineTipjarMessage"),
		perUserAmount,
		GetUserStrMd(toUser.Telegram),
		0,
		amount,
		0,
		MakeTipjarbar(0, amount),
	)
	if len(memo) > 0 {
		inlineMessage = inlineMessage + fmt.Sprintf(Translate(ctx, "inlineTipjarAppendMemo"), memo)
	}
	id := fmt.Sprintf("tipjar:%s:%d", RandStringRunes(10), amount)

	return &InlineTipjar{
		Base:          storage.New(storage.ID(id)),
		Message:       inlineMessage,
		Amount:        amount,
		To:            toUser,
		Memo:          memo,
		PerUserAmount: perUserAmount,
		NTotal:        nTotal,
		NGiven:        0,
		GivenAmount:   0,
		LanguageCode:  ctx.Value("publicLanguageCode").(string),
	}, nil

}
func (bot TipBot) makeTipjar(ctx context.Context, m *tb.Message, query bool) (*InlineTipjar, error) {
	tipjar, err := bot.createTipjar(ctx, m.Text, m.Sender)
	if err != nil {
		switch err.(errors.TipBotError).Code {
		case errors.DecodeAmountError:
			bot.trySendMessage(m.Sender, fmt.Sprintf(Translate(ctx, "inlineTipjarHelpText"), Translate(ctx, "inlineTipjarInvalidAmountMessage")))
			bot.tryDeleteMessage(m)
			return nil, err
		case errors.DecodePerUserAmountError:
			bot.trySendMessage(m.Sender, fmt.Sprintf(Translate(ctx, "inlineTipjarHelpText"), ""))
			bot.tryDeleteMessage(m)
			return nil, err
		case errors.InvalidAmountError:
			bot.trySendMessage(m.Sender, fmt.Sprintf(Translate(ctx, "inlineTipjarHelpText"), Translate(ctx, "inlineTipjarInvalidAmountMessage")))
			bot.tryDeleteMessage(m)
			return nil, err
		case errors.InvalidAmountPerUserError:
			bot.trySendMessage(m.Sender, fmt.Sprintf(Translate(ctx, "inlineTipjarHelpText"), Translate(ctx, "inlineTipjarInvalidPeruserAmountMessage")))
			bot.tryDeleteMessage(m)
			return nil, err
		case errors.GetBalanceError:
			// log.Errorln(err.Error())
			bot.tryDeleteMessage(m)
			return nil, err
		case errors.BalanceToLowError:
			// log.Errorf(err.Error())
			bot.trySendMessage(m.Sender, Translate(ctx, "inlineSendBalanceLowMessage"))
			bot.tryDeleteMessage(m)
			return nil, err
		}
	}
	return tipjar, err
}

func (bot TipBot) makeQueryTipjar(ctx context.Context, q *tb.Query, query bool) (*InlineTipjar, error) {
	tipjar, err := bot.createTipjar(ctx, q.Text, q.Sender)
	if err != nil {
		switch err.(errors.TipBotError).Code {
		case errors.DecodeAmountError:
			bot.inlineQueryReplyWithError(q, TranslateUser(ctx, "inlineQueryTipjarTitle"), fmt.Sprintf(TranslateUser(ctx, "inlineQueryTipjarDescription"), bot.Telegram.Me.Username))
			return nil, err
		case errors.DecodePerUserAmountError:
			bot.inlineQueryReplyWithError(q, TranslateUser(ctx, "inlineQueryTipjarTitle"), fmt.Sprintf(TranslateUser(ctx, "inlineQueryTipjarDescription"), bot.Telegram.Me.Username))
			return nil, err
		case errors.InvalidAmountError:
			bot.inlineQueryReplyWithError(q, TranslateUser(ctx, "inlineSendInvalidAmountMessage"), fmt.Sprintf(TranslateUser(ctx, "inlineQueryTipjarDescription"), bot.Telegram.Me.Username))
			return nil, err
		case errors.InvalidAmountPerUserError:
			bot.inlineQueryReplyWithError(q, TranslateUser(ctx, "inlineTipjarInvalidPeruserAmountMessage"), fmt.Sprintf(TranslateUser(ctx, "inlineQueryTipjarDescription"), bot.Telegram.Me.Username))
			return nil, err
		case errors.GetBalanceError:
			bot.inlineQueryReplyWithError(q, TranslateUser(ctx, "inlineQueryTipjarTitle"), fmt.Sprintf(TranslateUser(ctx, "inlineQueryTipjarDescription"), bot.Telegram.Me.Username))
			return nil, err
		case errors.BalanceToLowError:
			log.Errorf(err.Error())
			bot.inlineQueryReplyWithError(q, TranslateUser(ctx, "inlineSendBalanceLowMessage"), fmt.Sprintf(TranslateUser(ctx, "inlineQueryTipjarDescription"), bot.Telegram.Me.Username))
			return nil, err
		}
	}
	return tipjar, err
}

func (bot TipBot) makeTipjarKeyboard(ctx context.Context, inlineTipjar *InlineTipjar) *tb.ReplyMarkup {
	inlineTipjarMenu := &tb.ReplyMarkup{ResizeKeyboard: true}
	// slice of buttons
	buttons := make([]tb.Btn, 0)
	cancelInlineTipjarButton := inlineTipjarMenu.Data(Translate(ctx, "cancelButtonMessage"), "cancel_tipjar_inline", inlineTipjar.ID)
	buttons = append(buttons, cancelInlineTipjarButton)
	acceptInlineTipjarButton := inlineTipjarMenu.Data(Translate(ctx, "payReceiveButtonMessage"), "confirm_tipjar_inline", inlineTipjar.ID)
	buttons = append(buttons, acceptInlineTipjarButton)

	inlineTipjarMenu.Inline(
		inlineTipjarMenu.Row(buttons...))
	return inlineTipjarMenu
}

func (bot TipBot) tipjarHandler(handler intercept.Handler) (intercept.Handler, error) {
	m := handler.Message()
	bot.anyTextHandler(handler)
	if m.Private() {
		bot.trySendMessage(m.Sender, fmt.Sprintf(Translate(handler.Ctx, "inlineTipjarHelpText"), Translate(handler.Ctx, "inlineTipjarHelpTipjarInGroup")))
		return handler, errors.Create(errors.NoPrivateChatError)
	}
	handler.Ctx = bot.mapTipjarLanguage(handler.Ctx, m.Text)
	inlineTipjar, err := bot.makeTipjar(handler.Ctx, m, false)
	if err != nil {
		log.Errorf("[tipjar] %s", err.Error())
		return handler, err
	}
	toUserStr := GetUserStr(m.Sender)
	bot.trySendMessage(m.Chat, inlineTipjar.Message, bot.makeTipjarKeyboard(handler.Ctx, inlineTipjar))
	log.Infof("[tipjar] %s created tipjar %s: %d sat (%d per user)", toUserStr, inlineTipjar.ID, inlineTipjar.Amount, inlineTipjar.PerUserAmount)
	return handler, inlineTipjar.Set(inlineTipjar, bot.Bunt)
}

func (bot TipBot) handleInlineTipjarQuery(handler intercept.Handler) (intercept.Handler, error) {
	q := handler.Query()
	inlineTipjar, err := bot.makeQueryTipjar(handler.Ctx, q, false)
	if err != nil {
		// log.Errorf("[tipjar] %s", err.Error())
		return handler, err
	}
	urls := []string{
		queryImage,
	}
	results := make(tb.Results, len(urls)) // []tb.Result
	for i, url := range urls {
		result := &tb.ArticleResult{
			// URL:         url,
			Text:        inlineTipjar.Message,
			Title:       fmt.Sprintf(TranslateUser(handler.Ctx, "inlineResultTipjarTitle"), inlineTipjar.Amount),
			Description: TranslateUser(handler.Ctx, "inlineResultTipjarDescription"),
			// required for photos
			ThumbURL: url,
		}
		result.ReplyMarkup = &tb.ReplyMarkup{InlineKeyboard: bot.makeTipjarKeyboard(handler.Ctx, inlineTipjar).InlineKeyboard}
		results[i] = result
		// needed to set a unique string ID for each result
		results[i].SetResultID(inlineTipjar.ID)

		bot.Cache.Set(inlineTipjar.ID, inlineTipjar, &store.Options{Expiration: 5 * time.Minute})
		log.Infof("[tipjar] %s created inline tipjar %s: %d sat (%d per user)", GetUserStr(inlineTipjar.To.Telegram), inlineTipjar.ID, inlineTipjar.Amount, inlineTipjar.PerUserAmount)
	}

	err = bot.Telegram.Answer(q, &tb.QueryResponse{
		Results:    results,
		CacheTime:  1,
		IsPersonal: true,
	})
	if err != nil {
		log.Errorln(err)
		return handler, err
	}
	return handler, nil
}

func (bot *TipBot) acceptInlineTipjarHandler(handler intercept.Handler) (intercept.Handler, error) {
	c := handler.Callback()
	from := LoadUser(handler.Ctx)
	if from.Wallet == nil {
		return handler, errors.Create(errors.UserNoWalletError)
	}
	tx := &InlineTipjar{Base: storage.New(storage.ID(c.Data))}
	mutex.LockWithContext(handler.Ctx, tx.ID)
	defer mutex.UnlockWithContext(handler.Ctx, tx.ID)
	fn, err := tx.Get(tx, bot.Bunt)
	if err != nil {
		// log.Errorf("[tipjar] %s", err.Error())
		return handler, err
	}
	inlineTipjar := fn.(*InlineTipjar)
	to := inlineTipjar.To
	if !inlineTipjar.Active {
		log.Errorf(fmt.Sprintf("[tipjar] tipjar %s inactive.", inlineTipjar.ID))
		bot.tryEditMessage(c.Message, i18n.Translate(inlineTipjar.LanguageCode, "inlineTipjarCancelledMessage"), &tb.ReplyMarkup{})
		return handler, errors.Create(errors.NotActiveError)
	}

	if from.Telegram.ID == to.Telegram.ID {
		bot.trySendMessage(from.Telegram, Translate(handler.Ctx, "sendYourselfMessage"))
		return handler, errors.Create(errors.SelfPaymentError)
	}
	// // check if to user has already given to the tipjar
	for _, a := range inlineTipjar.From {
		if a.Telegram.ID == from.Telegram.ID {
			// to user is already in To slice, has taken from facuet
			// log.Infof("[tipjar] %s already gave to tipjar %s", GetUserStr(to.Telegram), inlineTipjar.ID)
			return handler, errors.Create(errors.UnknownError)
		}
	}

	defer inlineTipjar.Set(inlineTipjar, bot.Bunt)

	if inlineTipjar.GivenAmount < inlineTipjar.Amount {
		toUserStrMd := GetUserStrMd(to.Telegram)
		fromUserStrMd := GetUserStrMd(from.Telegram)
		toUserStr := GetUserStr(to.Telegram)
		fromUserStr := GetUserStr(from.Telegram)

		// todo: user new get username function to get userStrings
		transactionMemo := fmt.Sprintf("Tipjar from %s to %s (%d sat).", fromUserStr, toUserStr, inlineTipjar.PerUserAmount)
		t := NewTransaction(bot, from, to, inlineTipjar.PerUserAmount, TransactionType("tipjar"))
		t.Memo = transactionMemo

		success, err := t.Send()
		if !success {
			bot.trySendMessage(from.Telegram, Translate(handler.Ctx, "sendErrorMessage"))
			errMsg := fmt.Sprintf("[tipjar] Transaction failed: %s", err.Error())
			log.Errorln(errMsg)
			return handler, errors.New(errors.UnknownError, err)
		}

		log.Infof("[💸 tipjar] Tipjar %s from %s to %s (%d sat).", inlineTipjar.ID, fromUserStr, toUserStr, inlineTipjar.PerUserAmount)
		inlineTipjar.NGiven += 1
		inlineTipjar.From = append(inlineTipjar.From, from)
		inlineTipjar.GivenAmount = inlineTipjar.GivenAmount + inlineTipjar.PerUserAmount

		bot.trySendMessage(to.Telegram, fmt.Sprintf(i18n.Translate(to.Telegram.LanguageCode, "inlineTipjarReceivedMessage"), fromUserStrMd, inlineTipjar.PerUserAmount))
		bot.trySendMessage(from.Telegram, fmt.Sprintf(i18n.Translate(from.Telegram.LanguageCode, "inlineTipjarSentMessage"), inlineTipjar.PerUserAmount, toUserStrMd))
		if err != nil {
			errmsg := fmt.Errorf("[tipjar] Error: Send message to %s: %s", toUserStr, err)
			log.Warnln(errmsg)
		}

		// build tipjar message
		inlineTipjar.Message = fmt.Sprintf(
			i18n.Translate(inlineTipjar.LanguageCode, "inlineTipjarMessage"),
			inlineTipjar.PerUserAmount,
			GetUserStrMd(inlineTipjar.To.Telegram),
			inlineTipjar.GivenAmount,
			inlineTipjar.Amount,
			inlineTipjar.NGiven,
			MakeTipjarbar(inlineTipjar.GivenAmount, inlineTipjar.Amount),
		)
		memo := inlineTipjar.Memo
		if len(memo) > 0 {
			inlineTipjar.Message = inlineTipjar.Message + fmt.Sprintf(i18n.Translate(inlineTipjar.LanguageCode, "inlineTipjarAppendMemo"), memo)
		}
		// update message
		log.Infoln(inlineTipjar.Message)
		bot.tryEditMessage(c.Message, inlineTipjar.Message, bot.makeTipjarKeyboard(handler.Ctx, inlineTipjar))
	}
	if inlineTipjar.GivenAmount >= inlineTipjar.Amount {
		// tipjar is full
		inlineTipjar.Message = fmt.Sprintf(
			i18n.Translate(inlineTipjar.LanguageCode, "inlineTipjarEndedMessage"),
			GetUserStrMd(inlineTipjar.To.Telegram),
			inlineTipjar.Amount,
			inlineTipjar.NGiven,
		)
		bot.tryEditMessage(c.Message, inlineTipjar.Message)
		// send update to tipjar creator
		if inlineTipjar.Active && inlineTipjar.To.Telegram.ID != 0 {
			bot.trySendMessage(inlineTipjar.To.Telegram, listTipjarGivers(inlineTipjar))
		}
		inlineTipjar.Active = false
	}
	return handler, nil

}

func (bot *TipBot) cancelInlineTipjarHandler(handler intercept.Handler) (intercept.Handler, error) {
	c := handler.Callback()
	tx := &InlineTipjar{Base: storage.New(storage.ID(c.Data))}
	mutex.LockWithContext(handler.Ctx, tx.ID)
	defer mutex.UnlockWithContext(handler.Ctx, tx.ID)
	fn, err := tx.Get(tx, bot.Bunt)
	if err != nil {
		log.Errorf("[cancelInlineTipjarHandler] %s", err.Error())
		return handler, err
	}
	inlineTipjar := fn.(*InlineTipjar)
	if c.Sender.ID != inlineTipjar.To.Telegram.ID {
		return handler, errors.Create(errors.UnknownError)
	}
	bot.tryEditMessage(c.Message, i18n.Translate(inlineTipjar.LanguageCode, "inlineTipjarCancelledMessage"), &tb.ReplyMarkup{})

	// send update to tipjar creator
	if inlineTipjar.Active && inlineTipjar.To.Telegram.ID != 0 {
		bot.trySendMessage(inlineTipjar.To.Telegram, listTipjarGivers(inlineTipjar))
	}

	// set the inlineTipjar inactive
	inlineTipjar.Active = false
	return handler, inlineTipjar.Set(inlineTipjar, bot.Bunt)
}

func listTipjarGivers(inlineTipjar *InlineTipjar) string {
	var from_str string
	from_str = fmt.Sprintf("🍯 *Tipjar summary*\n\nMemo: %s\nCapacity: %d sat\nGivers: %d\nCollected: %d sat\n\n*Givers:*\n\n", inlineTipjar.Memo, inlineTipjar.Amount, inlineTipjar.NGiven, inlineTipjar.GivenAmount)
	from_str += "```\n"
	for _, from := range inlineTipjar.From {
		from_str += fmt.Sprintf("%s\n", GetUserStr(from.Telegram))
	}
	from_str += "```"
	return from_str
}
