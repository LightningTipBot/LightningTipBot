package telegram

import (
	"context"
	"fmt"
	"github.com/LightningTipBot/LightningTipBot/internal/telegram/intercept"
	"reflect"
	"strconv"
	"strings"

	"github.com/LightningTipBot/LightningTipBot/internal/runtime"
	"github.com/LightningTipBot/LightningTipBot/internal/storage"

	"github.com/LightningTipBot/LightningTipBot/internal/i18n"
	i18n2 "github.com/nicksnyder/go-i18n/v2/i18n"
	log "github.com/sirupsen/logrus"
	tb "gopkg.in/telebot.v3"
)

const queryImage = "https://avatars.githubusercontent.com/u/88730856?v=4"

func (bot TipBot) inlineQueryInstructions(ctx intercept.Context) (intercept.Context, error) {
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

	err := ctx.Answer(&tb.QueryResponse{
		Results:    results,
		CacheTime:  5, // a minute
		IsPersonal: true,
		QueryID:    ctx.Query().ID,
	})

	if err != nil {
		log.Errorln(err)
	}
	return ctx, err
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
	id := fmt.Sprintf("inl-error-%d-%s", q.Sender.ID, RandStringRunes(5))
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
func (bot TipBot) anyChosenInlineHandler(ctx intercept.Context) (intercept.Context, error) {
	// load inline object from cache
	inlineObject, err := bot.Cache.Get(ctx.InlineResult().ResultID)
	// check error
	if err != nil {
		log.Errorf("[anyChosenInlineHandler] could not find inline object in cache. %v", err.Error())
		return ctx, err
	}
	switch inlineObject.(type) {
	case storage.Storable:
		// persist inline object in bunt
		runtime.IgnoreError(bot.Bunt.Set(inlineObject.(storage.Storable)))
	default:
		log.Errorf("[anyChosenInlineHandler] invalid inline object type: %s, query: %s", reflect.TypeOf(inlineObject).String(), ctx.InlineResult().Query)
	}
	return ctx, nil
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

func (bot TipBot) anyQueryHandler(ctx intercept.Context) (intercept.Context, error) {
	if ctx.Query().Text == "" {
		return bot.inlineQueryInstructions(ctx)
	}

	// create the inline send result
	var text = ctx.Query().Text
	if strings.HasPrefix(text, "/") {
		text = strings.TrimPrefix(text, "/")
	}
	if strings.HasPrefix(text, "send") || strings.HasPrefix(text, "pay") {
		return bot.handleInlineSendQuery(ctx)
	}

	if strings.HasPrefix(text, "faucet") || strings.HasPrefix(text, "zapfhahn") || strings.HasPrefix(text, "kraan") || strings.HasPrefix(text, "grifo") {
		if len(strings.Split(text, " ")) > 1 {
			c := strings.Split(text, " ")[0]
			ctx.Context = bot.commandTranslationMap(ctx, c)
		}
		return bot.handleInlineFaucetQuery(ctx)
	}
	if strings.HasPrefix(text, "tipjar") || strings.HasPrefix(text, "spendendose") {
		if len(strings.Split(text, " ")) > 1 {
			c := strings.Split(text, " ")[0]
			ctx.Context = bot.commandTranslationMap(ctx, c)
		}
		return bot.handleInlineTipjarQuery(ctx)
	}

	if strings.HasPrefix(text, "receive") || strings.HasPrefix(text, "get") || strings.HasPrefix(text, "payme") || strings.HasPrefix(text, "request") {
		return bot.handleInlineReceiveQuery(ctx)
	}
	return ctx, nil
}
