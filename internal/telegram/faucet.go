package telegram

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/LightningTipBot/LightningTipBot/internal/telegram/intercept"

	"github.com/LightningTipBot/LightningTipBot/internal/runtime/mutex"
	"github.com/LightningTipBot/LightningTipBot/internal/runtime/once"
	"github.com/LightningTipBot/LightningTipBot/internal/storage"

	"github.com/eko/gocache/store"

	"github.com/LightningTipBot/LightningTipBot/internal/errors"
	"github.com/LightningTipBot/LightningTipBot/internal/i18n"

	"github.com/LightningTipBot/LightningTipBot/internal/lnbits"

	log "github.com/sirupsen/logrus"
	tb "gopkg.in/lightningtipbot/telebot.v3"
)

var (
	inlineFaucetMenu      = &tb.ReplyMarkup{ResizeKeyboard: true}
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

func (bot TipBot) makeQueryFaucet(ctx intercept.Context) (*InlineFaucet, error) {
	faucet, err := bot.createFaucet(ctx, ctx.Query().Text, ctx.Query().Sender)
	if err != nil {
		switch err.(errors.TipBotError).Code {
		case errors.DecodeAmountError:
			bot.inlineQueryReplyWithError(ctx, TranslateUser(ctx, "inlineQueryFaucetTitle"), fmt.Sprintf(TranslateUser(ctx, "inlineQueryFaucetDescription"), bot.Telegram.Me.Username))
			return nil, err
		case errors.DecodePerUserAmountError:
			bot.inlineQueryReplyWithError(ctx, TranslateUser(ctx, "inlineQueryFaucetTitle"), fmt.Sprintf(TranslateUser(ctx, "inlineQueryFaucetDescription"), bot.Telegram.Me.Username))
			return nil, err
		case errors.InvalidAmountError:
			bot.inlineQueryReplyWithError(ctx, TranslateUser(ctx, "inlineSendInvalidAmountMessage"), fmt.Sprintf(TranslateUser(ctx, "inlineQueryFaucetDescription"), bot.Telegram.Me.Username))
			return nil, err
		case errors.InvalidAmountPerUserError:
			bot.inlineQueryReplyWithError(ctx, TranslateUser(ctx, "inlineFaucetInvalidPeruserAmountMessage"), fmt.Sprintf(TranslateUser(ctx, "inlineQueryFaucetDescription"), bot.Telegram.Me.Username))
			return nil, err
		case errors.GetBalanceError:
			bot.inlineQueryReplyWithError(ctx, TranslateUser(ctx, "inlineQueryFaucetTitle"), fmt.Sprintf(TranslateUser(ctx, "inlineQueryFaucetDescription"), bot.Telegram.Me.Username))
			return nil, err
		case errors.BalanceToLowError:
			bot.inlineQueryReplyWithError(ctx, TranslateUser(ctx, "inlineSendBalanceLowMessage"), fmt.Sprintf(TranslateUser(ctx, "inlineQueryFaucetDescription"), bot.Telegram.Me.Username))
			return nil, err
		}
	}
	return faucet, err
}

func (bot TipBot) makeFaucetKeyboard(ctx context.Context, id string) *tb.ReplyMarkup {
	inlineFaucetMenu := &tb.ReplyMarkup{ResizeKeyboard: true}
	acceptInlineFaucetButton := inlineFaucetMenu.Data(Translate(ctx, "collectButtonMessage"), "confirm_faucet_inline", id)
	cancelInlineFaucetButton := inlineFaucetMenu.Data(Translate(ctx, "cancelButtonMessage"), "cancel_faucet_inline", id)
	inlineFaucetMenu.Inline(
		inlineFaucetMenu.Row(
			acceptInlineFaucetButton,
			cancelInlineFaucetButton),
	)
	return inlineFaucetMenu
}

func (bot TipBot) faucetHandler(ctx intercept.Context) (intercept.Context, error) {
	bot.anyTextHandler(ctx)
	if ctx.Message().Private() {
		bot.trySendMessage(ctx.Message().Sender, fmt.Sprintf(Translate(ctx, "inlineFaucetHelpText"), Translate(ctx, "inlineFaucetHelpFaucetInGroup")))
		return ctx, errors.Create(errors.NoPrivateChatError)
	}
	ctx.Context = bot.mapFaucetLanguage(ctx, ctx.Text())
	inlineFaucet, err := bot.makeFaucet(ctx, ctx.Message(), false)
	if err != nil {
		log.WithFields(log.Fields{
			"module": "telegram-faucet",
			"func":   "faucetHandler",
			"error":  err.Error()}).Warnf("could not create faucet from message")
		return ctx, err
	}
	fromUserStr := GetUserStr(ctx.Message().Sender)
	mFaucet := bot.trySendMessage(ctx.Message().Chat, inlineFaucet.Message, bot.makeFaucetKeyboard(ctx, inlineFaucet.ID))
	log.WithFields(log.Fields{
		"module":      "telegram-faucet",
		"func":        "faucetHandler",
		"user":        fromUserStr,
		"telegram_id": ctx.Message().Sender.ID,
		"data":        fmt.Sprintf("[faucet] %s created faucet %s: %d sat (%d per user)", fromUserStr, inlineFaucet.ID, inlineFaucet.Amount, inlineFaucet.PerUserAmount),
		"error":       err.Error()}).Infof("created faucet")

	// log faucet link if possible
	if mFaucet != nil && mFaucet.Chat != nil {
		log.WithFields(log.Fields{
			"module":      "telegram-faucet",
			"func":        "faucetHandler",
			"user":        fromUserStr,
			"telegram_id": ctx.Message().Sender.ID,
			"data":        fmt.Sprintf("https://t.me/c/%s/%d", strconv.FormatInt(mFaucet.Chat.ID, 10)[4:], mFaucet.ID),
			"error":       err.Error()}).Infof("found faucet link")
	}
	return ctx, inlineFaucet.Set(inlineFaucet, bot.Bunt)
}

func (bot TipBot) handleInlineFaucetQuery(ctx intercept.Context) (intercept.Context, error) {
	inlineFaucet, err := bot.makeQueryFaucet(ctx)
	if err != nil {
		log.WithFields(log.Fields{
			"module": "telegram-faucet",
			"func":   "handleInlineFaucetQuery",
			"error":  err.Error()}).Errorf("could not create faucet from query")
		return ctx, err
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
		result.ReplyMarkup = &tb.ReplyMarkup{InlineKeyboard: bot.makeFaucetKeyboard(ctx, inlineFaucet.ID).InlineKeyboard}
		results[i] = result
		// needed to set a unique string ID for each result
		results[i].SetResultID(inlineFaucet.ID)

		bot.Cache.Set(inlineFaucet.ID, inlineFaucet, &store.Options{Expiration: 5 * time.Minute})
		log.Infof("[faucet] %s:%d created inline faucet %s: %d sat (%d per user)", GetUserStr(inlineFaucet.From.Telegram), inlineFaucet.From.Telegram.ID, inlineFaucet.ID, inlineFaucet.Amount, inlineFaucet.PerUserAmount)
	}

	err = bot.Telegram.Answer(ctx.Query(), &tb.QueryResponse{
		Results:   results,
		CacheTime: 1,
	})
	if err != nil {
		log.Errorln(err.Error())
		return ctx, err
	}
	return ctx, nil
}

func (bot *TipBot) acceptInlineFaucetHandler(ctx intercept.Context) (intercept.Context, error) {
	c := ctx.Callback()
	to := LoadUser(ctx)
	tx := &InlineFaucet{Base: storage.New(storage.ID(c.Data))}
	mutex.LockWithContext(ctx, tx.ID)
	defer mutex.UnlockWithContext(ctx, tx.ID)
	fn, err := tx.Get(tx, bot.Bunt)
	if err != nil {
		log.WithFields(log.Fields{
			"module":      "telegram-faucet",
			"func":        "acceptInlineFaucetHandler",
			"telegram_id": ctx.Message().Sender.ID,
			"data":        c.Data,
			"error":       err.Error()}).Errorf("could not fetch transaction")
		return ctx, err
	}
	log.Tracef("[acceptInlineFaucetHandler] Callback c.Data: %s tx.ID: %s", c.Data, tx.ID)

	inlineFaucet := fn.(*InlineFaucet)
	from := inlineFaucet.From
	// failsafe for queued users
	if !inlineFaucet.Active {
		log.Tracef(fmt.Sprintf("[faucet] faucet %s inactive. Remaining: %d sat", inlineFaucet.ID, inlineFaucet.RemainingAmount))
		bot.finishFaucet(ctx, c, inlineFaucet)
		return ctx, errors.Create(errors.NotActiveError)
	}
	// log faucet link if possible
	if c.Message != nil && c.Message.Chat != nil {
		log.WithFields(log.Fields{
			"module":      "telegram-faucet",
			"func":        "acceptInlineFaucetHandler",
			"user":        GetUserStr(from.Telegram),
			"user_id":     from.ID,
			"telegram_id": from.Telegram.ID,
			"wallet_id":   from.Wallet.ID,
			"data":        fmt.Sprintf("[faucet] Link: https://t.me/c/%s/%d", strconv.FormatInt(c.Message.Chat.ID, 10)[4:], c.Message.ID),
		}).Infof("found faucet link")
	}

	if from.Telegram.ID == to.Telegram.ID {
		log.WithFields(log.Fields{
			"module":      "telegram-faucet",
			"func":        "acceptInlineFaucetHandler",
			"user":        GetUserStr(from.Telegram),
			"user_id":     from.ID,
			"telegram_id": from.Telegram.ID,
			"wallet_id":   from.Wallet.ID,
			"data":        inlineFaucet.ID,
		}).Debugf("cannot drain own faucet")
		ctx.Context = context.WithValue(ctx, "callback_response", Translate(ctx, "sendYourselfMessage"))
		return ctx, errors.Create(errors.SelfPaymentError)
	}
	// check if to user has already taken from the faucet
	for _, a := range inlineFaucet.To {
		if a.Telegram.ID == to.Telegram.ID {
			// to user is already in To slice, has taken from facuet
			log.WithFields(log.Fields{
				"module":         "telegram-faucet",
				"func":           "acceptInlineFaucetHandler",
				"user":           GetUserStr(from.Telegram),
				"to_user_id":     to.ID,
				"to_telegram_id": to.Telegram.ID,
				"user_id":        from.ID,
				"telegram_id":    from.Telegram.ID,
				"wallet_id":      from.Wallet.ID,
				"data":           inlineFaucet.ID,
			}).Debugf("already took from faucet")
			ctx.Context = context.WithValue(ctx, "callback_response", Translate(ctx, "inlineFaucetAlreadyTookMessage"))
			return ctx, errors.Create(errors.UnknownError)
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
				log.WithFields(log.Fields{
					"module":       "telegram-faucet",
					"func":         "acceptInlineFaucetHandler",
					"to_user":      GetUserStr(to.Telegram),
					"to_user_id":   to.ID,
					"to_wallet_id": to.Wallet.ID,
					"user":         GetUserStr(from.Telegram),
					"user_id":      from.ID,
					"wallet_id":    from.Wallet.ID,
					"error":        err.Error()},
				).Errorln("Could not create wallet for user")
				return ctx, err
			}
		}

		if !to.Initialized {
			inlineFaucet.UserNeedsWallet = true
		}

		// todo: user new get username function to get userStrings
		transactionMemo := fmt.Sprintf("🚰 Faucet from %s to %s.", fromUserStr, toUserStr)
		t := NewTransaction(bot, from, to, inlineFaucet.PerUserAmount, TransactionType("faucet"))
		t.Memo = transactionMemo

		success, err := t.Send()
		if !success {
			// bot.trySendMessage(from.Telegram, Translate(ctx, "sendErrorMessage"))
			errMsg := fmt.Sprintf("[faucet] Transaction failed: %s", err.Error())
			log.Warnln(errMsg)
			ctx.Context = context.WithValue(ctx, "callback_response", Translate(ctx, "errorTryLaterMessage"))
			// if faucet fails, cancel it:
			// c.Sender.ID = inlineFaucet.From.Telegram.ID // overwrite the sender of the callback to be the faucet owner
			// log.Debugf("[faucet] Canceling faucet %s...", inlineFaucet.ID)
			// bot.cancelInlineFaucet(ctx, c, true) // cancel without ID check
			bot.finishFaucet(ctx, c, inlineFaucet)
			return ctx, errors.New(errors.UnknownError, err)
		}

		log.Infof("[💸 faucet] Faucet %s from %s to %s:%d (%d sat).", inlineFaucet.ID, fromUserStr, toUserStr, to.Telegram.ID, inlineFaucet.PerUserAmount)
		inlineFaucet.NTaken += 1
		inlineFaucet.To = append(inlineFaucet.To, to)
		inlineFaucet.RemainingAmount = inlineFaucet.RemainingAmount - inlineFaucet.PerUserAmount
		go func() {
			to_message := fmt.Sprintf(i18n.Translate(to.Telegram.LanguageCode, "inlineFaucetReceivedMessage"), fromUserStrMd, inlineFaucet.PerUserAmount)
			ctx.Context = context.WithValue(ctx, "callback_response", to_message)
			bot.trySendMessage(to.Telegram, to_message)
			bot.trySendMessage(from.Telegram, fmt.Sprintf(i18n.Translate(from.Telegram.LanguageCode, "inlineFaucetSentMessage"), inlineFaucet.PerUserAmount, toUserStrMd))
		}()

		// build faucet message
		inlineFaucet.Message = fmt.Sprintf(i18n.Translate(inlineFaucet.LanguageCode, "inlineFaucetMessage"), inlineFaucet.PerUserAmount, GetUserStrMd(inlineFaucet.From.Telegram), inlineFaucet.RemainingAmount, inlineFaucet.Amount, inlineFaucet.NTaken, inlineFaucet.NTotal, MakeProgressbar(inlineFaucet.RemainingAmount, inlineFaucet.Amount))
		memo := inlineFaucet.Memo
		if len(memo) > 0 {
			inlineFaucet.Message = inlineFaucet.Message + fmt.Sprintf(i18n.Translate(inlineFaucet.LanguageCode, "inlineFaucetAppendMemo"), memo)
		}
		if inlineFaucet.UserNeedsWallet {
			inlineFaucet.Message += "\n\n" + fmt.Sprintf(i18n.Translate(inlineFaucet.LanguageCode, "inlineFaucetCreateWalletMessage"), GetUserStr(bot.Telegram.Me))
		}
		// update message
		log.Infoln(inlineFaucet.Message)

		// update the message if the faucet still has some sats left after this tx
		if inlineFaucet.RemainingAmount >= inlineFaucet.PerUserAmount {
			bot.tryEditStack(c, inlineFaucet.ID, inlineFaucet.Message, bot.makeFaucetKeyboard(ctx, inlineFaucet.ID))
		}
	}
	if inlineFaucet.RemainingAmount < inlineFaucet.PerUserAmount {
		log.Debugf(fmt.Sprintf("[faucet] faucet %s empty. Remaining: %d sat", inlineFaucet.ID, inlineFaucet.RemainingAmount))
		// faucet is depleted
		bot.finishFaucet(ctx, c, inlineFaucet)
	}
	return ctx, nil
}

func (bot *TipBot) cancelInlineFaucet(ctx context.Context, c *tb.Callback, ignoreID bool) (context.Context, error) {
	tx := &InlineFaucet{Base: storage.New(storage.ID(c.Data))}
	mutex.LockWithContext(ctx, tx.ID)
	defer mutex.UnlockWithContext(ctx, tx.ID)
	fn, err := tx.Get(tx, bot.Bunt)
	if err != nil {
		log.Debugf("[cancelInlineFaucetHandler] %s", err.Error())
		return ctx, err
	}

	inlineFaucet := fn.(*InlineFaucet)
	if ignoreID || c.Sender.ID == inlineFaucet.From.Telegram.ID {
		faucet_cancelled_message := i18n.Translate(inlineFaucet.LanguageCode, "inlineFaucetCancelledMessage")
		bot.tryEditStack(c, inlineFaucet.ID, faucet_cancelled_message, &tb.ReplyMarkup{})
		ctx = context.WithValue(ctx, "callback_response", faucet_cancelled_message)
		// set the inlineFaucet inactive
		inlineFaucet.Active = false
		inlineFaucet.Canceled = true
		err = inlineFaucet.Set(inlineFaucet, bot.Bunt)
		if err != nil {
			return ctx, err
		}
		log.Debugf("[faucet] Faucet %s canceled.", inlineFaucet.ID)
		once.Remove(inlineFaucet.ID)
	}
	return ctx, nil
}

func (bot *TipBot) finishFaucet(ctx context.Context, c *tb.Callback, inlineFaucet *InlineFaucet) {
	inlineFaucet.Message = fmt.Sprintf(i18n.Translate(inlineFaucet.LanguageCode, "inlineFaucetEndedMessage"), inlineFaucet.Amount, inlineFaucet.NTaken)
	if inlineFaucet.UserNeedsWallet {
		inlineFaucet.Message += "\n\n" + fmt.Sprintf(i18n.Translate(inlineFaucet.LanguageCode, "inlineFaucetCreateWalletMessage"), GetUserStrMd(bot.Telegram.Me))
	}
	bot.tryEditStack(c, inlineFaucet.ID, inlineFaucet.Message, &tb.ReplyMarkup{})

	log.Debugf("[faucet] Faucet finished %s", inlineFaucet.ID)
	once.Remove(inlineFaucet.ID)
	// send update to faucet creator
	if inlineFaucet.Active && inlineFaucet.From.Telegram.ID != 0 {
		bot.trySendMessage(inlineFaucet.From.Telegram, listFaucetTakers(inlineFaucet))
	}
	inlineFaucet.Active = false
}

func listFaucetTakers(inlineFaucet *InlineFaucet) string {
	var to_str string
	to_str = fmt.Sprintf("🚰 *Faucet summary*\n\nMemo: %s\nCapacity: %d sat\nTakers: %d\nRemaining: %d sat\n\n*Takers:*\n\n", inlineFaucet.Memo, inlineFaucet.Amount, inlineFaucet.NTaken, inlineFaucet.RemainingAmount)
	to_str += "```\n"
	for _, to := range inlineFaucet.To {
		to_str += fmt.Sprintf("%s\n", GetUserStr(to.Telegram))
	}
	to_str += "```"
	return to_str
}

func (bot *TipBot) cancelInlineFaucetHandler(ctx intercept.Context) (intercept.Context, error) {
	var err error
	ctx.Context, err = bot.cancelInlineFaucet(ctx, ctx.Callback(), false)
	return ctx, err

}
