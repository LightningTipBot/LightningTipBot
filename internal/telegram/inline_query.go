package telegram

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/LightningTipBot/LightningTipBot/internal/runtime"
	"github.com/LightningTipBot/LightningTipBot/internal/storage"

	"github.com/LightningTipBot/LightningTipBot/internal/i18n"
	i18n2 "github.com/nicksnyder/go-i18n/v2/i18n"
	log "github.com/sirupsen/logrus"
	tb "gopkg.in/lightningtipbot/telebot.v2"
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
			description: fmt.Sprintf(TranslateUser(ctx, "inlineQuerySendDescription"), bot.Telegram.Me.Username),
		},
		{
			url:         queryImage,
			title:       TranslateUser(ctx, "inlineQueryReceiveTitle"),
			description: fmt.Sprintf(TranslateUser(ctx, "inlineQueryReceiveDescription"), bot.Telegram.Me.Username),
		},
		{
			url:         queryImage,
			title:       TranslateUser(ctx, "inlineQueryFaucetTitle"),
			description: fmt.Sprintf(TranslateUser(ctx, "inlineQueryFaucetDescription"), bot.Telegram.Me.Username),
		},
		{
			url:         queryImage,
			title:       TranslateUser(ctx, "inlineQueryTipjarTitle"),
			description: fmt.Sprintf(TranslateUser(ctx, "inlineQueryTipjarDescription"), bot.Telegram.Me.Username),
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

	err := bot.Telegram.Answer(q, &tb.QueryResponse{
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
	err := bot.Telegram.Answer(q, &tb.QueryResponse{
		Results:   results,
		CacheTime: 1, // 60 == 1 minute, todo: make higher than 1 s in production

	})
	if err != nil {
		log.Errorln(err)
	}
}

// anyChosenInlineHandler will load any inline object from cache and store into bunt.
// this is used to decrease bunt db write ops.
func (bot TipBot) anyChosenInlineHandler(q *tb.ChosenInlineResult) {
	// load inline object from cache
	inlineObject, err := bot.Cache.Get(q.ResultID)
	// check error
	if err != nil {
		log.Errorf("[anyChosenInlineHandler] could not find inline object in cache. %v", err.Error())
		return
	}
	switch inlineObject.(type) {
	case storage.Storable:
		// persist inline object in bunt
		runtime.IgnoreError(bot.Bunt.Set(inlineObject.(storage.Storable)))
	default:
		log.Errorf("[anyChosenInlineHandler] invalid inline object type: %s, query: %s", reflect.TypeOf(inlineObject).String(), q.Query)
	}
}

func (bot TipBot) commandTranslationMap(ctx context.Context, command string) context.Context {
	switch command {
	// is default, we don't have to check it
	// case "faucet":
	// 	ctx = context.WithValue(ctx, "publicLanguageCode", "en")
	// 	ctx = context.WithValue(ctx, "publicLocalizer", i18n.NewLocalizer(i18n.Bundle, "en"))
	case "zapfhahn", "spendendose":
		ctx = context.WithValue(ctx, "publicLanguageCode", "de")
		ctx = context.WithValue(ctx, "publicLocalizer", i18n2.NewLocalizer(i18n.Bundle, "de"))
	case "kraan":
		ctx = context.WithValue(ctx, "publicLanguageCode", "nl")
		ctx = context.WithValue(ctx, "publicLocalizer", i18n2.NewLocalizer(i18n.Bundle, "nl"))
	case "grifo":
		ctx = context.WithValue(ctx, "publicLanguageCode", "es")
		ctx = context.WithValue(ctx, "publicLocalizer", i18n2.NewLocalizer(i18n.Bundle, "es"))
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
	if strings.HasPrefix(q.Text, "tipjar") || strings.HasPrefix(q.Text, "spendendose") {
		if len(strings.Split(q.Text, " ")) > 1 {
			c := strings.Split(q.Text, " ")[0]
			ctx = bot.commandTranslationMap(ctx, c)
		}
		bot.handleInlineTipjarQuery(ctx, q)
	}

	if strings.HasPrefix(q.Text, "receive") || strings.HasPrefix(q.Text, "get") || strings.HasPrefix(q.Text, "payme") || strings.HasPrefix(q.Text, "request") {
		bot.handleInlineReceiveQuery(ctx, q)
	}
}
