package telegram

import (
	"context"
	"encoding/json"
	"fmt"

	tb "gopkg.in/lightningtipbot/telebot.v2"
)

func (bot TipBot) groupHandler(ctx context.Context, m *tb.Message) (context.Context, error) {
	type CreateChatInviteLink struct {
		ChatID             int64  `json:"chat_id"`
		Name               string `json:"name"`
		ExpiryDate         int    `json:"expiry_date"`
		MemberLimit        int    `json:"member_limit"`
		CreatesJoinRequest bool   `json:"creates_join_request"`
	}

	params := map[string]interface {
	}{
		"chat_id": m.Chat.ID,
		"name":    m.Text,
	}
	data, err := bot.Telegram.Raw("createChatInviteLink", params)
	if err != nil {
		return ctx, err
	}

	type ChatInviteLink struct {
		InviteLink              string  `json:"invite_link"`
		Creator                 tb.User `json:"creator"`
		CreatesJoinRequest      bool    `json:"creates_join_request"`
		IsPrimary               bool    `json:"is_primary"`
		IsReovked               bool    `json:"is_reovked"`
		Name                    string  `json:"name"`
		ExpiryDate              int     `json:"expiry_date"`
		MemberLimit             int     `json:"member_limit"`
		PendingJoinRequestCount int     `json:"pending_join_request_count"`
	}

	var resp ChatInviteLink

	bot.trySendMessage(m.Chat, fmt.Sprintf("Data: `%s`", data))

	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}

	bot.trySendMessage(m.Chat, fmt.Sprintf("Link: %s", resp.InviteLink))
	return ctx, nil
}
