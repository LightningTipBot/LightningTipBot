package telegram

import tb "gopkg.in/tucnak/telebot.v2"

func getNextItemButton(data string) tb.Btn {
	return shopMainMenu.Data(">", "shop_nextitem", data)
}
func selectShopButtons(shops []*Shop) []tb.Btn {
	var buttons []tb.Btn
	for _, shop := range shops {
		buttons = append(buttons, shopMainMenu.Data(shop.Title, "select_shop", shop.ID))
	}
	return buttons
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
