package telegram

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/LightningTipBot/LightningTipBot/internal/runtime/mutex"
	"github.com/LightningTipBot/LightningTipBot/internal/runtime/once"
	"github.com/LightningTipBot/LightningTipBot/internal/storage"

	"github.com/eko/gocache/store"

	"github.com/LightningTipBot/LightningTipBot/internal/errors"
	"github.com/LightningTipBot/LightningTipBot/internal/i18n"

	"github.com/LightningTipBot/LightningTipBot/internal/lnbits"

	"github.com/LightningTipBot/LightningTipBot/internal/runtime"
	log "github.com/sirupsen/logrus"
	tb "gopkg.in/lightningtipbot/telebot.v2"
)

var (
	inlineFaucetMenu      = &tb.ReplyMarkup{ResizeReplyKeyboard: true}
	btnCancelInlineFaucet = inlineFaucetMenu.Data("🚫 Cancel", "cancel_faucet_inline")
	btnAcceptInlineFaucet = inlineFaucetMenu.Data("✅ Collect", "confirm_faucet_inline")
)

type InlineFaucet struct {
	*storage.Base
	Message         string         `json:"inline_faucet_message"`
	Amount          int64          `json:"inline_faucet_amount"`
	RemainingAmount int64          `json:"inline_faucet_remainingamount"`
	PerUserAmount   int64          `json:"inline_faucet_peruseramount"`
	From            *lnbits.User   `json:"inline_faucet_from"`
	To              []*lnbits.User `json:"inline_faucet_to"`
	Memo            string         `json:"inline_faucet_memo"`
	NTotal          int            `json:"inline_faucet_ntotal"`
	NTaken          int            `json:"inline_faucet_ntaken"`
	UserNeedsWallet bool           `json:"inline_faucet_userneedswallet"`
	LanguageCode    string         `json:"languagecode"`
}

func (bot TipBot) mapFaucetLanguage(ctx context.Context, command string) context.Context {
	if len(strings.Split(command, " ")) > 1 {
		c := strings.Split(command, " ")[0][1:] // cut the /
		ctx = bot.commandTranslationMap(ctx, c)
	}
	return ctx
}

func (bot TipBot) createFaucet(ctx context.Context, text string, sender *tb.User) (*InlineFaucet, error) {
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
	if perUserAmount < 5 || amount%perUserAmount != 0 {
		return nil, errors.New(errors.InvalidAmountPerUserError, fmt.Errorf("invalid amount per user"))
	}
	nTotal := int(amount / perUserAmount)
	fromUser := LoadUser(ctx)
	fromUserStr := GetUserStr(sender)
	balance, err := bot.GetUserBalanceCached(fromUser)
	if err != nil {
		return nil, errors.New(errors.GetBalanceError, err)
	}
	// check if fromUser has balance
	if balance < amount {
		return nil, errors.New(errors.BalanceToLowError, fmt.Errorf("[faucet] Balance of user %s too low", fromUserStr))
	}
	// // check for memo in command
	memo := GetMemoFromCommand(text, 3)

	inlineMessage := fmt.Sprintf(Translate(ctx, "inlineFaucetMessage"), perUserAmount, GetUserStrMd(sender), amount, amount, 0, nTotal, MakeProgressbar(amount, amount))
	if len(memo) > 0 {
		inlineMessage = inlineMessage + fmt.Sprintf(Translate(ctx, "inlineFaucetAppendMemo"), memo)
	}
	id := fmt.Sprintf("faucet:%s:%d", RandStringRunes(10), amount)

	return &InlineFaucet{
		Base:            storage.New(storage.ID(id)),
		Message:         inlineMessage,
		Amount:          amount,
		From:            fromUser,
		Memo:            memo,
		PerUserAmount:   perUserAmount,
		NTotal:          nTotal,
		NTaken:          0,
		RemainingAmount: amount,
		UserNeedsWallet: false,
		LanguageCode:    ctx.Value("publicLanguageCode").(string),
	}, nil

}
func (bot TipBot) makeFaucet(ctx context.Context, m *tb.Message, query bool) (*InlineFaucet, error) {
	faucet, err := bot.createFaucet(ctx, m.Text, m.Sender)
	if err != nil {
		switch err.(errors.TipBotError).Code {
		case errors.DecodeAmountError:
			bot.trySendMessage(m.Sender, fmt.Sprintf(Translate(ctx, "inlineFaucetHelpText"), Translate(ctx, "inlineFaucetInvalidAmountMessage")))
			bot.tryDeleteMessage(m)
			return nil, err
		case errors.DecodePerUserAmountError:
			bot.trySendMessage(m.Sender, fmt.Sprintf(Translate(ctx, "inlineFaucetHelpText"), ""))
			bot.tryDeleteMessage(m)
			return nil, err
		case errors.InvalidAmountError:
			bot.trySendMessage(m.Sender, fmt.Sprintf(Translate(ctx, "inlineFaucetHelpText"), Translate(ctx, "inlineFaucetInvalidAmountMessage")))
			bot.tryDeleteMessage(m)
			return nil, err
		case errors.InvalidAmountPerUserError:
			bot.trySendMessage(m.Sender, fmt.Sprintf(Translate(ctx, "inlineFaucetHelpText"), Translate(ctx, "inlineFaucetInvalidPeruserAmountMessage")))
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
	return faucet, err
}

func (bot TipBot) makeQueryFaucet(ctx context.Context, q *tb.Query, query bool) (*InlineFaucet, error) {
	faucet, err := bot.createFaucet(ctx, q.Text, &q.From)
	if err != nil {
		switch err.(errors.TipBotError).Code {
		case errors.DecodeAmountError:
			bot.inlineQueryReplyWithError(q, TranslateUser(ctx, "inlineQueryFaucetTitle"), fmt.Sprintf(TranslateUser(ctx, "inlineQueryFaucetDescription"), bot.Telegram.Me.Username))
			return nil, err
		case errors.DecodePerUserAmountError:
			bot.inlineQueryReplyWithError(q, TranslateUser(ctx, "inlineQueryFaucetTitle"), fmt.Sprintf(TranslateUser(ctx, "inlineQueryFaucetDescription"), bot.Telegram.Me.Username))
			return nil, err
		case errors.InvalidAmountError:
			bot.inlineQueryReplyWithError(q, TranslateUser(ctx, "inlineSendInvalidAmountMessage"), fmt.Sprintf(TranslateUser(ctx, "inlineQueryFaucetDescription"), bot.Telegram.Me.Username))
			return nil, err
		case errors.InvalidAmountPerUserError:
			bot.inlineQueryReplyWithError(q, TranslateUser(ctx, "inlineFaucetInvalidPeruserAmountMessage"), fmt.Sprintf(TranslateUser(ctx, "inlineQueryFaucetDescription"), bot.Telegram.Me.Username))
			return nil, err
		case errors.GetBalanceError:
			bot.inlineQueryReplyWithError(q, TranslateUser(ctx, "inlineQueryFaucetTitle"), fmt.Sprintf(TranslateUser(ctx, "inlineQueryFaucetDescription"), bot.Telegram.Me.Username))
			return nil, err
		case errors.BalanceToLowError:
			log.Errorf(err.Error())
			bot.inlineQueryReplyWithError(q, TranslateUser(ctx, "inlineSendBalanceLowMessage"), fmt.Sprintf(TranslateUser(ctx, "inlineQueryFaucetDescription"), bot.Telegram.Me.Username))
			return nil, err
		}
	}
	return faucet, err
}

func (bot TipBot) makeFaucetKeyboard(ctx context.Context, id string) *tb.ReplyMarkup {
	inlineFaucetMenu := &tb.ReplyMarkup{ResizeReplyKeyboard: true}
	acceptInlineFaucetButton := inlineFaucetMenu.Data(Translate(ctx, "collectButtonMessage"), "confirm_faucet_inline", id)
	cancelInlineFaucetButton := inlineFaucetMenu.Data(Translate(ctx, "cancelButtonMessage"), "cancel_faucet_inline", id)
	inlineFaucetMenu.Inline(
		inlineFaucetMenu.Row(
			acceptInlineFaucetButton,
			cancelInlineFaucetButton),
	)
	return inlineFaucetMenu
}

func (bot TipBot) faucetHandler(ctx context.Context, m *tb.Message) {
	bot.anyTextHandler(ctx, m)
	if m.Private() {
		bot.trySendMessage(m.Sender, fmt.Sprintf(Translate(ctx, "inlineFaucetHelpText"), Translate(ctx, "inlineFaucetHelpFaucetInGroup")))
		return
	}
	ctx = bot.mapFaucetLanguage(ctx, m.Text)
	inlineFaucet, err := bot.makeFaucet(ctx, m, false)
	if err != nil {
		log.Warnf("[faucet] %s", err.Error())
		return
	}
	fromUserStr := GetUserStr(m.Sender)
	mFaucet := bot.trySendMessage(m.Chat, inlineFaucet.Message, bot.makeFaucetKeyboard(ctx, inlineFaucet.ID))
	log.Infof("[faucet] %s created faucet %s: %d sat (%d per user)", fromUserStr, inlineFaucet.ID, inlineFaucet.Amount, inlineFaucet.PerUserAmount)

	// log faucet link if possible
	if mFaucet != nil && mFaucet.Chat != nil {
		log.Infof("[faucet] Link: https://t.me/c/%s/%d", strconv.FormatInt(mFaucet.Chat.ID, 10)[4:], mFaucet.ID)
	}

	runtime.IgnoreError(inlineFaucet.Set(inlineFaucet, bot.Bunt))
}

func (bot TipBot) handleInlineFaucetQuery(ctx context.Context, q *tb.Query) {
	inlineFaucet, err := bot.makeQueryFaucet(ctx, q, false)
	if err != nil {
		log.Errorf("[handleInlineFaucetQuery] %s", err.Error())
		return
	}
	urls := []string{
		queryImage,
	}
	results := make(tb.Results, len(urls)) // []tb.Result
	for i, url := range urls {
		result := &tb.ArticleResult{
			// URL:         url,
			Text:        inlineFaucet.Message,
			Title:       fmt.Sprintf(TranslateUser(ctx, "inlineResultFaucetTitle"), inlineFaucet.Amount),
			Description: TranslateUser(ctx, "inlineResultFaucetDescription"),
			// required for photos
			ThumbURL: url,
		}
		result.ReplyMarkup = &tb.InlineKeyboardMarkup{InlineKeyboard: bot.makeFaucetKeyboard(ctx, inlineFaucet.ID).InlineKeyboard}
		results[i] = result
		// needed to set a unique string ID for each result
		results[i].SetResultID(inlineFaucet.ID)

		bot.Cache.Set(inlineFaucet.ID, inlineFaucet, &store.Options{Expiration: 5 * time.Minute})
		log.Infof("[faucet] %s:%d created inline faucet %s: %d sat (%d per user)", GetUserStr(inlineFaucet.From.Telegram), inlineFaucet.From.Telegram.ID, inlineFaucet.ID, inlineFaucet.Amount, inlineFaucet.PerUserAmount)
	}

	err = bot.Telegram.Answer(q, &tb.QueryResponse{
		Results:   results,
		CacheTime: 1,
	})
	if err != nil {
		log.Errorln(err.Error())
	}
}

func (bot *TipBot) acceptInlineFaucetHandler(ctx context.Context, c *tb.Callback) {
	to := LoadUser(ctx)
	tx := &InlineFaucet{Base: storage.New(storage.ID(c.Data))}
	mutex.LockWithContext(ctx, tx.ID)
	defer mutex.UnlockWithContext(ctx, tx.ID)
	fn, err := tx.Get(tx, bot.Bunt)
	if err != nil {
		log.Errorf("[acceptInlineFaucetHandler] c.Data: %s, Error: %s", c.Data, err.Error())
		return
	}
	log.Debugf("[acceptInlineFaucetHandler] Callback c.Data: %s tx.ID: %s", c.Data, tx.ID)

	inlineFaucet := fn.(*InlineFaucet)
	from := inlineFaucet.From
	// log faucet link if possible
	if !inlineFaucet.Active {
		log.Debugf(fmt.Sprintf("[faucet] faucet %s inactive. Remaining: %d sat", inlineFaucet.ID, inlineFaucet.RemainingAmount))
		bot.finishFaucet(ctx, c, inlineFaucet)
		return
	}
	if c.Message != nil && c.Message.Chat != nil {
		log.Infof("[faucet] Link: https://t.me/c/%s/%d", strconv.FormatInt(c.Message.Chat.ID, 10)[4:], c.Message.ID)
	}
	// release faucet no matter what

	if from.Telegram.ID == to.Telegram.ID {
		log.Debugf("[faucet] %s is the owner faucet %s", GetUserStr(to.Telegram), inlineFaucet.ID)
		bot.trySendMessage(from.Telegram, Translate(ctx, "sendYourselfMessage"))
		return
	}
	// check if to user has already taken from the faucet
	for _, a := range inlineFaucet.To {
		if a.Telegram.ID == to.Telegram.ID {
			// to user is already in To slice, has taken from facuet
			log.Debugf("[faucet] %s:%d already took from faucet %s", GetUserStr(to.Telegram), to.Telegram.ID, inlineFaucet.ID)
			return
		}
	}

	defer inlineFaucet.Set(inlineFaucet, bot.Bunt)

	if inlineFaucet.RemainingAmount >= inlineFaucet.PerUserAmount {
		toUserStrMd := GetUserStrMd(to.Telegram)
		fromUserStrMd := GetUserStrMd(from.Telegram)
		toUserStr := GetUserStr(to.Telegram)
		fromUserStr := GetUserStr(from.Telegram)
		// check if user exists and create a wallet if not
		_, exists := bot.UserExists(to.Telegram)
		if !exists {
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
			// bot.trySendMessage(from.Telegram, Translate(ctx, "sendErrorMessage"))
			errMsg := fmt.Sprintf("[faucet] Transaction failed: %s", err.Error())
			log.Warnln(errMsg)
			// if faucet fails, cancel it:
			// c.Sender.ID = inlineFaucet.From.Telegram.ID // overwrite the sender of the callback to be the faucet owner
			// log.Debugf("[faucet] Canceling faucet %s...", inlineFaucet.ID)
			// bot.cancelInlineFaucet(ctx, c, true) // cancel without ID check
			return
		}

		log.Infof("[💸 faucet] Faucet %s from %s to %s:%d (%d sat).", inlineFaucet.ID, fromUserStr, toUserStr, to.Telegram.ID, inlineFaucet.PerUserAmount)
		inlineFaucet.NTaken += 1
		inlineFaucet.To = append(inlineFaucet.To, to)
		inlineFaucet.RemainingAmount = inlineFaucet.RemainingAmount - inlineFaucet.PerUserAmount
		go func() {
			bot.trySendMessage(to.Telegram, fmt.Sprintf(i18n.Translate(to.Telegram.LanguageCode, "inlineFaucetReceivedMessage"), fromUserStrMd, inlineFaucet.PerUserAmount))
			bot.trySendMessage(from.Telegram, fmt.Sprintf(i18n.Translate(from.Telegram.LanguageCode, "inlineFaucetSentMessage"), inlineFaucet.PerUserAmount, toUserStrMd))
		}()
		// build faucet message
		inlineFaucet.Message = fmt.Sprintf(i18n.Translate(inlineFaucet.LanguageCode, "inlineFaucetMessage"), inlineFaucet.PerUserAmount, GetUserStrMd(inlineFaucet.From.Telegram), inlineFaucet.RemainingAmount, inlineFaucet.Amount, inlineFaucet.NTaken, inlineFaucet.NTotal, MakeProgressbar(inlineFaucet.RemainingAmount, inlineFaucet.Amount))
		memo := inlineFaucet.Memo
		if len(memo) > 0 {
			inlineFaucet.Message = inlineFaucet.Message + fmt.Sprintf(i18n.Translate(inlineFaucet.LanguageCode, "inlineFaucetAppendMemo"), memo)
		}
		if inlineFaucet.UserNeedsWallet {
			inlineFaucet.Message += "\n\n" + fmt.Sprintf(i18n.Translate(inlineFaucet.LanguageCode, "inlineFaucetCreateWalletMessage"), GetUserStrMd(bot.Telegram.Me))
		}
		// update message
		log.Infoln(inlineFaucet.Message)

		// update the message if the faucet still has some sats left after this tx
		if inlineFaucet.RemainingAmount >= inlineFaucet.PerUserAmount {
			bot.tryEditStack(c.Message, inlineFaucet.ID, inlineFaucet.Message, bot.makeFaucetKeyboard(ctx, inlineFaucet.ID))
		}

	}
	if inlineFaucet.RemainingAmount < inlineFaucet.PerUserAmount {
		log.Debugf(fmt.Sprintf("[faucet] faucet %s empty. Remaining: %d sat", inlineFaucet.ID, inlineFaucet.RemainingAmount))
		// faucet is depleted
		bot.finishFaucet(ctx, c, inlineFaucet)
	}

}

func (bot *TipBot) cancelInlineFaucet(ctx context.Context, c *tb.Callback, ignoreID bool) {
	tx := &InlineFaucet{Base: storage.New(storage.ID(c.Data))}
	mutex.LockWithContext(ctx, tx.ID)
	defer mutex.UnlockWithContext(ctx, tx.ID)
	fn, err := tx.Get(tx, bot.Bunt)
	if err != nil {
		log.Debugf("[cancelInlineFaucetHandler] %s", err.Error())
		return
	}

	inlineFaucet := fn.(*InlineFaucet)
	if ignoreID || c.Sender.ID == inlineFaucet.From.Telegram.ID {
		bot.tryEditStack(c.Message, inlineFaucet.ID, i18n.Translate(inlineFaucet.LanguageCode, "inlineFaucetCancelledMessage"), &tb.ReplyMarkup{})
		// set the inlineFaucet inactive
		inlineFaucet.Active = false
		inlineFaucet.Canceled = true
		runtime.IgnoreError(inlineFaucet.Set(inlineFaucet, bot.Bunt))
		log.Debugf("[faucet] Faucet %s canceled.", inlineFaucet.ID)
		once.Remove(inlineFaucet.ID)
	}
	return
}

func (bot *TipBot) finishFaucet(ctx context.Context, c *tb.Callback, inlineFaucet *InlineFaucet) {
	inlineFaucet.Message = fmt.Sprintf(i18n.Translate(inlineFaucet.LanguageCode, "inlineFaucetEndedMessage"), inlineFaucet.Amount, inlineFaucet.NTaken)
	if inlineFaucet.UserNeedsWallet {
		inlineFaucet.Message += "\n\n" + fmt.Sprintf(i18n.Translate(inlineFaucet.LanguageCode, "inlineFaucetCreateWalletMessage"), GetUserStrMd(bot.Telegram.Me))
	}
	bot.tryEditStack(c.Message, inlineFaucet.ID, inlineFaucet.Message, &tb.ReplyMarkup{})
	inlineFaucet.Active = false
	log.Debugf("[faucet] Faucet finished %s", inlineFaucet.ID)
	once.Remove(inlineFaucet.ID)
}

func (bot *TipBot) cancelInlineFaucetHandler(ctx context.Context, c *tb.Callback) {
	bot.cancelInlineFaucet(ctx, c, false)
	return
}
