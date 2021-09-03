package main

import (
	"fmt"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"
	tb "gopkg.in/tucnak/telebot.v2"
)

const queryImage = "https://avatars.githubusercontent.com/u/88730856?v=4"

var (
	sendInlineMenu      = &tb.ReplyMarkup{ResizeReplyKeyboard: true}
	btnCancelSendInline = paymentConfirmationMenu.Data("🚫 Cancel", "cancel_send_inline")
	btnSendInline       = paymentConfirmationMenu.Data("✅ Accept", "confirm_send_inline")
)

func (bot TipBot) inlineQueryInstructions(q *tb.Query) {
	instructions := []struct {
		url         string
		title       string
		description string
	}{
		{
			url:         queryImage,
			title:       "💸 Send sats to the person you chat with.",
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
			result := &tb.ArticleResult{
				URL:         url,
				Text:        fmt.Sprintf("Press ✅ to accept payment.\n\n💸 Amount: %d sat\n", amount),
				Title:       fmt.Sprintf("💸 Send %d sat to chat.", amount),
				Description: fmt.Sprintf("You chat with will receive %d sat if they accept the payment.", amount),
				// required for photos
				ThumbURL: url,
			}
			sendInlineMenu.Inline(sendInlineMenu.Row(btnSendInline, btnCancelSendInline))

			result.ReplyMarkup = &tb.InlineKeyboardMarkup{InlineKeyboard: sendInlineMenu.InlineKeyboard}

			results[i] = result
			// needed to set a unique string ID for each result
			results[i].SetResultID(strconv.Itoa(i))
		}

		err = bot.telegram.Answer(q, &tb.QueryResponse{
			Results:   results,
			CacheTime: 1, // a minute

		})

		if err != nil {
			log.Println(err)
		}
	}
}
