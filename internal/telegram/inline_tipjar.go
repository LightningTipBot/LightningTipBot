package telegram

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/eko/gocache/store"

	"github.com/LightningTipBot/LightningTipBot/internal/errors"
	"github.com/LightningTipBot/LightningTipBot/internal/i18n"
	"github.com/LightningTipBot/LightningTipBot/internal/lnbits"
	"github.com/LightningTipBot/LightningTipBot/internal/storage/transaction"

	"github.com/LightningTipBot/LightningTipBot/internal/runtime"
	log "github.com/sirupsen/logrus"
	tb "gopkg.in/lightningtipbot/telebot.v2"
)

var (
	inlineTipjarMenu      = &tb.ReplyMarkup{ResizeReplyKeyboard: false}
	btnCancelInlineTipjar = inlineTipjarMenu.Data("🚫", "cancel_tipjar_inline")
	btnAcceptInlineTipjar = inlineTipjarMenu.Data("💸 Pay", "confirm_tipjar_inline")
)

type InlineTipjar struct {
	*transaction.Base
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
	id := fmt.Sprintf("inl-tipjar-%d-%d-%s", sender.ID, amount, RandStringRunes(5))

	return &InlineTipjar{
		Base:          transaction.New(transaction.ID(id)),
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
	tipjar, err := bot.createTipjar(ctx, q.Text, &q.From)
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
	// inlineTipjarMenu := &tb.ReplyMarkup{ResizeReplyKeyboard: true}
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

func (bot TipBot) tipjarHandler(ctx context.Context, m *tb.Message) {
	bot.anyTextHandler(ctx, m)
	if m.Private() {
		bot.trySendMessage(m.Sender, fmt.Sprintf(Translate(ctx, "inlineTipjarHelpText"), Translate(ctx, "inlineTipjarHelpTipjarInGroup")))
		return
	}
	ctx = bot.mapTipjarLanguage(ctx, m.Text)
	inlineTipjar, err := bot.makeTipjar(ctx, m, false)
	if err != nil {
		log.Errorf("[tipjar] %s", err)
		return
	}
	toUserStr := GetUserStr(m.Sender)
	bot.trySendMessage(m.Chat, inlineTipjar.Message, bot.makeTipjarKeyboard(ctx, inlineTipjar))
	log.Infof("[tipjar] %s created tipjar %s: %d sat (%d per user)", toUserStr, inlineTipjar.ID, inlineTipjar.Amount, inlineTipjar.PerUserAmount)
	runtime.IgnoreError(inlineTipjar.Set(inlineTipjar, bot.Bunt))
}

func (bot TipBot) handleInlineTipjarQuery(ctx context.Context, q *tb.Query) {
	inlineTipjar, err := bot.makeQueryTipjar(ctx, q, false)
	if err != nil {
		// log.Errorf("[tipjar] %s", err)
		return
	}
	urls := []string{
		queryImage,
	}
	results := make(tb.Results, len(urls)) // []tb.Result
	for i, url := range urls {
		result := &tb.ArticleResult{
			// URL:         url,
			Text:        inlineTipjar.Message,
			Title:       fmt.Sprintf(TranslateUser(ctx, "inlineResultTipjarTitle"), inlineTipjar.Amount),
			Description: TranslateUser(ctx, "inlineResultTipjarDescription"),
			// required for photos
			ThumbURL: url,
		}
		result.ReplyMarkup = &tb.InlineKeyboardMarkup{InlineKeyboard: bot.makeTipjarKeyboard(ctx, inlineTipjar).InlineKeyboard}
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
	}
}

func (bot *TipBot) acceptInlineTipjarHandler(ctx context.Context, c *tb.Callback) {
	from := LoadUser(ctx)
	if from.Wallet == nil {
		return
	}
	tx := &InlineTipjar{Base: transaction.New(transaction.ID(c.Data))}
	fn, err := tx.Get(tx, bot.Bunt)
	if err != nil {
		// log.Errorf("[tipjar] %s", err)
		return
	}
	inlineTipjar := fn.(*InlineTipjar)
	to := inlineTipjar.To
	err = inlineTipjar.Lock(inlineTipjar, bot.Bunt)
	if err != nil {
		log.Errorf("[tipjar] LockTipjar %s error: %s", inlineTipjar.ID, err)
		return
	}
	if !inlineTipjar.Active {
		log.Errorf(fmt.Sprintf("[tipjar] tipjar %s inactive.", inlineTipjar.ID))
		return
	}
	// release tipjar no matter what
	defer inlineTipjar.Release(inlineTipjar, bot.Bunt)

	if from.Telegram.ID == to.Telegram.ID {
		bot.trySendMessage(from.Telegram, Translate(ctx, "sendYourselfMessage"))
		return
	}
	// // check if to user has already given to the tipjar
	for _, a := range inlineTipjar.From {
		if a.Telegram.ID == from.Telegram.ID {
			// to user is already in To slice, has taken from facuet
			// log.Infof("[tipjar] %s already gave to tipjar %s", GetUserStr(to.Telegram), inlineTipjar.ID)
			return
		}
	}
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
			bot.trySendMessage(from.Telegram, Translate(ctx, "sendErrorMessage"))
			errMsg := fmt.Sprintf("[tipjar] Transaction failed: %s", err)
			log.Errorln(errMsg)
			return
		}

		log.Infof("[💸 tipjar] Tipjar %s from %s to %s (%d sat).", inlineTipjar.ID, fromUserStr, toUserStr, inlineTipjar.PerUserAmount)
		inlineTipjar.NGiven += 1
		inlineTipjar.From = append(inlineTipjar.From, from)
		inlineTipjar.GivenAmount = inlineTipjar.GivenAmount + inlineTipjar.PerUserAmount

		_, err = bot.Telegram.Send(to.Telegram, fmt.Sprintf(i18n.Translate(to.Telegram.LanguageCode, "inlineTipjarReceivedMessage"), fromUserStrMd, inlineTipjar.PerUserAmount))
		_, err = bot.Telegram.Send(from.Telegram, fmt.Sprintf(i18n.Translate(from.Telegram.LanguageCode, "inlineTipjarSentMessage"), inlineTipjar.PerUserAmount, toUserStrMd))
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
		// if inlineTipjar.UserNeedsWallet {
		// 	inlineTipjar.Message += "\n\n" + fmt.Sprintf(i18n.Translate(inlineTipjar.LanguageCode, "inlineTipjarCreateWalletMessage"), GetUserStrMd(bot.Telegram.Me))
		// }
		// update message
		log.Infoln(inlineTipjar.Message)
		bot.tryEditMessage(c.Message, inlineTipjar.Message, bot.makeTipjarKeyboard(ctx, inlineTipjar))
	}
	if inlineTipjar.GivenAmount >= inlineTipjar.Amount {
		// tipjar is full
		inlineTipjar.Message = fmt.Sprintf(
			i18n.Translate(inlineTipjar.LanguageCode, "inlineTipjarEndedMessage"),
			GetUserStrMd(inlineTipjar.To.Telegram),
			inlineTipjar.Amount,
			inlineTipjar.NGiven,
		)
		// if inlineTipjar.UserNeedsWallet {
		// 	inlineTipjar.Message += "\n\n" + fmt.Sprintf(i18n.Translate(inlineTipjar.LanguageCode, "inlineTipjarCreateWalletMessage"), GetUserStrMd(bot.Telegram.Me))
		// }
		bot.tryEditMessage(c.Message, inlineTipjar.Message)
		inlineTipjar.Active = false
	}

}

func (bot *TipBot) cancelInlineTipjarHandler(ctx context.Context, c *tb.Callback) {
	tx := &InlineTipjar{Base: transaction.New(transaction.ID(c.Data))}
	fn, err := tx.Get(tx, bot.Bunt)
	if err != nil {
		log.Errorf("[cancelInlineTipjarHandler] %s", err)
		return
	}
	defer transaction.Unlock(tx.ID)
	inlineTipjar := fn.(*InlineTipjar)
	if c.Sender.ID == inlineTipjar.To.Telegram.ID {
		bot.tryEditMessage(c.Message, i18n.Translate(inlineTipjar.LanguageCode, "inlineTipjarCancelledMessage"), &tb.ReplyMarkup{})
		// set the inlineTipjar inactive
		inlineTipjar.Active = false
		inlineTipjar.InTransaction = false
		runtime.IgnoreError(inlineTipjar.Set(inlineTipjar, bot.Bunt))
	}
	return
}
