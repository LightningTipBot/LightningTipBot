package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/LightningTipBot/LightningTipBot/internal/lnbits"
	"github.com/LightningTipBot/LightningTipBot/internal/runtime"
	"github.com/LightningTipBot/LightningTipBot/internal/storage/transaction"
	"github.com/eko/gocache/store"
	log "github.com/sirupsen/logrus"
	tb "gopkg.in/tucnak/telebot.v2"
)

type ShopView struct {
	ID             string
	ShopID         string
	Page           int
	Message        *tb.Message
	StatusMessages []*tb.Message
}

type ShopItem struct {
	ID           string       `json:"ID"`          // ID of the tx object in bunt db
	ShopID       string       `json:"shopID"`      // ID of the shop
	Owner        *lnbits.User `json:"owner"`       // Owner of the item
	Type         string       `json:"Type"`        // Type of the tx object in bunt db
	FileID       string       `json:"fileID"`      // Telegram fileID of the item
	Title        string       `json:"title"`       // Title of the item
	Description  string       `json:"description"` // Description of the item
	Price        int          `json:"price"`       // price of the item
	NSold        int          `json:"nSold"`       // number of times item was sold
	TbPhoto      *tb.Photo    `json:"tbPhoto"`     // Telegram photo object
	LanguageCode string       `json:"languagecode"`
}

type Shop struct {
	*transaction.Base
	ID           string              `json:"ID"`          // holds the ID of the tx object in bunt db
	Owner        *lnbits.User        `json:"owner"`       // owner of the shop
	Type         string              `json:"Type"`        // type of the shop
	Title        string              `json:"title"`       // Title of the item
	Description  string              `json:"description"` // Description of the item
	ItemIds      []string            `json:"ItemsIDs"`    //
	Items        map[string]ShopItem `json:"Items"`       //
	LanguageCode string              `json:"languagecode"`
	ShopsID      string              `json:"shopsID"`
}

type Shops struct {
	*transaction.Base
	ID    string       `json:"ID"`    // holds the ID of the tx object in bunt db
	Owner *lnbits.User `json:"owner"` // owner of the shop
	Shops []string     `json:"shop"`  //
}

func (shop *Shop) getItem(itemId string) (item ShopItem, ok bool) {
	item, ok = shop.Items[itemId]
	return
}

var (
	shopKeyboard         = &tb.ReplyMarkup{ResizeReplyKeyboard: false}
	browseShopButton     = shopKeyboard.Data("Browse shops", "shops_browse")
	shopNewShopButton    = shopKeyboard.Data("New Shop", "shops_newshop")
	shopDeleteShopButton = shopKeyboard.Data("Delete Shop", "shops_deleteshop")
	shopSettingsButton   = shopKeyboard.Data("Settings", "shops_settings")
	shopShopsButton      = shopKeyboard.Data("Back", "shops_shops")

	shopAddItemButton  = shopKeyboard.Data("Add item", "shop_additem")
	shopNextitemButton = shopKeyboard.Data(">", "shop_nextitem")
	shopPrevitemButton = shopKeyboard.Data("<", "shop_previtem")
	shopBuyitemButton  = shopKeyboard.Data("Buy", "shop_buyitem")

	shopSelectButton           = shopKeyboard.Data("SHOP SELECTOR", "select_shop")        // shop slectino buttons
	shopDeleteSelectButton     = shopKeyboard.Data("DELETE SHOP SELECTOR", "delete_shop") // shop slectino buttons
	shopItemPriceButton        = shopKeyboard.Data("Price", "shop_itemprice")
	shopItemDeleteButton       = shopKeyboard.Data("Delete", "shop_itemdelete")
	shopItemTitleButton        = shopKeyboard.Data("Set title", "shop_itemtitle")
	shopItemSettingsButton     = shopKeyboard.Data("Item settings", "shop_itemsettings")
	shopItemSettingsBackButton = shopKeyboard.Data("Back", "shop_itemsettingsback")
)

// shopItemPriceHandler is invoked when the user presses the item settings button to set a price
func (bot *TipBot) shopItemPriceHandler(ctx context.Context, c *tb.Callback) {
	user := LoadUser(ctx)
	_, err := bot.getUserShops(ctx, user)
	if err != nil {
		// _, err = bot.initUserShops(ctx, user)
		// if err != nil {
		// 	log.Errorf("[shopNewShopHandler] %s", err)
		// 	return
		// }
		log.Errorf("[shopNewShopHandler] %s", err)
		return
	}
	shop, err := bot.addUserShop(ctx, user)
	// We need to save the pay state in the user state so we can load the payment in the next handler
	SetUserState(user, bot, lnbits.UserEnterShopTitle, shop.ID)
	statusMsg := bot.trySendMessage(c.Sender, fmt.Sprintf("Enter the name of your shop"), tb.ForceReply)
	shopView, err := bot.getUserShopview(ctx, user)
	if err == nil {
		shopView.StatusMessages = append(shopView.StatusMessages, statusMsg)
	}
	bot.Cache.Set(shopView.ID, shopView, &store.Options{Expiration: 24 * time.Hour})
}

// enterShopItemPriceHandler is invoked when the user enters a price amount
func (bot *TipBot) enterShopItemPriceHandler(ctx context.Context, m *tb.Message) {
	user := LoadUser(ctx)
	// read item from user.StateData
	shop, err := bot.getShop(ctx, user.StateData)
	if err != nil {
		return
	}
	if shop.Owner.Telegram.ID != m.Sender.ID {
		return
	}
	shop.Title = m.Text
	runtime.IgnoreError(shop.Set(shop, bot.Bunt))
	bot.shopsHandler(ctx, m)
	bot.tryDeleteMessage(m)
	statusMsg := bot.trySendMessage(m.Sender, fmt.Sprintf("✅ Shop added."))
	shopView, err := bot.getUserShopview(ctx, user)
	if err == nil {
		shopView.StatusMessages = append(shopView.StatusMessages, statusMsg)
		bot.Cache.Set(shopView.ID, shopView, &store.Options{Expiration: 24 * time.Hour})
		time.Sleep(time.Duration(2) * time.Second)
		bot.shopViewDeleteAllStatusMsgs(ctx, user)
	}
}

// shopItemSettingsHandler is invoked when the user presses the item settings button
func (bot *TipBot) shopItemSettingsHandler(ctx context.Context, c *tb.Callback) {
	user := LoadUser(ctx)
	shopView, err := bot.getUserShopview(ctx, user)
	if err != nil {
		return
	}
	shop, err := bot.getShop(ctx, shopView.ShopID)
	item := shop.Items[shop.ItemIds[shopView.Page]]
	// sanity check
	if item.ID != c.Data {
		log.Error("[shopItemSettingsHandler] item id mismatch")
	}
	bot.tryEditMessage(shopView.Message, item.TbPhoto, bot.shopItemSettingsMenu(ctx, shop, &item))
}

// displayShopItemHandler is invoked when the user presses the back button in the item settings
func (bot *TipBot) displayShopItemHandler(ctx context.Context, c *tb.Callback) {
	user := LoadUser(ctx)
	shopView, err := bot.getUserShopview(ctx, user)
	if err != nil {
		return
	}
	shop, err := bot.getShop(ctx, shopView.ShopID)
	item := shop.Items[shop.ItemIds[shopView.Page]]
	// sanity check
	if item.ID != c.Data {
		log.Error("[shopItemSettingsHandler] item id mismatch")
	}
	bot.displayShopItem(ctx, c.Message, shop)
}

// shopNextItemHandler is invoked when the user presses the next item button
func (bot *TipBot) shopNextItemButtonHandler(ctx context.Context, c *tb.Callback) {
	user := LoadUser(ctx)
	// shopView, err := bot.Cache.Get(fmt.Sprintf("shopview-%d", user.Telegram.ID))
	shopView, err := bot.getUserShopview(ctx, user)
	if err != nil {
		return
	}
	shop, err := bot.getShop(ctx, shopView.ShopID)
	if shopView.Page < len(shop.Items)-1 {
		shopView.Page++
		bot.Cache.Set(shopView.ID, shopView, &store.Options{Expiration: 24 * time.Hour})
		shop, err = bot.getShop(ctx, shopView.ShopID)
		bot.displayShopItem(ctx, c.Message, shop)
	}
}

// shopPrevItemButtonHandler is invoked when the user presses the previous item button
func (bot *TipBot) shopPrevItemButtonHandler(ctx context.Context, c *tb.Callback) {
	user := LoadUser(ctx)
	shopView, err := bot.getUserShopview(ctx, user)
	if err != nil {
		return
	}
	if shopView.Page == 0 {
		bot.shopsHandler(ctx, c.Message)
		return
	}
	if shopView.Page > 0 {
		shopView.Page--
	}
	bot.Cache.Set(shopView.ID, shopView, &store.Options{Expiration: 24 * time.Hour})
	shop, err := bot.getShop(ctx, shopView.ShopID)
	bot.displayShopItem(ctx, c.Message, shop)
}

// displayShopItem renders the current item in the shopView
func (bot *TipBot) displayShopItem(ctx context.Context, m *tb.Message, shop *Shop) *tb.Message {
	user := LoadUser(ctx)
	shopView, err := bot.getUserShopview(ctx, user)
	if err != nil {
		log.Errorf("[displayShopItem] %s", err.Error())
		return nil
	}
	item := shop.Items[shop.ItemIds[shopView.Page]]

	if len(item.Title) > 0 {
		item.TbPhoto.Caption = fmt.Sprintf("%s", item.Title)
	}
	var msg *tb.Message
	if m.Photo != nil {
		// can only edit photo messages with another photo
		msg = bot.tryEditMessage(m, item.TbPhoto, bot.shopMenu(ctx, shop, &item))
	}
	if msg == nil {
		// if editing has failed
		if m != nil {
			bot.tryDeleteMessage(m)
		}
		msg = bot.trySendMessage(m.Chat, item.TbPhoto, bot.shopMenu(ctx, shop, &item))
		shopView.Message = msg
		shopView.Page = 0
	}
	bot.Cache.Set(shopView.ID, shopView, &store.Options{Expiration: 24 * time.Hour})
	return msg
}

// shopHandler is invoked when the user enters /shop
func (bot *TipBot) shopHandler(ctx context.Context, m *tb.Message) {
	if !m.Private() {
		return
	}
	user := LoadUser(ctx)
	shops, err := bot.getUserShops(ctx, user)
	if err != nil {
		log.Errorf("[shopHandler] %s", err)
		return
	}

	shop, err := bot.getShop(ctx, shops.Shops[len(shops.Shops)-1])

	if shop.Owner.Telegram.ID != m.Sender.ID {
		return
	}

	shopView := ShopView{
		ID:     fmt.Sprintf("shopview-%d", user.Telegram.ID),
		ShopID: shop.ID,
		Page:   0,
	}
	bot.Cache.Set(shopView.ID, shopView, &store.Options{Expiration: 24 * time.Hour})
	shopMessage := &tb.Message{Chat: m.Chat}
	if len(shop.ItemIds) > 0 {
		// item := shop.Items[shop.ItemIds[shopView.Page]]
		// shopMessage = bot.trySendMessage(m.Chat, item.TbPhoto, bot.shopMenu(ctx, shop, &item))
		shopMessage = bot.displayShopItem(ctx, shopMessage, shop)
	} else {
		shopMessage = bot.trySendMessage(m.Chat, "No items in shop.", bot.shopMenu(ctx, shop, &ShopItem{}))
	}
	shopView.Message = shopMessage
	bot.Cache.Set(shopView.ID, shopView, &store.Options{Expiration: 24 * time.Hour})
	return
}

// shopNewItemHandler is invoked when the user presses the new item button
func (bot *TipBot) shopNewItemHandler(ctx context.Context, c *tb.Callback) {
	user := LoadUser(ctx)
	shop, err := bot.getShop(ctx, c.Data)
	if err != nil {
		log.Errorf("[shopNewItemHandler] %s", err)
		return
	}
	// We need to save the pay state in the user state so we can load the payment in the next handler
	paramsJson, err := json.Marshal(shop)
	if err != nil {
		log.Errorf("[lnurlWithdrawHandler] Error: %s", err.Error())
		// bot.trySendMessage(m.Sender, err.Error())
		return
	}
	SetUserState(user, bot, lnbits.UserStateShopItemSendPhoto, string(paramsJson))
	statusMsg := bot.trySendMessage(c.Sender, fmt.Sprintf("🌄 Send me an image ."))
	shopView, err := bot.getUserShopview(ctx, user)
	if err == nil {
		shopView.StatusMessages = append(shopView.StatusMessages, statusMsg)
	}
	bot.Cache.Set(shopView.ID, shopView, &store.Options{Expiration: 24 * time.Hour})
}

// addShopItem is a helper function for creating a shop item in the database
func (bot *TipBot) addShopItem(ctx context.Context, shopId string) (*Shop, ShopItem, error) {
	shop, err := bot.getShop(ctx, shopId)
	if err != nil {
		log.Errorf("[addShopItem] %s", err)
		return shop, ShopItem{}, err
	}
	user := LoadUser(ctx)
	// onnly the correct user can press
	if shop.Owner.Telegram.ID != user.Telegram.ID {
		return shop, ShopItem{}, fmt.Errorf("not owner")
	}
	err = shop.Lock(shop, bot.Bunt)
	defer shop.Release(shop, bot.Bunt)

	itemId := fmt.Sprintf("item-%s-%s", shop.ID, RandStringRunes(8))
	item := ShopItem{
		ID:           itemId,
		ShopID:       shop.ID,
		Owner:        user,
		Type:         "photo",
		LanguageCode: shop.LanguageCode,
		Price:        10,
	}
	shop.Items[itemId] = item
	shop.ItemIds = append(shop.ItemIds, itemId)
	runtime.IgnoreError(shop.Set(shop, bot.Bunt))
	return shop, shop.Items[itemId], nil
}

// addShopItemPhoto is invoked when the users sends a photo as a new item
func (bot *TipBot) addShopItemPhoto(ctx context.Context, m *tb.Message) {
	user := LoadUser(ctx)
	if user.Wallet == nil {
		return // errors.New("user has no wallet"), 0
	}

	// read item from user.StateData
	var state_shop Shop
	err := json.Unmarshal([]byte(user.StateData), &state_shop)
	if err != nil {
		log.Errorf("[lnurlWithdrawHandlerWithdraw] Error: %s", err.Error())
		bot.trySendMessage(m.Sender, Translate(ctx, "errorTryLaterMessage"), Translate(ctx, "errorTryLaterMessage"))
		return
	}
	// todo: can go away

	// shop, err := bot.getShop(ctx, state_shop.ID)
	// if err != nil {
	// 	log.Errorf("[addShopItemPhoto] %s", err)
	// }
	// if shop.Owner.Telegram.ID != m.Sender.ID {
	// 	return
	// }
	// immediatelly set lock to block duplicate calls

	shop, item, err := bot.addShopItem(ctx, state_shop.ID)
	// err = shop.Lock(shop, bot.Bunt)
	// defer shop.Release(shop, bot.Bunt)
	item.FileID = m.Photo.FileID
	item.TbPhoto = m.Photo
	item.Title = m.Caption
	shop.Items[item.ID] = item
	runtime.IgnoreError(shop.Set(shop, bot.Bunt))
	bot.tryDeleteMessage(m)
	statusMsg := bot.trySendMessage(m.Sender, fmt.Sprintf("✅ Image added."))
	shopView, err := bot.getUserShopview(ctx, user)
	if err == nil {
		shopView.StatusMessages = append(shopView.StatusMessages, statusMsg)
		bot.Cache.Set(shopView.ID, shopView, &store.Options{Expiration: 24 * time.Hour})
		time.Sleep(time.Duration(2) * time.Second)
		bot.shopViewDeleteAllStatusMsgs(ctx, user)
		shopView.Page = len(shop.Items) - 1
		bot.Cache.Set(shopView.ID, shopView, &store.Options{Expiration: 24 * time.Hour})
		bot.displayShopItem(ctx, shopView.Message, shop)
	}
}

// -------------- shops handler --------------
var ShopsText = "*Welcome to your shop.*\nYour have %d shops.\n%s\n🔞 Look at me `(8 items for 100 sat each)`\n📚 Audiobooks `(12 items for 1000 sat each)`\n\nPress buttons to add a new shop."

// shopsHandlerCallback is a warpper for shopsHandler for callbacks
func (bot *TipBot) shopsHandlerCallback(ctx context.Context, c *tb.Callback) {
	bot.shopsHandler(ctx, c.Message)
}

// shopsHandler is invoked when the user enters /shops
func (bot *TipBot) shopsHandler(ctx context.Context, m *tb.Message) {
	if !m.Private() {
		return
	}
	user := LoadUser(ctx)

	shops, err := bot.getUserShops(ctx, user)
	if err != nil {
		shops, err = bot.initUserShops(ctx, user)
		if err != nil {
			log.Errorf("[shopsHandler] %s", err)
			return
		}
		bot.trySendMessage(m.Chat, "You have no shops.", bot.shopsMainMenu(ctx, shops))
		return
	}

	shopTitles := ""
	for _, shopId := range shops.Shops {
		shop, err := bot.getShop(ctx, shopId)
		if err != nil {
			log.Errorf("[shopsHandler] %s", err)
			return
		}
		shopTitles += fmt.Sprintf("\n%s", shop.Title)

	}

	// if the user used the command /shops, we will send a new message
	// if the user clicked a button and has a shopview set, we will edit an old message
	shopView, err := bot.getUserShopview(ctx, user)
	if err == nil {
		// hack: this m came from a handler from shopHandlerCallback
		// and has the same ID as the message we want to edit
		var msg *tb.Message
		if shopView.Message.Photo == nil {
			msg = bot.tryEditMessage(shopView.Message, fmt.Sprintf(ShopsText, len(shops.Shops), shopTitles), bot.shopsMainMenu(ctx, shops))
		}
		if msg == nil {
			// if editing has failed, we will send a new message
			bot.tryDeleteMessage(shopView.Message)
			shopsMsg := bot.trySendMessage(m.Chat, fmt.Sprintf(ShopsText, len(shops.Shops), shopTitles), bot.shopsMainMenu(ctx, shops))
			shopView = ShopView{
				ID:      fmt.Sprintf("shopview-%d", m.Sender.ID),
				Message: shopsMsg,
			}
		}
	} else {
		shopsMsg := bot.trySendMessage(m.Chat, fmt.Sprintf(ShopsText, len(shops.Shops), shopTitles), bot.shopsMainMenu(ctx, shops))
		shopView = ShopView{
			ID:      fmt.Sprintf("shopview-%d", m.Sender.ID),
			Message: shopsMsg,
		}
	}

	bot.Cache.Set(shopView.ID, shopView, &store.Options{Expiration: 24 * time.Hour})
	return
}

// shopsDeleteShopBrowser is invoked when the user clicks on "delete shops" and makes a list of all shops
func (bot *TipBot) shopsDeleteShopBrowser(ctx context.Context, c *tb.Callback) {
	user := LoadUser(ctx)

	shops, err := bot.getUserShops(ctx, user)
	if err != nil {
		return
	}
	var s []*Shop
	for _, shopId := range shops.Shops {
		shop, _ := bot.getShop(ctx, shopId)
		if shop.Owner.Telegram.ID != c.Sender.ID {
			return
		}
		s = append(s, shop)
	}
	shopShopsButton := shopKeyboard.Data("Back", "shops_shops", shops.ID)
	shopKeyboard.Inline(buttonWrapper(append(bot.makseShopSelectionButtons(s, "delete_shop"), shopShopsButton), shopKeyboard, 1)...)
	bot.tryEditMessage(c.Message, "Which shop do you want to delete?", shopKeyboard)

}

// shopSelect is invoked when the user has selected a shop to browse
func (bot *TipBot) shopSelect(ctx context.Context, c *tb.Callback) {
	shop, _ := bot.getShop(ctx, c.Data)
	user := LoadUser(ctx)
	shopView, err := bot.getUserShopview(ctx, user)
	if err != nil {
		shopView = ShopView{
			ID:     fmt.Sprintf("shopview-%d", c.Sender.ID),
			ShopID: shop.ID,
			Page:   0,
		}
		bot.Cache.Set(shopView.ID, shopView, &store.Options{Expiration: 24 * time.Hour})
	}
	shopView.Page = 0
	shopView.ShopID = shop.ID

	var shopMessage *tb.Message
	if len(shop.ItemIds) > 0 {
		bot.tryDeleteMessage(c.Message)
		item := shop.Items[shop.ItemIds[shopView.Page]]
		shopMessage = bot.trySendMessage(c.Message.Chat, item.TbPhoto, bot.shopMenu(ctx, shop, &item))
	} else {
		shopMessage = bot.tryEditMessage(c.Message, "No items in shop.", bot.shopMenu(ctx, shop, &ShopItem{}))
	}
	shopView.Message = shopMessage
	bot.Cache.Set(shopView.ID, shopView, &store.Options{Expiration: 24 * time.Hour})
}

// shopSelectDelete is invoked when the user has chosen a shop to delete
func (bot *TipBot) shopSelectDelete(ctx context.Context, c *tb.Callback) {
	shop, _ := bot.getShop(ctx, c.Data)
	user := LoadUser(ctx)
	shops, err := bot.getUserShops(ctx, user)
	if err != nil {
		return
	}
	// first, delete from Shops
	for i, shopId := range shops.Shops {
		if shopId == shop.ID {
			shops.Shops = append(shops.Shops[:i], shops.Shops[i+1:]...)
			break
		}
	}
	runtime.IgnoreError(shops.Set(shops, bot.Bunt))

	// then, delete shop
	runtime.IgnoreError(shop.Delete(shop, bot.Bunt))
}

// shopsBrowser makes a button list of all shops the user can browse
func (bot *TipBot) shopsBrowser(ctx context.Context, c *tb.Callback) {
	user := LoadUser(ctx)

	shops, err := bot.getUserShops(ctx, user)
	if err != nil {
		return
	}
	var s []*Shop
	for _, shopId := range shops.Shops {
		shop, _ := bot.getShop(ctx, shopId)
		if shop.Owner.Telegram.ID != c.Sender.ID {
			return
		}
		s = append(s, shop)
	}
	shopShopsButton := shopKeyboard.Data("Back", "shops_shops", shops.ID)
	shopKeyboard.Inline(buttonWrapper(append(bot.makseShopSelectionButtons(s, "select_shop"), shopShopsButton), shopKeyboard, 3)...)
	shopMessage := bot.tryEditMessage(c.Message, "Select your shop!", shopKeyboard)
	shopView, err := bot.getUserShopview(ctx, user)
	if err != nil {
		shopView.Message = shopMessage
		bot.Cache.Set(shopView.ID, shopView, &store.Options{Expiration: 24 * time.Hour})
	}

}

// shopItemSettingsHandler is invoked when the user presses the shop settings button
func (bot *TipBot) shopSettingsHandler(ctx context.Context, c *tb.Callback) {
	user := LoadUser(ctx)
	shopView, err := bot.getUserShopview(ctx, user)
	if err != nil {
		return
	}
	shops, err := bot.getUserShops(ctx, user)
	if err != nil {
		return
	}
	if shops.ID != c.Data || shops.Owner.Telegram.ID != user.Telegram.ID {
		log.Error("[shopSettingsHandler] item id mismatch")
	}
	bot.tryEditMessage(shopView.Message, shopView.Message.Text, bot.shopsSettingsMenu(ctx, shops))
}

// shopNewShopHandler is invoked when the user presses the new shop button
func (bot *TipBot) shopNewShopHandler(ctx context.Context, c *tb.Callback) {
	user := LoadUser(ctx)
	_, err := bot.getUserShops(ctx, user)
	if err != nil {
		// _, err = bot.initUserShops(ctx, user)
		// if err != nil {
		// 	log.Errorf("[shopNewShopHandler] %s", err)
		// 	return
		// }
		log.Errorf("[shopNewShopHandler] %s", err)
		return
	}
	shop, err := bot.addUserShop(ctx, user)
	// We need to save the pay state in the user state so we can load the payment in the next handler
	SetUserState(user, bot, lnbits.UserEnterShopTitle, shop.ID)
	statusMsg := bot.trySendMessage(c.Sender, fmt.Sprintf("Enter the name of your shop"), tb.ForceReply)
	shopView, err := bot.getUserShopview(ctx, user)
	if err == nil {
		shopView.StatusMessages = append(shopView.StatusMessages, statusMsg)
	}
	bot.Cache.Set(shopView.ID, shopView, &store.Options{Expiration: 24 * time.Hour})
}

// enterShopTitleHandler is invoked when the user enters the shop title
func (bot *TipBot) enterShopTitleHandler(ctx context.Context, m *tb.Message) {
	user := LoadUser(ctx)
	// read item from user.StateData
	shop, err := bot.getShop(ctx, user.StateData)
	if err != nil {
		return
	}
	if shop.Owner.Telegram.ID != m.Sender.ID {
		return
	}
	shop.Title = m.Text
	runtime.IgnoreError(shop.Set(shop, bot.Bunt))
	bot.shopsHandler(ctx, m)
	bot.tryDeleteMessage(m)
	statusMsg := bot.trySendMessage(m.Sender, fmt.Sprintf("✅ Shop added."))
	shopView, err := bot.getUserShopview(ctx, user)
	if err == nil {
		shopView.StatusMessages = append(shopView.StatusMessages, statusMsg)
		bot.Cache.Set(shopView.ID, shopView, &store.Options{Expiration: 24 * time.Hour})
		time.Sleep(time.Duration(2) * time.Second)
		bot.shopViewDeleteAllStatusMsgs(ctx, user)
	}

}
