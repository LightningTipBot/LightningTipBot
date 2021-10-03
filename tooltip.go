package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/LightningTipBot/LightningTipBot/internal/runtime"
	"github.com/tidwall/buntdb"
	"github.com/tidwall/gjson"

	log "github.com/sirupsen/logrus"

	tb "gopkg.in/tucnak/telebot.v2"
)

const (
	tooltipChatWithBotMessage  = "🗑 Chat with %s 👈 to manage your wallet."
	tooltipAndOthersMessage    = " ... and others"
	tooltipMultipleTipsMessage = "%s (%d tips by %s)"
	tooltipSingleTipMessage    = "%s (by %s)"
	tooltipTipAmountMessage    = "🏅 %d sat"
)

type TipTooltip struct {
	Message
	TipAmount int        `json:"tip_amount"`
	Ntips     int        `json:"ntips"`
	LastTip   time.Time  `json:"last_tip"`
	Tippers   []*tb.User `json:"tippers"`
}

const maxNamesInTipperMessage = 5

type TipTooltipOption func(m *TipTooltip)

func TipAmount(amount int) TipTooltipOption {
	return func(m *TipTooltip) {
		m.TipAmount = amount
	}
}
func Tips(nTips int) TipTooltipOption {
	return func(m *TipTooltip) {
		m.LastTip = time.Now()
		m.Ntips = nTips
	}
}

func NewTipTooltip(m *tb.Message, opts ...TipTooltipOption) *TipTooltip {
	tipTooltip := &TipTooltip{
		Message: Message{
			Message: m,
		},
	}
	for _, opt := range opts {
		opt(tipTooltip)
	}
	return tipTooltip

}

// getUpdatedTipTooltipMessage will return the full tip tool tip
func (ttt TipTooltip) getUpdatedTipTooltipMessage(botUserName string, notInitializedWallet bool) string {
	tippersStr := getTippersString(ttt.Tippers)
	tipToolTipMessage := fmt.Sprintf(tooltipTipAmountMessage, ttt.TipAmount)
	if len(ttt.Tippers) > 1 {
		tipToolTipMessage = fmt.Sprintf(tooltipMultipleTipsMessage, tipToolTipMessage, ttt.Ntips, tippersStr)
	} else {
		tipToolTipMessage = fmt.Sprintf(tooltipSingleTipMessage, tipToolTipMessage, tippersStr)
	}

	if notInitializedWallet {
		tipToolTipMessage = tipToolTipMessage + fmt.Sprintf("\n%s", fmt.Sprintf(tooltipChatWithBotMessage, botUserName))
	}
	return tipToolTipMessage
}

// getTippersString joins all tippers username or telegram id's as mentions (@username or [inline mention of a user](tg://user?id=123456789))
func getTippersString(tippers []*tb.User) string {
	var tippersStr string
	for _, uniqueUser := range tippers {
		userStr := GetUserStrMd(uniqueUser)
		tippersStr += fmt.Sprintf("%s, ", userStr)
	}
	// get rid of the trailing comma
	if len(tippersStr) > 2 {
		tippersStr = tippersStr[:len(tippersStr)-2]
	}
	tippersSlice := strings.Split(tippersStr, " ")
	// crop the message to the max length
	if len(tippersSlice) > maxNamesInTipperMessage {
		// tippersStr = tippersStr[:50]
		tippersStr = strings.Join(tippersSlice[:maxNamesInTipperMessage], " ")
		tippersStr = tippersStr + tooltipAndOthersMessage
	}
	return tippersStr
}

// tipTooltipExists checks if this tip is already known
func tipTooltipExists(replyToId int, bot *TipBot) (bool, *TipTooltip) {
	message := NewTipTooltip(&tb.Message{ReplyTo: &tb.Message{ID: replyToId}})
	err := bot.bunt.Get(message)
	if err != nil {
		return false, message
	}
	return true, message

}

// tipTooltipHandler function to update the tooltip below a tipped message. either updates or creates initial tip tool tip
func tipTooltipHandler(m *tb.Message, bot *TipBot, amount int, initializedWallet bool) (hasTip bool) {
	// todo: this crashes if the tooltip message (maybe also the original tipped message) was deleted in the mean time!!! need to check for existence!
	hasTip, ttt := tipTooltipExists(m.ReplyTo.ID, bot)
	if hasTip {
		// update the tooltip with new tippers
		err := ttt.updateTooltip(bot, m.Sender, amount, !initializedWallet)
		if err != nil {
			log.Println(err)
			// could not update the message (return false to )
			return false
		}
	} else {
		tipmsg := fmt.Sprintf(tooltipTipAmountMessage, amount)
		userStr := GetUserStrMd(m.Sender)
		tipmsg = fmt.Sprintf(tooltipSingleTipMessage, tipmsg, userStr)

		if !initializedWallet {
			tipmsg = tipmsg + fmt.Sprintf("\n%s", fmt.Sprintf(tooltipChatWithBotMessage, GetUserStrMd(bot.telegram.Me)))
		}
		msg, err := bot.telegram.Reply(m.ReplyTo, tipmsg, tb.Silent)
		if err != nil {
			log.Errorf("[tipTooltipHandler Reply] %s", err.Error())
			// todo: in case of error we should do something better than just return 0
			return false
		}
		message := NewTipTooltip(msg, TipAmount(amount), Tips(1))
		message.Tippers = appendUinqueUsersToSlice(message.Tippers, m.Sender)
		runtime.IgnoreError(bot.bunt.Set(message))
	}
	// first call will return false, every following call will return true
	return hasTip
}

// updateToolTip updates existing tip tool tip in telegram
func (ttt *TipTooltip) updateTooltip(bot *TipBot, user *tb.User, amount int, notInitializedWallet bool) error {
	ttt.TipAmount += amount
	ttt.Ntips += 1
	ttt.Tippers = appendUinqueUsersToSlice(ttt.Tippers, user)
	ttt.LastTip = time.Now()
	err := ttt.editTooltip(bot, notInitializedWallet)
	if err != nil {
		return err
	}
	return bot.bunt.Set(ttt)
}

// tipTooltipInitializedHandler is called when the user initializes the wallet
func tipTooltipInitializedHandler(user *tb.User, bot TipBot) {
	runtime.IgnoreError(bot.bunt.View(func(tx *buntdb.Tx) error {
		err := tx.Ascend(MessageOrderedByReplyToFrom, func(key, value string) bool {
			replyToUserId := gjson.Get(value, MessageOrderedByReplyToFrom)
			if replyToUserId.String() == strconv.Itoa(user.ID) {
				log.Debugln("loading persistent tip tool tip messages")
				ttt := &TipTooltip{}
				err := json.Unmarshal([]byte(value), ttt)
				if err != nil {
					log.Errorln(err)
				}
				err = ttt.editTooltip(&bot, false)
				if err != nil {
					log.Errorf("[tipTooltipInitializedHandler] could not edit tooltip: %s", err.Error())
				}
			}

			return true
		})
		return err
	}))
}

func (ttt TipTooltip) Key() string {
	return fmt.Sprintf("tip-tool-tip:%s", strconv.Itoa(ttt.Message.Message.ReplyTo.ID))
}

// editTooltip updates the tooltip message with the new tip amount and tippers and edits it
func (ttt *TipTooltip) editTooltip(bot *TipBot, notInitializedWallet bool) error {
	tipToolTip := ttt.getUpdatedTipTooltipMessage(GetUserStrMd(bot.telegram.Me), notInitializedWallet)
	m, err := bot.telegram.Edit(ttt.Message.Message, tipToolTip)
	if err != nil {
		return err
	}
	ttt.Message.Message.Text = m.Text
	return nil
}
