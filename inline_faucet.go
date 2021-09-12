package main

import (
	"fmt"

	"github.com/LightningTipBot/LightningTipBot/internal/runtime"
	log "github.com/sirupsen/logrus"
	tb "gopkg.in/tucnak/telebot.v2"
)

const (
	inlineFaucetMessage             = "Press ✅ to collect.\n\n🏅 Balance: %d/%d sat (%d users)"
	inlineFaucetAppendMemo          = "\n✉️ %s"
	inlineFaucetUpdateMessageAccept = "💸 %d sat sent from %s to %s."
	inlineFaucetCreateWalletMessage = "Chat with %s 👈 to manage your wallet."
	inlineFaucetYourselfMessage     = "📖 You can't pay to yourself."
	inlineFaucetFailedMessage       = "🚫 Send failed."
	inlineFaucetCancelledMessage    = "🚫 Faucet cancelled."
)

var (
	inlineQueryFaucetTitle        = "Create a faucet."
	inlineQueryFaucetDescription  = "Usage: @%s faucet <capacity> <per_user>"
	inlineResultFaucetTitle       = "💸 Create a %d sat faucet."
	inlineResultFaucetDescription = "👉 Click here to create a faucet worth %d sat to this chat."
	faucetInlineMenu              = &tb.ReplyMarkup{ResizeReplyKeyboard: true}
	btnCancelFaucetInline         = paymentConfirmationMenu.Data("🚫 Cancel", "cancel_faucet_inline")
	btnFaucetInline               = paymentConfirmationMenu.Data("✅ Receive", "confirm_faucet_inline")
)

type InlineFaucet struct {
	Message         string     `json:"inline_faucet_message"`
	Amount          int        `json:"inline_faucet_amount"`
	RemainingAmount int        `json:"inline_faucet_remainingamount"`
	PerUserAmount   int        `json:"inline_faucet_peruseramount"`
	From            *tb.User   `json:"inline_faucet_from"`
	To              []*tb.User `json:"inline_faucet_to"`
	Memo            string
	ID              string `json:"inline_faucet_id"`
	Active          bool   `json:"inline_faucet_active"`
	NTotal          int    `json:"inline_faucet_ntotal"`
	NTaken          int    `json:"inline_faucet_ntaken"`
}

func NewInlineFaucet(m string) *InlineFaucet {
	inlineFaucet := &InlineFaucet{
		Message: m,
		NTaken:  0,
	}
	return inlineFaucet

}

func (msg InlineFaucet) Key() string {
	return msg.ID
}

// tipTooltipExists checks if this tip is already known
func (bot *TipBot) getInlineFaucet(c *tb.Callback) (*InlineFaucet, error) {
	inlineFaucet := NewInlineFaucet("")
	inlineFaucet.ID = c.Data
	err := bot.bunt.Get(inlineFaucet)
	if err != nil {
		return nil, fmt.Errorf("could not get inline faucet")
	}
	return inlineFaucet, nil

}

func (bot *TipBot) inlineFaucetHandler(c *tb.Callback) {
	inlineFaucet, err := bot.getInlineFaucet(c)
	if err != nil {
		log.Errorf("[sendInlineHandler] %s", err)
		return
	}
	if !inlineFaucet.Active {
		log.Errorf("[sendInlineHandler] inline send not active anymore")
		return
	}
	amount := inlineFaucet.Amount
	peruser := inlineFaucet.PerUserAmount
	to := c.Sender
	from := inlineFaucet.From
	remaining := inlineFaucet.RemainingAmount
	ntaken := inlineFaucet.NTaken

	// if from.ID == to.ID {
	// 	bot.trySendMessage(from, sendYourselfMessage)
	// 	return
	// }

	// check if to user has already taken from the faucet
	for _, a := range inlineFaucet.To {
		if a.ID == to.ID {
			// to user is already in To slice, has taken from facuet
			return
		}
	}

	toUserStrMd := GetUserStrMd(to)
	fromUserStrMd := GetUserStrMd(from)
	toUserStr := GetUserStr(to)
	fromUserStr := GetUserStr(from)
	// check if user exists and create a wallet if not
	_, exists := bot.UserExists(to)
	if !exists {
		log.Infof("[sendInline] User %s has no wallet.", toUserStr)
		err = bot.CreateWalletForTelegramUser(to)
		if err != nil {
			errmsg := fmt.Errorf("[sendInline] Error: Could not create wallet for %s", toUserStr)
			log.Errorln(errmsg)
			return
		}
	}

	// todo: user new get username function to get userStrings
	transactionMemo := fmt.Sprintf("Faucet from %s to %s (%d sat).", fromUserStr, toUserStr, peruser)
	t := NewTransaction(bot, from, to, peruser, TransactionType("inline faucet"))
	t.Memo = transactionMemo
	success, err := t.Send()
	if !success {
		if err != nil {
			bot.trySendMessage(from, fmt.Sprintf(tipErrorMessage, err))
		} else {
			bot.trySendMessage(from, fmt.Sprintf(tipErrorMessage, tipUndefinedErrorMsg))
		}
		errMsg := fmt.Sprintf("[sendInline] Transaction failed: %s", err)
		log.Errorln(errMsg)
		// bot.tryEditMessage(c.Message, inlineFaucetFailedMessage, &tb.ReplyMarkup{})
		return
	}

	log.Infof("[sendInline] %d sat from %s to %s", peruser, fromUserStr, toUserStr)
	ntaken += 1
	inlineFaucet.Message = fmt.Sprintf("%s", fmt.Sprintf(inlineFaucetMessage, remaining, amount, ntaken))
	memo := inlineFaucet.Memo
	if len(memo) > 0 {
		inlineFaucet.Message = inlineFaucet.Message + fmt.Sprintf(inlineFaucetAppendMemo, memo)
	}

	// if !bot.UserInitializedWallet(to) {
	// 	inlineFaucet.Message += "\n\n" + fmt.Sprintf(sendInlineCreateWalletMessage, GetUserStrMd(bot.telegram.Me))
	// }
	// btnFaucetInline.Data = inlineFaucet.ID
	// btnCancelFaucetInline.Data = inlineFaucet.ID
	// bot.telegram.Handle(&btnFaucetInline, bot.inlineFaucetHandler)
	// bot.telegram.Handle(&btnCancelFaucetInline, bot.cancelinlineFaucetHandler)
	faucetInlineMenu.Inline(faucetInlineMenu.Row(btnFaucetInline, btnCancelFaucetInline))

	log.Infoln(inlineFaucet.Message)
	bot.tryEditMessage(c.Message, inlineFaucet.Message, faucetInlineMenu)
	// notify users
	_, err = bot.telegram.Send(to, fmt.Sprintf(sendReceivedMessage, fromUserStrMd, peruser))
	_, err = bot.telegram.Send(from, fmt.Sprintf(tipSentMessage, peruser, toUserStrMd))
	if err != nil {
		errmsg := fmt.Errorf("[sendInline] Error: Send message to %s: %s", toUserStr, err)
		log.Errorln(errmsg)
		return
	}

	// edit persistent object and store it
	inlineFaucet.To = append(inlineFaucet.To, to)
	inlineFaucet.RemainingAmount = inlineFaucet.RemainingAmount - peruser
	inlineFaucet.NTaken = ntaken

	runtime.IgnoreError(bot.bunt.Set(inlineFaucet))

}

func (bot *TipBot) cancelinlineFaucetHandler(c *tb.Callback) {
	inlineFaucet, err := bot.getInlineFaucet(c)
	if err != nil {
		log.Errorf("[cancelSendInlineHandler] %s", err)
		return
	}
	if c.Sender.ID == inlineFaucet.From.ID {
		bot.tryEditMessage(c.Message, inlineFaucetCancelledMessage, &tb.ReplyMarkup{})
		// set the inlineFaucet inactive
		inlineFaucet.Active = false
		runtime.IgnoreError(bot.bunt.Set(inlineFaucet))
	}
	return
}
