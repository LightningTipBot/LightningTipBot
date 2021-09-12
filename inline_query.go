package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/LightningTipBot/LightningTipBot/internal/runtime"
	log "github.com/sirupsen/logrus"
	tb "gopkg.in/tucnak/telebot.v2"
)

const queryImage = "https://avatars.githubusercontent.com/u/88730856?v=4"

func (bot TipBot) inlineQueryInstructions(q *tb.Query) {
	instructions := []struct {
		url         string
		title       string
		description string
	}{
		{
			url:         queryImage,
			title:       inlineQuerySendTitle,
			description: fmt.Sprintf(inlineQuerySendDescription, bot.telegram.Me.Username),
		},
		{
			url:         queryImage,
			title:       inlineQueryFaucetTitle,
			description: fmt.Sprintf(inlineQueryFaucetDescription, bot.telegram.Me.Username),
		},
	}
	results := make(tb.Results, len(instructions)) // []tb.Result
	for i, instruction := range instructions {
		result := &tb.ArticleResult{
			//URL:         instruction.url,
			Text:        instruction.description,
			Title:       instruction.title,
			Description: instruction.description,
			// required for photos
			ThumbURL: instruction.url,
		}
		results[i] = result
		// needed to set a unique string ID for each result
		results[i].SetResultID(strconv.Itoa(i))
	}

	err := bot.telegram.Answer(q, &tb.QueryResponse{
		Results:    results,
		CacheTime:  5, // a minute
		IsPersonal: true,
		QueryID:    q.ID,
	})

	if err != nil {
		log.Errorln(err)
	}
}

func (bot TipBot) anyChosenInlineHandler(q *tb.ChosenInlineResult) {
	fmt.Printf(q.Query)
}

func (bot TipBot) handleInlineSendQuery(q *tb.Query) {
	amount, err := decodeAmountFromCommand(q.Text)
	if err != nil {
		return
	}
	fromUserStr := GetUserStr(&q.From)
	balance, err := bot.GetUserBalance(&q.From)
	if err != nil {
		errmsg := fmt.Sprintf("could not get balance of user %s", fromUserStr)
		log.Errorln(errmsg)
		return
	}
	// check if fromUser has balance
	if balance < amount {
		log.Errorln("Balance of user %s too low", fromUserStr)
		return
	}

	// check for memo in command
	memo := ""
	if len(strings.Split(q.Text, " ")) > 2 {
		memo = strings.SplitN(q.Text, " ", 3)[2]
		memoMaxLen := 159
		if len(memo) > memoMaxLen {
			memo = memo[:memoMaxLen]
		}
	}

	urls := []string{
		queryImage,
	}
	results := make(tb.Results, len(urls)) // []tb.Result
	for i, url := range urls {

		inlineMessage := fmt.Sprintf(inlineSendMessage, amount)

		if len(memo) > 0 {
			inlineMessage = inlineMessage + fmt.Sprintf(inlineSendAppendMemo, memo)
		}

		result := &tb.ArticleResult{
			// URL:         url,
			Text:        inlineMessage,
			Title:       fmt.Sprintf(inlineResultSendTitle, amount),
			Description: fmt.Sprintf(inlineResultSendDescription, amount),
			// required for photos
			ThumbURL: url,
		}
		id := fmt.Sprintf("inl-send-%d-%d-%s", q.From.ID, amount, RandStringRunes(5))
		btnSendInline.Data = id
		btnCancelSendInline.Data = id
		sendInlineMenu.Inline(sendInlineMenu.Row(btnSendInline, btnCancelSendInline))
		result.ReplyMarkup = &tb.InlineKeyboardMarkup{InlineKeyboard: sendInlineMenu.InlineKeyboard}

		results[i] = result

		// needed to set a unique string ID for each result
		results[i].SetResultID(id)

		// create persistend inline send struct
		inlineSend := NewInlineSend(inlineMessage)
		// add data to persistent object
		inlineSend.ID = id
		inlineSend.From = &q.From
		// add result to persistent struct
		inlineSend.Amount = amount
		inlineSend.Memo = memo
		inlineSend.Active = true
		runtime.IgnoreError(bot.bunt.Set(inlineSend))
	}

	err = bot.telegram.Answer(q, &tb.QueryResponse{
		Results:   results,
		CacheTime: 1, // 60 == 1 minute, todo: make higher than 1 s in production

	})

	if err != nil {
		log.Errorln(err)
	}
}

func (bot TipBot) handleInlineFaucetQuery(q *tb.Query) {
	amount, err := decodeAmountFromCommand(q.Text)
	if err != nil {
		return
	}

	peruserStr, err := getArgumentFromCommand(q.Text, 2)
	if err != nil {
		return
	}
	peruser, err := strconv.Atoi(peruserStr)
	if err != nil {
		return
	}
	// peruser amount must be >1 and a divisor of amount
	if peruser < 1 || amount%peruser != 0 {
		return
	}
	ntotal := amount / peruser

	fromUserStr := GetUserStr(&q.From)
	balance, err := bot.GetUserBalance(&q.From)
	if err != nil {
		errmsg := fmt.Sprintf("could not get balance of user %s", fromUserStr)
		log.Errorln(errmsg)
		return
	}
	// check if fromUser has balance
	if balance < amount {
		log.Errorln("Balance of user %s too low", fromUserStr)
		return
	}

	// check for memo in command
	memo := ""
	if len(strings.Split(q.Text, " ")) > 2 {
		memo = strings.SplitN(q.Text, " ", 3)[2]
		memoMaxLen := 159
		if len(memo) > memoMaxLen {
			memo = memo[:memoMaxLen]
		}
	}

	urls := []string{
		queryImage,
	}
	results := make(tb.Results, len(urls)) // []tb.Result
	for i, url := range urls {

		inlineMessage := fmt.Sprintf(inlineFaucetMessage, amount, amount, 0)

		if len(memo) > 0 {
			inlineMessage = inlineMessage + fmt.Sprintf(inlineFaucetAppendMemo, memo)
		}

		result := &tb.ArticleResult{
			// URL:         url,
			Text:        inlineMessage,
			Title:       fmt.Sprintf(inlineResultFaucetTitle, amount),
			Description: fmt.Sprintf(inlineResultFaucetDescription, amount),
			// required for photos
			ThumbURL: url,
		}
		id := fmt.Sprintf("inl-faucet-%d-%d-%s", q.From.ID, amount, RandStringRunes(5))
		btnFaucetInline.Data = id
		btnCancelFaucetInline.Data = id
		faucetInlineMenu.Inline(faucetInlineMenu.Row(btnFaucetInline, btnCancelFaucetInline))
		result.ReplyMarkup = &tb.InlineKeyboardMarkup{InlineKeyboard: faucetInlineMenu.InlineKeyboard}

		results[i] = result

		// needed to set a unique string ID for each result
		results[i].SetResultID(id)

		// create persistend inline send struct
		inlineFaucet := NewInlineFaucet(inlineMessage)
		// add data to persistent object
		inlineFaucet.ID = id
		inlineFaucet.From = &q.From
		// add result to persistent struct
		inlineFaucet.Amount = amount
		inlineFaucet.PerUserAmount = peruser
		inlineFaucet.RemainingAmount = amount
		inlineFaucet.NTotal = ntotal
		inlineFaucet.NTaken = 0

		inlineFaucet.Memo = memo
		inlineFaucet.Active = true
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

func (bot TipBot) anyQueryHandler(q *tb.Query) {
	if q.Text == "" {
		bot.inlineQueryInstructions(q)
		return
	}

	// create the inline send result
	if strings.HasPrefix(q.Text, "send") || strings.HasPrefix(q.Text, "/send") || strings.HasPrefix(q.Text, "giveaway") || strings.HasPrefix(q.Text, "/giveaway") {
		bot.handleInlineSendQuery(q)
	}

	if strings.HasPrefix(q.Text, "faucet") || strings.HasPrefix(q.Text, "/faucet") {
		bot.handleInlineFaucetQuery(q)
	}
}
