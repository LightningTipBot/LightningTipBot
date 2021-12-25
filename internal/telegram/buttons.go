package telegram

import tb "gopkg.in/lightningtipbot/telebot.v2"

const (
	CommandSend    = "💸 Send"
	CommandBalance = "👑 Balance"
	CommandInvoice = "⚡️ Invoice"
	CommandHelp    = "📖 Help"
)

var (
	mainMenu           = &tb.ReplyMarkup{ResizeReplyKeyboard: true}
	btnHelpMainMenu    = mainMenu.Text(CommandHelp)
	btnSendMainMenu    = mainMenu.Text(CommandSend)
	btnBalanceMainMenu = mainMenu.Text(CommandBalance)
	btnInvoiceMainMenu = mainMenu.Text(CommandInvoice)
)

func init() {
	mainMenu.Reply(
		mainMenu.Row(btnBalanceMainMenu, btnHelpMainMenu),
		mainMenu.Row(btnInvoiceMainMenu, btnSendMainMenu),
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
