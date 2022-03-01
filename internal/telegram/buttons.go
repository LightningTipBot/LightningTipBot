package telegram

import (
	"context"
	"fmt"

	"github.com/LightningTipBot/LightningTipBot/internal/lnbits"
	log "github.com/sirupsen/logrus"
	tb "gopkg.in/lightningtipbot/telebot.v2"
)

// we can't use space in the label of buttons, because string splitting will mess everything up.
const (
	MainMenuCommandSend    = "💸 Send"
	MainMenuCommandBalance = "Balance"
	MainMenuCommandInvoice = "⚡️ Invoice"
	MainMenuCommandHelp    = "📖 Help"
	SendMenuCommandEnter   = "👤 Enter"
)

var (
	mainMenu           = &tb.ReplyMarkup{ResizeReplyKeyboard: true}
	btnHelpMainMenu    = mainMenu.Text(MainMenuCommandHelp)
	btnSendMainMenu    = mainMenu.Text(MainMenuCommandSend)
	btnBalanceMainMenu = mainMenu.Text(MainMenuCommandBalance)
	btnInvoiceMainMenu = mainMenu.Text(MainMenuCommandInvoice)

	sendToMenu       = &tb.ReplyMarkup{ResizeReplyKeyboard: true}
	sendToButtons    = []tb.Btn{}
	btnSendMenuEnter = mainMenu.Text(SendMenuCommandEnter)
)

func init() {
	mainMenu.Reply(
		mainMenu.Row(btnBalanceMainMenu),
		mainMenu.Row(btnInvoiceMainMenu, btnSendMainMenu, btnHelpMainMenu),
	)
}

// buttonWrapper wrap buttons slice in rows of length i
func buttonWrapper(buttons []tb.Btn, markup *tb.ReplyMarkup, length int) []tb.Row {
	buttonLength := len(buttons)
	rows := make([]tb.Row, 0)

	if buttonLength > length {
		for i := 0; i < buttonLength; i = i + length {
			buttonRow := make([]tb.Btn, length)
			if i+length < buttonLength {
				buttonRow = buttons[i : i+length]
			} else {
				buttonRow = buttons[i:]
			}
			rows = append(rows, markup.Row(buttonRow...))
		}
		return rows
	}
	rows = append(rows, markup.Row(buttons...))
	return rows
}

// mainMenuBalanceButtonUpdate updates the balance button in the mainMenu
func (bot *TipBot) mainMenuBalanceButtonUpdate(to int64) {
	var user *lnbits.User
	var err error
	if user, err = getCachedUser(&tb.User{ID: to}, *bot); err != nil {
		user, err = GetLnbitsUser(&tb.User{ID: to}, *bot)
		if err != nil {
			return
		}
		updateCachedUser(user, *bot)
	}
	if user.Wallet != nil {
		amount, err := bot.GetUserBalanceCached(user)
		if err != nil {
			amount = 0
		}
		log.Tracef("[appendMainMenu] user %s balance %d sat", GetUserStr(user.Telegram), amount)
		MainMenuCommandBalance := fmt.Sprintf("%s %d sat", MainMenuCommandBalance, amount)
		btnBalanceMainMenu = mainMenu.Text(MainMenuCommandBalance)
		mainMenu.Reply(
			mainMenu.Row(btnBalanceMainMenu),
			mainMenu.Row(btnInvoiceMainMenu, btnSendMainMenu, btnHelpMainMenu),
		)
	}
}

// makeContactsButtons will create a slice of buttons for the send menu
// it will show the 5 most recently interacted contacts and one button to use a custom contact
func (bot *TipBot) makeContactsButtons(ctx context.Context) []tb.Btn {
	var records []Transaction

	sendToButtons = []tb.Btn{}
	user := LoadUser(ctx)
	// get 5 most recent transactions by from_id with distint to_user
	// where to_user starts with an @ and is not the user itself
	bot.DB.Transactions.Where("from_id = ? AND to_user LIKE ? AND to_user <> ?", user.Telegram.ID, "@%", GetUserStr(user.Telegram)).Distinct("to_user").Order("id desc").Limit(5).Find(&records)
	log.Debugf("[makeContactsButtons] found %d records", len(records))

	// get all contacts and add them to the buttons
	for i, r := range records {
		log.Tracef("[makeContactsButtons] toNames[%d] = %s (id=%d)", i, r.ToUser, r.ID)
		sendToButtons = append(sendToButtons, tb.Btn{Text: fmt.Sprintf("%s", r.ToUser)})
	}

	// add the "enter a username" button to the end
	sendToButtons = append(sendToButtons, tb.Btn{Text: SendMenuCommandEnter})
	sendToMenu.Reply(buttonWrapper(sendToButtons, sendToMenu, 3)...)
	return sendToButtons
}

// appendMainMenu will check if to (recipient) ID is from private or group chat.
// appendMainMenu is called in telegram.go every time a user receives a PM from the bot.
// this function will only add a keyboard if this is a private chat and no force reply.
func (bot *TipBot) appendMainMenu(to int64, recipient interface{}, options []interface{}) []interface{} {

	// update the balance button
	if to > 0 {
		bot.mainMenuBalanceButtonUpdate(to)
	}

	appendKeyboard := true
	for _, option := range options {
		if option == tb.ForceReply {
			appendKeyboard = false
		}
		switch option.(type) {
		case *tb.ReplyMarkup:
			appendKeyboard = false
			//option.(*tb.ReplyMarkup).ReplyKeyboard = mainMenu.ReplyKeyboard
			//if option.(*tb.ReplyMarkup).InlineKeyboard == nil {
			//	options = append(options[:i], options[i+1:]...)
			//}
		}
	}
	// to > 0 is private chats
	if to > 0 && appendKeyboard {
		options = append(options, mainMenu)
	}
	return options
}
