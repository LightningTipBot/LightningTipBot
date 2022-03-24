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

func (bot TipBot) inlineQueryInstructions(handler intercept.Handler) (intercept.Handler, error) {
	instructions := []struct {
		url         string
		title       string
		description string
	}{
		{
			url:         queryImage,
			title:       TranslateUser(handler.Ctx, "inlineQuerySendTitle"),
			description: fmt.Sprintf(TranslateUser(handler.Ctx, "inlineQuerySendDescription"), bot.Telegram.Me.Username),
		},
		{
			url:         queryImage,
			title:       TranslateUser(handler.Ctx, "inlineQueryReceiveTitle"),
			description: fmt.Sprintf(TranslateUser(handler.Ctx, "inlineQueryReceiveDescription"), bot.Telegram.Me.Username),
		},
		{
			url:         queryImage,
			title:       TranslateUser(handler.Ctx, "inlineQueryFaucetTitle"),
			description: fmt.Sprintf(TranslateUser(handler.Ctx, "inlineQueryFaucetDescription"), bot.Telegram.Me.Username),
		},
		{
			url:         queryImage,
			title:       TranslateUser(handler.Ctx, "inlineQueryTipjarTitle"),
			description: fmt.Sprintf(TranslateUser(handler.Ctx, "inlineQueryTipjarDescription"), bot.Telegram.Me.Username),
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

	err := handler.Answer(&tb.QueryResponse{
		Results:    results,
		CacheTime:  5, // a minute
		IsPersonal: true,
		QueryID:    handler.Query().ID,
	})

	if err != nil {
		log.Errorln(err)
	}
	return handler, err
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
func (bot TipBot) anyChosenInlineHandler(handler intercept.Handler) (intercept.Handler, error) {
	// load inline object from cache
	inlineObject, err := bot.Cache.Get(handler.InlineResult().ResultID)
	// check error
	if err != nil {
		log.Errorf("[anyChosenInlineHandler] could not find inline object in cache. %v", err.Error())
		return handler, err
	}
	switch inlineObject.(type) {
	case storage.Storable:
		// persist inline object in bunt
		runtime.IgnoreError(bot.Bunt.Set(inlineObject.(storage.Storable)))
	default:
		log.Errorf("[anyChosenInlineHandler] invalid inline object type: %s, query: %s", reflect.TypeOf(inlineObject).String(), handler.InlineResult().Query)
	}
	return handler, nil
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

func (bot TipBot) anyQueryHandler(handler intercept.Handler) (intercept.Handler, error) {
	if handler.Query().Text == "" {
		return bot.inlineQueryInstructions(handler)
	}

	// create the inline send result
	var text = handler.Query().Text
	if strings.HasPrefix(text, "/") {
		text = strings.TrimPrefix(text, "/")
	}
	if strings.HasPrefix(text, "send") || strings.HasPrefix(text, "pay") {
		return bot.handleInlineSendQuery(handler)
	}

	if strings.HasPrefix(text, "faucet") || strings.HasPrefix(text, "zapfhahn") || strings.HasPrefix(text, "kraan") || strings.HasPrefix(text, "grifo") {
		if len(strings.Split(text, " ")) > 1 {
			c := strings.Split(text, " ")[0]
			handler.Ctx = bot.commandTranslationMap(handler.Ctx, c)
		}
		return bot.handleInlineFaucetQuery(handler)
	}
	if strings.HasPrefix(text, "tipjar") || strings.HasPrefix(text, "spendendose") {
		if len(strings.Split(text, " ")) > 1 {
			c := strings.Split(text, " ")[0]
			handler.Ctx = bot.commandTranslationMap(handler.Ctx, c)
		}
		return bot.handleInlineTipjarQuery(handler)
	}

	if strings.HasPrefix(text, "receive") || strings.HasPrefix(text, "get") || strings.HasPrefix(text, "payme") || strings.HasPrefix(text, "request") {
		return bot.handleInlineReceiveQuery(handler)
	}
	return handler, nil
}
