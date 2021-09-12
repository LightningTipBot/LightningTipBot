package main

import (
	"fmt"
	"strconv"
	"strings"

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
			title:       inlineQueryReceiveTitle,
			description: fmt.Sprintf(inlineQueryReceiveDescription, bot.telegram.Me.Username),
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

func (bot TipBot) inlineQueryReplyWithError(q *tb.Query, message string, help string) {
	results := make(tb.Results, 1) // []tb.Result
	result := &tb.ArticleResult{
		// URL:         url,
		Text:        help,
		Title:       message,
		Description: help,
		// required for photos
		ThumbURL: queryImage,
	}
	id := fmt.Sprintf("inl-error-%d-%s", q.From.ID, RandStringRunes(5))
	result.SetResultID(id)
	results[0] = result
	err := bot.telegram.Answer(q, &tb.QueryResponse{
		Results:   results,
		CacheTime: 1, // 60 == 1 minute, todo: make higher than 1 s in production

	})
	if err != nil {
		log.Errorln(err)
	}
}

func (bot TipBot) anyChosenInlineHandler(q *tb.ChosenInlineResult) {
	fmt.Printf(q.Query)
}

func (bot TipBot) anyQueryHandler(q *tb.Query) {
	if q.Text == "" {
		bot.inlineQueryInstructions(q)
		return
	}

	// create the inline send result
	if strings.HasPrefix(q.Text, "send") || strings.HasPrefix(q.Text, "/send") || strings.HasPrefix(q.Text, "giveaway") || strings.HasPrefix(q.Text, "/giveaway") || strings.HasPrefix(q.Text, "pay") {
		bot.handleInlineSendQuery(q)
	}

	if strings.HasPrefix(q.Text, "faucet") || strings.HasPrefix(q.Text, "/faucet") {
		bot.handleInlineFaucetQuery(q)
	}

	if strings.HasPrefix(q.Text, "receive") || strings.HasPrefix(q.Text, "/receive") || strings.HasPrefix(q.Text, "get") || strings.HasPrefix(q.Text, "payme") || strings.HasPrefix(q.Text, "request") {
		bot.handleInlineReceiveQuery(q)
	}
}
