package telegram

import (
	"context"
	"fmt"
	"strings"

	"github.com/LightningTipBot/LightningTipBot/internal/errors"
	"github.com/LightningTipBot/LightningTipBot/internal/i18n"

	"github.com/LightningTipBot/LightningTipBot/internal/lnbits"
	"github.com/LightningTipBot/LightningTipBot/internal/storage/transaction"

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
	*transaction.Base
	Message         string         `json:"inline_faucet_message"`
	Amount          int            `json:"inline_faucet_amount"`
	RemainingAmount int            `json:"inline_faucet_remainingamount"`
	PerUserAmount   int            `json:"inline_faucet_peruseramount"`
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
	if perUserAmount < 1 || amount%perUserAmount != 0 {
		return nil, errors.New(errors.InvalidAmountPerUserError, fmt.Errorf("invalid amount per user"))
	}
	nTotal := amount / perUserAmount
	fromUser := LoadUser(ctx)
	fromUserStr := GetUserStr(sender)
	balance, err := bot.GetUserBalanceCached(fromUser)
	if err != nil {
		return nil, errors.New(errors.GetBalanceError, err)
	}
	// check if fromUser has balance
	if balance < amount {
		return nil, errors.New(errors.BalanceToLowError, fmt.Errorf("[faucet] Balance of user %s too low: %v", fromUserStr, err))
	}
	// // check for memo in command
	memo := GetMemoFromCommand(text, 3)

	inlineMessage := fmt.Sprintf(Translate(ctx, "inlineFaucetMessage"), perUserAmount, amount, amount, 0, nTotal, MakeProgressbar(amount, amount))
	if len(memo) > 0 {
		inlineMessage = inlineMessage + fmt.Sprintf(Translate(ctx, "inlineFaucetAppendMemo"), memo)
	}
	id := fmt.Sprintf("inl-faucet-%d-%d-%s", sender.ID, amount, RandStringRunes(5))

	return &InlineFaucet{
		Base:            transaction.New(transaction.ID(id)),
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
	// inlineFaucetMenu := &tb.ReplyMarkup{ResizeReplyKeyboard: true}
	acceptInlineFaucetButton := inlineFaucetMenu.Data(Translate(ctx, "collectButtonMessage"), "confirm_faucet_inline")
	cancelInlineFaucetButton := inlineFaucetMenu.Data(Translate(ctx, "cancelButtonMessage"), "cancel_faucet_inline")
	acceptInlineFaucetButton.Data = id
	cancelInlineFaucetButton.Data = id
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
		log.Errorf("[faucet] %s", err)
		return
	}
	fromUserStr := GetUserStr(m.Sender)
	bot.trySendMessage(m.Chat, inlineFaucet.Message, bot.makeFaucetKeyboard(ctx, inlineFaucet.ID))
	log.Infof("[faucet] %s created faucet %s: %d sat (%d per user)", fromUserStr, inlineFaucet.ID, inlineFaucet.Amount, inlineFaucet.PerUserAmount)
	runtime.IgnoreError(inlineFaucet.Set(inlineFaucet, bot.Bunt))
}

func (bot TipBot) handleInlineFaucetQuery(ctx context.Context, q *tb.Query) {
	inlineFaucet, err := bot.makeQueryFaucet(ctx, q, false)
	if err != nil {
		// log.Errorf("[faucet] %s", err)
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

		runtime.IgnoreError(inlineFaucet.Set(inlineFaucet, bot.Bunt))
		log.Infof("[faucet] %s created inline faucet %s: %d sat (%d per user)", GetUserStr(inlineFaucet.From.Telegram), inlineFaucet.ID, inlineFaucet.Amount, inlineFaucet.PerUserAmount)
	}

	err = bot.Telegram.Answer(q, &tb.QueryResponse{
		Results:   results,
		CacheTime: 1,
	})
	if err != nil {
		log.Errorln(err)
	}
}

func (bot *TipBot) acceptInlineFaucetHandler(ctx context.Context, c *tb.Callback) {
	to := LoadUser(ctx)
	tx := &InlineFaucet{Base: transaction.New(transaction.ID(c.Data))}
	fn, err := tx.Get(tx, bot.Bunt)
	if err != nil {
		log.Errorf("[faucet] %s", err)
		return
	}
	inlineFaucet := fn.(*InlineFaucet)
	from := inlineFaucet.From
	err = inlineFaucet.Lock(inlineFaucet, bot.Bunt)
	if err != nil {
		log.Errorf("[faucet] LockFaucet %s error: %s", inlineFaucet.ID, err)
		return
	}
	if !inlineFaucet.Active {
		log.Errorf(fmt.Sprintf("[faucet] faucet %s inactive.", inlineFaucet.ID))
		return
	}
	// release faucet no matter what
	defer inlineFaucet.Release(inlineFaucet, bot.Bunt)

	if from.Telegram.ID == to.Telegram.ID {
		bot.trySendMessage(from.Telegram, Translate(ctx, "sendYourselfMessage"))
		return
	}
	// check if to user has already taken from the faucet
	for _, a := range inlineFaucet.To {
		if a.Telegram.ID == to.Telegram.ID {
			// to user is already in To slice, has taken from facuet
			// log.Infof("[faucet] %s already took from faucet %s", GetUserStr(to.Telegram), inlineFaucet.ID)
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
		inlineFaucet.To = append(inlineFaucet.To, to)
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
		// update message
		log.Infoln(inlineFaucet.Message)
		bot.tryEditMessage(c.Message, inlineFaucet.Message, bot.makeFaucetKeyboard(ctx, inlineFaucet.ID))
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
	tx := &InlineFaucet{Base: transaction.New(transaction.ID(c.Data))}
	fn, err := tx.Get(tx, bot.Bunt)
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
		runtime.IgnoreError(inlineFaucet.Set(inlineFaucet, bot.Bunt))
	}
	return
}
