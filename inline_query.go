package main

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	log "github.com/sirupsen/logrus"
	tb "gopkg.in/tucnak/telebot.v2"
)

const queryImage = "https://avatars.githubusercontent.com/u/88730856?v=4"

func (bot TipBot) inlineQueryInstructions(ctx context.Context, q *tb.Query) {
	instructions := []struct {
		url         string
		title       string
		description string
	}{
		{
			url:         queryImage,
			title:       TranslateUser(ctx, "inlineQuerySendTitle"),
			description: fmt.Sprintf(TranslateUser(ctx, "inlineQuerySendDescription"), bot.telegram.Me.Username),
		},
		{
			url:         queryImage,
			title:       TranslateUser(ctx, "inlineQueryReceiveTitle"),
			description: fmt.Sprintf(TranslateUser(ctx, "inlineQueryReceiveDescription"), bot.telegram.Me.Username),
		},
		{
			url:         queryImage,
			title:       TranslateUser(ctx, "inlineQueryFaucetTitle"),
			description: fmt.Sprintf(TranslateUser(ctx, "inlineQueryFaucetDescription"), bot.telegram.Me.Username),
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

func (bot TipBot) commandTranslationMap(ctx context.Context, command string) context.Context {
	switch command {
	// is default, we don't have to check it
	// case "faucet":
	// 	ctx = context.WithValue(ctx, "publicLanguageCode", "en")
	// 	ctx = context.WithValue(ctx, "publicLocalizer", i18n.NewLocalizer(bot.bundle, "en"))
	case "zapfhahn":
		ctx = context.WithValue(ctx, "publicLanguageCode", "de")
		ctx = context.WithValue(ctx, "publicLocalizer", i18n.NewLocalizer(bot.bundle, "de"))
	case "kraan":
		ctx = context.WithValue(ctx, "publicLanguageCode", "nl")
		ctx = context.WithValue(ctx, "publicLocalizer", i18n.NewLocalizer(bot.bundle, "nl"))
	case "grifo":
		ctx = context.WithValue(ctx, "publicLanguageCode", "es")
		ctx = context.WithValue(ctx, "publicLocalizer", i18n.NewLocalizer(bot.bundle, "es"))
	}
	return ctx
}

func (bot TipBot) anyQueryHandler(ctx context.Context, q *tb.Query) {
	if q.Text == "" {
		bot.inlineQueryInstructions(ctx, q)
		return
	}

	// create the inline send result
	if strings.HasPrefix(q.Text, "/") {
		q.Text = strings.TrimPrefix(q.Text, "/")
	}
	if strings.HasPrefix(q.Text, "send") || strings.HasPrefix(q.Text, "pay") {
		bot.handleInlineSendQuery(ctx, q)
	}

	if strings.HasPrefix(q.Text, "faucet") || strings.HasPrefix(q.Text, "zapfhahn") || strings.HasPrefix(q.Text, "kraan") || strings.HasPrefix(q.Text, "grifo") {
		if len(strings.Split(q.Text, " ")) > 1 {
			c := strings.Split(q.Text, " ")[0]
			ctx = bot.commandTranslationMap(ctx, c)
		}
		bot.handleInlineFaucetQuery(ctx, q)
	}

	if strings.HasPrefix(q.Text, "receive") || strings.HasPrefix(q.Text, "get") || strings.HasPrefix(q.Text, "payme") || strings.HasPrefix(q.Text, "request") {
		bot.handleInlineReceiveQuery(ctx, q)
	}
}
