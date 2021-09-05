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

var (
	sendInlineMenu      = &tb.ReplyMarkup{ResizeReplyKeyboard: true}
	btnCancelSendInline = paymentConfirmationMenu.Data("🚫 Cancel", "cancel_send_inline")
	btnSendInline       = paymentConfirmationMenu.Data("✅ Accept", "confirm_send_inline")
)

type InlineSend struct {
	Message string            `json:"inline_send_message"`
	result  *tb.ArticleResult `json:"inline_send_articleresult"`
	Amount  int               `json:"inline_send_amount"`
	From    *tb.User          `json:"inline_send_from"`
	To      *tb.User          `json:"inline_send_to"`
	ID      int64             `json:"inline_send_id"`
}

func NewInlineSend(m string, opts ...TipTooltipOption) *InlineSend {
	inlineSend := &InlineSend{
		Message: m,
	}
	// for _, opt := range opts {
	// 	opt(tipTooltip)
	// }
	return inlineSend

}

func (msg InlineSend) Key() string {
	return strconv.Itoa(int(msg.ID))
}

func (bot TipBot) inlineQueryInstructions(q *tb.Query) {
	instructions := []struct {
		url         string
		title       string
		description string
	}{
		{
			url:         queryImage,
			title:       "💸 Send sats to a group or a private chat.",
			description: fmt.Sprintf("Usage: @%s send <amount>", bot.telegram.Me.Username),
		},
	}
	results := make(tb.Results, len(instructions)) // []tb.Result
	for i, instruction := range instructions {
		result := &tb.ArticleResult{
			URL:         instruction.url,
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
		log.Println(err)
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
	if strings.HasPrefix(q.Text, "send ") {
		amount, err := decodeAmountFromCommand(q.Text)
		if err != nil {
			return
		}
		urls := []string{
			queryImage,
		}
		results := make(tb.Results, len(urls)) // []tb.Result
		for i, url := range urls {
			inlineMessage := fmt.Sprintf("Press ✅ to accept payment.\n\n💸 Amount: %d sat\n", amount)

			// create persistend inline send struct
			inlineSend := NewInlineSend(inlineMessage)

			result := &tb.ArticleResult{
				URL:         url,
				Text:        inlineMessage,
				Title:       fmt.Sprintf("💸 Send %d sat.", amount),
				Description: fmt.Sprintf("You will send %d sat in this chat.", amount),
				// required for photos
				ThumbURL: url,
			}
			sendInlineMenu.Inline(sendInlineMenu.Row(btnSendInline, btnCancelSendInline))

			result.ReplyMarkup = &tb.InlineKeyboardMarkup{InlineKeyboard: sendInlineMenu.InlineKeyboard}

			results[i] = result
			// needed to set a unique string ID for each result
			// results[i].SetResultID(strconv.Itoa(i))
			results[i].SetResultID(fmt.Sprintf("inline-send-%d-%d-%d", q.From.ID, amount, i))

			// add result to persistent struct
			inlineSend.result = result
			runtime.IgnoreError(bot.bunt.Set(inlineSend))
		}

		err = bot.telegram.Answer(q, &tb.QueryResponse{
			Results:   results,
			CacheTime: 1, // 60 == 1 minute, todo: make higher than 1 s in production

		})

		if err != nil {
			log.Println(err)
		}
	}
}
