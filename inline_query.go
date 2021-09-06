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
	inlineQuerySendTitle    = "Send sats to a chat."
	inlineQueryDescription  = "Usage: @%s send <amount> [<memo>]"
	inlineResultSendTitle   = "💸 Send %d sat."
	inlineResultDescription = "Click here to send %d sat to this chat."
	sendInlineMenu          = &tb.ReplyMarkup{ResizeReplyKeyboard: true}
	btnCancelSendInline     = paymentConfirmationMenu.Data("🚫 Cancel", "cancel_send_inline")
	btnSendInline           = paymentConfirmationMenu.Data("✅ Receive", "confirm_send_inline")
)

type InlineSend struct {
	Message string   `json:"inline_send_message"`
	Amount  int      `json:"inline_send_amount"`
	From    *tb.User `json:"inline_send_from"`
	To      *tb.User `json:"inline_send_to"`
	Memo    string
	ID      string `json:"inline_send_id"`
	Active  bool   `json:"inline_send_active"`
}

func NewInlineSend(m string) *InlineSend {
	inlineSend := &InlineSend{
		Message: m,
	}
	return inlineSend

}

func (msg InlineSend) Key() string {
	return msg.ID
}

func (bot TipBot) inlineQueryInstructions(q *tb.Query) {
	instructions := []struct {
		url         string
		title       string
		description string
	}{
		{
			url:         queryImage,
			title:       inlineQuerySendTitle,
			description: fmt.Sprintf(inlineQueryDescription, bot.telegram.Me.Username),
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
	if strings.HasPrefix(q.Text, "send") || strings.HasPrefix(q.Text, "/send") || strings.HasPrefix(q.Text, "giveaway") || strings.HasPrefix(q.Text, "/giveaway") {
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
				inlineMessage = inlineMessage + fmt.Sprintf(inlineSendAppendMemo, MarkdownEscape(memo))
			}

			result := &tb.ArticleResult{
				// URL:         url,
				Text:        inlineMessage,
				Title:       fmt.Sprintf(inlineResultSendTitle, amount),
				Description: fmt.Sprintf(inlineResultDescription, amount),
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
			log.Println(err)
		}
	}
}
