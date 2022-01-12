package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/LightningTipBot/LightningTipBot/internal/errors"

	"github.com/LightningTipBot/LightningTipBot/internal/i18n"
	"github.com/LightningTipBot/LightningTipBot/internal/lnbits"
	"github.com/LightningTipBot/LightningTipBot/internal/runtime"
	"github.com/LightningTipBot/LightningTipBot/internal/storage"
	"github.com/LightningTipBot/LightningTipBot/internal/str"
	"github.com/eko/gocache/store"
	log "github.com/sirupsen/logrus"
	tb "gopkg.in/lightningtipbot/telebot.v2"
)

type ShopView struct {
	ID             string
	ShopID         string
	ShopOwner      *lnbits.User
	Page           int
	Message        *tb.Message
	StatusMessages []*tb.Message
	Chat           *tb.Chat
}

type ShopItem struct {
	ID           string       `json:"ID"`          // ID of the tx object in bunt db
	ShopID       string       `json:"shopID"`      // ID of the shop
	Owner        *lnbits.User `json:"owner"`       // Owner of the item
	Type         string       `json:"Type"`        // Type of the tx object in bunt db
	FileIDs      []string     `json:"fileIDs"`     // Telegram fileID of the item files
	FileTypes    []string     `json:"fileTypes"`   // Telegram file type of the item files
	Title        string       `json:"title"`       // Title of the item
	Description  string       `json:"description"` // Description of the item
	Price        int64        `json:"price"`       // price of the item
	NSold        int          `json:"nSold"`       // number of times item was sold
	TbPhoto      *tb.Photo    `json:"tbPhoto"`     // Telegram photo object
	LanguageCode string       `json:"languagecode"`
	MaxFiles     int          `json:"maxFiles"`
}

type Shop struct {
	*storage.Base
	Owner        *lnbits.User        `json:"owner"`       // owner of the shop
	Type         string              `json:"Type"`        // type of the shop
	Title        string              `json:"title"`       // Title of the item
	Description  string              `json:"description"` // Description of the item
	ItemIds      []string            `json:"ItemsIDs"`    //
	Items        map[string]ShopItem `json:"Items"`       //
	LanguageCode string              `json:"languagecode"`
	ShopsID      string              `json:"shopsID"`
	MaxItems     int                 `json:"maxItems"`
}

type Shops struct {
	*storage.Base
	Owner       *lnbits.User `json:"owner"` // owner of the shop
	Shops       []string     `json:"shop"`  //
	MaxShops    int          `json:"maxShops"`
	Description string       `json:"description"`
}

const (
	MAX_SHOPS                    = 10
	MAX_ITEMS_PER_SHOP           = 20
	MAX_FILES_PER_ITEM           = 200
	SHOP_TITLE_MAX_LENGTH        = 50
	ITEM_TITLE_MAX_LENGTH        = 1500
	SHOPS_DESCRIPTION_MAX_LENGTH = 1500
)

func (shop *Shop) getItem(itemId string) (item ShopItem, ok bool) {
	item, ok = shop.Items[itemId]
	return
}

var (
	shopKeyboard              = &tb.ReplyMarkup{ResizeReplyKeyboard: false}
	browseShopButton          = shopKeyboard.Data("Browse shops", "shops_browse")
	shopNewShopButton         = shopKeyboard.Data("New Shop", "shops_newshop")
	shopDeleteShopButton      = shopKeyboard.Data("Delete Shops", "shops_deleteshop")
	shopLinkShopButton        = shopKeyboard.Data("Shop links", "shops_linkshop")
	shopRenameShopButton      = shopKeyboard.Data("Rename shop", "shops_renameshop")
	shopResetShopAskButton    = shopKeyboard.Data("Delete all shops", "shops_reset_ask")
	shopResetShopButton       = shopKeyboard.Data("Delete all shops", "shops_reset")
	shopDescriptionShopButton = shopKeyboard.Data("Shop description", "shops_description")
	shopSettingsButton        = shopKeyboard.Data("Settings", "shops_settings")
	shopShopsButton           = shopKeyboard.Data("Back", "shops_shops")

	shopAddItemButton  = shopKeyboard.Data("New item", "shop_additem")
	shopNextitemButton = shopKeyboard.Data(">", "shop_nextitem")
	shopPrevitemButton = shopKeyboard.Data("<", "shop_previtem")
	shopBuyitemButton  = shopKeyboard.Data("Buy", "shop_buyitem")

	shopSelectButton           = shopKeyboard.Data("SHOP SELECTOR", "select_shop")        // shop slectino buttons
	shopDeleteSelectButton     = shopKeyboard.Data("DELETE SHOP SELECTOR", "delete_shop") // shop slectino buttons
	shopLinkSelectButton       = shopKeyboard.Data("LINK SHOP SELECTOR", "link_shop")     // shop slectino buttons
	shopRenameSelectButton     = shopKeyboard.Data("RENAME SHOP SELECTOR", "rename_shop") // shop slectino buttons
	shopItemPriceButton        = shopKeyboard.Data("Price", "shop_itemprice")
	shopItemDeleteButton       = shopKeyboard.Data("Delete", "shop_itemdelete")
	shopItemTitleButton        = shopKeyboard.Data("Set title", "shop_itemtitle")
	shopItemAddFileButton      = shopKeyboard.Data("Add file", "shop_itemaddfile")
	shopItemSettingsButton     = shopKeyboard.Data("Item settings", "shop_itemsettings")
	shopItemSettingsBackButton = shopKeyboard.Data("Back", "shop_itemsettingsback")

	shopItemBuyButton       = shopKeyboard.Data("Buy", "shop_itembuy")
	shopItemCancelBuyButton = shopKeyboard.Data("Cancel", "shop_itemcancelbuy")
)

// shopItemPriceHandler is invoked when the user presses the item settings button to set a price
func (bot *TipBot) shopItemPriceHandler(ctx context.Context, c *tb.Callback) (context.Context, error) {
	log.Debugf("[shopItemPriceHandler] %s", c.Data)
	user := LoadUser(ctx)
	shopView, err := bot.getUserShopview(ctx, user)
	if err != nil {
		return ctx, err
	}
	shop, err := bot.getShop(ctx, shopView.ShopID)
	if shop.Owner.Telegram.ID != c.Sender.ID {
		return ctx, errors.Create(errors.UnknownError)
	}
	item := shop.Items[shop.ItemIds[shopView.Page]]
	// sanity check
	if item.ID != c.Data {
		log.Error("[shopItemPriceHandler] item id mismatch")
		return ctx, errors.Create(errors.ItemIdMismatchError)
	}
	// We need to save the pay state in the user state so we can load the payment in the next handler
	SetUserState(user, bot, lnbits.UserStateShopItemSendPrice, item.ID)
	bot.sendStatusMessage(ctx, c.Sender, fmt.Sprintf("💯 Enter a price."), tb.ForceReply)
	return ctx, nil
}

// enterShopItemPriceHandler is invoked when the user enters a price amount
func (bot *TipBot) enterShopItemPriceHandler(ctx context.Context, m *tb.Message) (context.Context, error) {
	log.Debugf("[enterShopItemPriceHandler] %s", m.Text)
	user := LoadUser(ctx)
	shopView, err := bot.getUserShopview(ctx, user)
	if err != nil {
		return ctx, err
	}
	shop, err := bot.getShop(ctx, shopView.ShopID)
	if err != nil {
		return ctx, err
	}
	if shop.Owner.Telegram.ID != m.Sender.ID {
		return ctx, errors.Create(errors.NotShopOwnerError)
	}
	item := shop.Items[shop.ItemIds[shopView.Page]]
	// sanity check
	if item.ID != user.StateData {
		log.Error("[shopItemPriceHandler] item id mismatch")
		return ctx, fmt.Errorf("item id mismatch")
	}

	var amount int64
	if m.Text == "0" {
		amount = 0
	} else {
		amount, err = getAmount(m.Text)
		if err != nil {
			log.Warnf("[enterShopItemPriceHandler] %s", err.Error())
			bot.trySendMessage(m.Sender, Translate(ctx, "lnurlInvalidAmountMessage"))
			ResetUserState(user, bot)
			return ctx, err
		}
	}

	if amount > 200 {
		bot.sendStatusMessageAndDelete(ctx, m.Sender, fmt.Sprintf("ℹ️ During alpha testing, price can be max 200 sat."))
		amount = 200
	}
	item.Price = amount
	shop.Items[item.ID] = item
	runtime.IgnoreError(shop.Set(shop, bot.ShopBunt))
	bot.tryDeleteMessage(m)
	bot.sendStatusMessageAndDelete(ctx, m.Sender, fmt.Sprintf("✅ Price set."))
	ResetUserState(user, bot)
	// go func() {
	// 	time.Sleep(time.Duration(5) * time.Second)
	// 	bot.shopViewDeleteAllStatusMsgs(ctx, user)
	// }()
	bot.displayShopItem(ctx, shopView.Message, shop)
	return ctx, nil
}

// shopItemPriceHandler is invoked when the user presses the item settings button to set a item title
func (bot *TipBot) shopItemTitleHandler(ctx context.Context, c *tb.Callback) (context.Context, error) {
	log.Debugf("[shopItemTitleHandler] %s", c.Data)
	user := LoadUser(ctx)
	shopView, err := bot.getUserShopview(ctx, user)
	if err != nil {
		return ctx, err
	}
	shop, err := bot.getShop(ctx, shopView.ShopID)
	if shop.Owner.Telegram.ID != c.Sender.ID {
		return ctx, errors.Create(errors.UnknownError)
	}
	item := shop.Items[shop.ItemIds[shopView.Page]]
	// sanity check
	if item.ID != c.Data {
		log.Error("[shopItemTitleHandler] item id mismatch")
		return ctx, errors.Create(errors.ItemIdMismatchError)
	}
	// We need to save the pay state in the user state so we can load the payment in the next handler
	SetUserState(user, bot, lnbits.UserStateShopItemSendTitle, item.ID)
	bot.sendStatusMessage(ctx, c.Sender, fmt.Sprintf("⌨️ Enter item title."), tb.ForceReply)
	return ctx, nil
}

// enterShopItemTitleHandler is invoked when the user enters a title of the item
func (bot *TipBot) enterShopItemTitleHandler(ctx context.Context, m *tb.Message) (context.Context, error) {
	log.Debugf("[enterShopItemTitleHandler] %s", m.Text)
	user := LoadUser(ctx)
	shopView, err := bot.getUserShopview(ctx, user)
	if err != nil {
		return ctx, err
	}
	shop, err := bot.getShop(ctx, shopView.ShopID)
	if err != nil {
		return ctx, err
	}
	if shop.Owner.Telegram.ID != m.Sender.ID {
		return ctx, errors.Create(errors.NotShopOwnerError)
	}
	item := shop.Items[shop.ItemIds[shopView.Page]]
	// sanity check
	if item.ID != user.StateData {
		log.Error("[enterShopItemTitleHandler] item id mismatch")
		return ctx, errors.Create(errors.ItemIdMismatchError)
	}
	if len(m.Text) == 0 {
		ResetUserState(user, bot)
		bot.sendStatusMessageAndDelete(ctx, m.Sender, "🚫 Action cancelled.")
		go func() {
			time.Sleep(time.Duration(5) * time.Second)
			bot.shopViewDeleteAllStatusMsgs(ctx, user)
		}()
		return ctx, errors.Create(errors.InvalidSyntaxError)
	}
	// crop item title
	if len(m.Text) > ITEM_TITLE_MAX_LENGTH {
		m.Text = m.Text[:ITEM_TITLE_MAX_LENGTH]
	}
	item.Title = m.Text
	item.TbPhoto.Caption = m.Text
	shop.Items[item.ID] = item
	runtime.IgnoreError(shop.Set(shop, bot.ShopBunt))
	bot.tryDeleteMessage(m)
	bot.sendStatusMessageAndDelete(ctx, m.Sender, fmt.Sprintf("✅ Title set."))
	ResetUserState(user, bot)
	// go func() {
	// 	time.Sleep(time.Duration(5) * time.Second)
	// 	bot.shopViewDeleteAllStatusMsgs(ctx, user)
	// }()
	bot.displayShopItem(ctx, shopView.Message, shop)
	return ctx, nil
}

// shopItemSettingsHandler is invoked when the user presses the item settings button
func (bot *TipBot) shopItemSettingsHandler(ctx context.Context, c *tb.Callback) (context.Context, error) {
	log.Debugf("[shopItemSettingsHandler] %s", c.Data)
	user := LoadUser(ctx)
	shopView, err := bot.getUserShopview(ctx, user)
	if err != nil {
		return ctx, err
	}
	shop, err := bot.getShop(ctx, shopView.ShopID)
	item := shop.Items[shop.ItemIds[shopView.Page]]
	// sanity check
	if item.ID != c.Data {
		log.Error("[shopItemSettingsHandler] item id mismatch")
		return ctx, errors.Create(errors.ItemIdMismatchError)
	}
	if item.TbPhoto != nil {
		item.TbPhoto.Caption = bot.getItemTitle(ctx, &item)
	}
	_, err = bot.tryEditMessage(shopView.Message, item.TbPhoto, bot.shopItemSettingsMenu(ctx, shop, &item))
	return ctx, err
}

// shopItemPriceHandler is invoked when the user presses the item settings button to set a item title
func (bot *TipBot) shopItemDeleteHandler(ctx context.Context, c *tb.Callback) (context.Context, error) {
	log.Debugf("[shopItemDeleteHandler] %s", c.Data)
	user := LoadUser(ctx)
	shopView, err := bot.getUserShopview(ctx, user)
	if err != nil {
		return ctx, err
	}
	shop, err := bot.getShop(ctx, shopView.ShopID)
	if err != nil {
		return ctx, err
	}
	if shop.Owner.Telegram.ID != c.Sender.ID {
		return ctx, errors.Create(errors.UnknownError)
	}
	item := shop.Items[shop.ItemIds[shopView.Page]]
	if shop.Owner.Telegram.ID != c.Sender.ID {
		return ctx, errors.Create(errors.UnknownError)
	}

	// delete ItemID of item
	for i, itemId := range shop.ItemIds {
		if itemId == item.ID {
			if len(shop.ItemIds) == 1 {
				shop.ItemIds = []string{}
			} else {
				shop.ItemIds = append(shop.ItemIds[:i], shop.ItemIds[i+1:]...)
			}
			break
		}
	}
	// delete item itself
	delete(shop.Items, item.ID)
	runtime.IgnoreError(shop.Set(shop, bot.ShopBunt))

	ResetUserState(user, bot)
	bot.sendStatusMessageAndDelete(ctx, c.Message.Chat, fmt.Sprintf("✅ Item deleted."))
	// go func() {
	// 	time.Sleep(time.Duration(5) * time.Second)
	// 	bot.shopViewDeleteAllStatusMsgs(ctx, user)
	// }()
	if shopView.Page > 0 {
		shopView.Page--
	}
	bot.Cache.Set(shopView.ID, shopView, &store.Options{Expiration: 24 * time.Hour})
	bot.displayShopItem(ctx, shopView.Message, shop)
	return ctx, nil
}

// displayShopItemHandler is invoked when the user presses the back button in the item settings
func (bot *TipBot) displayShopItemHandler(ctx context.Context, c *tb.Callback) (context.Context, error) {
	log.Debugf("[displayShopItemHandler] c.Data: %s", c.Data)
	user := LoadUser(ctx)
	shopView, err := bot.getUserShopview(ctx, user)
	if err != nil {
		return ctx, err
	}
	shop, err := bot.getShop(ctx, shopView.ShopID)
	if err != nil {
		return ctx, err
	}
	// item := shop.Items[shop.ItemIds[shopView.Page]]
	// // sanity check
	// if item.ID != c.Data {
	// 	log.Error("[shopItemSettingsHandler] item id mismatch")
	// 	return
	// }
	bot.displayShopItem(ctx, c.Message, shop)
	return ctx, nil
}

// shopNextItemHandler is invoked when the user presses the next item button
func (bot *TipBot) shopNextItemButtonHandler(ctx context.Context, c *tb.Callback) (context.Context, error) {
	log.Debugf("[shopNextItemButtonHandler] c.Data: %s", c.Data)
	user := LoadUser(ctx)
	// shopView, err := bot.Cache.Get(fmt.Sprintf("shopview-%d", user.Telegram.ID))
	shopView, err := bot.getUserShopview(ctx, user)
	if err != nil {
		return ctx, err
	}
	shop, err := bot.getShop(ctx, shopView.ShopID)
	if shopView.Page < len(shop.Items)-1 {
		shopView.Page++
		bot.Cache.Set(shopView.ID, shopView, &store.Options{Expiration: 24 * time.Hour})
		shop, err = bot.getShop(ctx, shopView.ShopID)
		if err != nil {
			return ctx, err
		}
		bot.displayShopItem(ctx, c.Message, shop)
	}
	return ctx, nil
}

// shopPrevItemButtonHandler is invoked when the user presses the previous item button
func (bot *TipBot) shopPrevItemButtonHandler(ctx context.Context, c *tb.Callback) (context.Context, error) {
	log.Debugf("[shopPrevItemButtonHandler] %s", c.Data)
	user := LoadUser(ctx)
	shopView, err := bot.getUserShopview(ctx, user)
	if err != nil {
		return ctx, err
	}
	if shopView.Page == 0 {
		c.Message.Text = "/shops " + shopView.ShopOwner.Telegram.Username
		return bot.shopsHandler(ctx, c.Message)

	}
	if shopView.Page > 0 {
		shopView.Page--
	}
	bot.Cache.Set(shopView.ID, shopView, &store.Options{Expiration: 24 * time.Hour})
	shop, err := bot.getShop(ctx, shopView.ShopID)
	bot.displayShopItem(ctx, c.Message, shop)
	return ctx, nil
}

func (bot *TipBot) getItemTitle(ctx context.Context, item *ShopItem) string {
	caption := ""
	if len(item.Title) > 0 {
		caption = fmt.Sprintf("%s", item.Title)
	}
	if len(item.FileIDs) > 0 {
		if len(caption) > 0 {
			caption += " "
		}
		caption += fmt.Sprintf("(%d Files)", len(item.FileIDs))
	}
	if item.Price > 0 {
		caption += fmt.Sprintf("\n\n💸 Price: %d sat", item.Price)
	}
	// item.TbPhoto.Caption = caption
	return caption
}

// displayShopItem renders the current item in the shopView
// requires that the shopview page is already set accordingly
// m is the message that will be edited
func (bot *TipBot) displayShopItem(ctx context.Context, m *tb.Message, shop *Shop) *tb.Message {
	log.Debugf("[displayShopItem] shop: %+v", shop)
	user := LoadUser(ctx)
	shopView, err := bot.getUserShopview(ctx, user)
	if err != nil {
		log.Errorf("[displayShopItem] %s", err.Error())
		return nil
	}
	// failsafe: if the page is out of bounds, reset it
	if len(shop.Items) > 0 && shopView.Page >= len(shop.Items) {
		shopView.Page = len(shop.Items) - 1
	} else if len(shop.Items) == 0 {
		shopView.Page = 0
	}

	log.Debugf("[displayShopItem] shop: %s page: %d items: %d", shop.ID, shopView.Page, len(shop.Items))
	if len(shop.Items) == 0 {
		no_items_message := "There are no items in this shop yet."
		if shopView.Message != nil && len(shopView.Message.Text) > 0 {
			shopView.Message, _ = bot.tryEditMessage(shopView.Message, no_items_message, bot.shopMenu(ctx, shop, &ShopItem{}))
		} else {
			if shopView.Message != nil {
				bot.tryDeleteMessage(shopView.Message)
			}
			shopView.Message = bot.trySendMessage(shopView.Chat, no_items_message, bot.shopMenu(ctx, shop, &ShopItem{}))
		}
		shopView.Page = 0
		return shopView.Message
	}

	item := shop.Items[shop.ItemIds[shopView.Page]]
	if item.TbPhoto != nil {
		item.TbPhoto.Caption = bot.getItemTitle(ctx, &item)
	}

	// var msg *tb.Message
	if shopView.Message != nil {
		if item.TbPhoto != nil {
			if shopView.Message.Photo != nil {
				// can only edit photo messages with another photo
				shopView.Message, _ = bot.tryEditMessage(shopView.Message, item.TbPhoto, bot.shopMenu(ctx, shop, &item))
			} else {
				// if editing failes
				bot.tryDeleteMessage(shopView.Message)
				shopView.Message = bot.trySendMessage(shopView.Message.Chat, item.TbPhoto, bot.shopMenu(ctx, shop, &item))
			}
		} else if item.Title != "" {
			shopView.Message, _ = bot.tryEditMessage(shopView.Message, item.Title, bot.shopMenu(ctx, shop, &item))
			if shopView.Message == nil {
				shopView.Message = bot.trySendMessage(shopView.Message.Chat, item.Title, bot.shopMenu(ctx, shop, &item))
			}
		}
	} else {
		if m != nil && m.Chat != nil {
			shopView.Message = bot.trySendMessage(m.Chat, item.TbPhoto, bot.shopMenu(ctx, shop, &item))
		} else {
			shopView.Message = bot.trySendMessage(user.Telegram, item.TbPhoto, bot.shopMenu(ctx, shop, &item))
		}
		// shopView.Page = 0
	}
	// shopView.Message = msg
	bot.Cache.Set(shopView.ID, shopView, &store.Options{Expiration: 24 * time.Hour})
	return shopView.Message
}

// shopHandler is invoked when the user enters /shop
func (bot *TipBot) shopHandler(ctx context.Context, m *tb.Message) (context.Context, error) {
	log.Debugf("[shopHandler] %s", m.Text)
	if !m.Private() {
		return ctx, errors.Create(errors.NoPrivateChatError)
	}
	user := LoadUser(ctx)
	shopOwner := user

	// when no argument is given, i.e. command is only /shop, load /shops
	shop := &Shop{}
	if len(strings.Split(m.Text, " ")) < 2 || !strings.HasPrefix(strings.Split(m.Text, " ")[1], "shop-") {
		return bot.shopsHandler(ctx, m)
	} else {
		// else: get shop by shop ID
		shopID := strings.Split(m.Text, " ")[1]
		var err error
		shop, err = bot.getShop(ctx, shopID)
		if err != nil {
			log.Errorf("[shopHandler] %s", err.Error())
			return ctx, err
		}
	}
	shopOwner = shop.Owner
	shopView := ShopView{
		ID:        fmt.Sprintf("shopview-%d", user.Telegram.ID),
		ShopID:    shop.ID,
		Page:      0,
		ShopOwner: shopOwner,
		Chat:      m.Chat,
	}
	bot.Cache.Set(shopView.ID, shopView, &store.Options{Expiration: 24 * time.Hour})
	shopView.Message = bot.displayShopItem(ctx, m, shop)
	// shopMessage := &tb.Message{Chat: m.Chat}
	// if len(shop.ItemIds) > 0 {
	// 	// item := shop.Items[shop.ItemIds[shopView.Page]]
	// 	// shopMessage = bot.trySendMessage(m.Chat, item.TbPhoto, bot.shopMenu(ctx, shop, &item))
	// 	shopMessage = bot.displayShopItem(ctx, m, shop)
	// } else {
	// 	shopMessage = bot.trySendMessage(m.Chat, "No items in shop.", bot.shopMenu(ctx, shop, &ShopItem{}))
	// }
	// shopView.Message = shopMessage
	bot.Cache.Set(shopView.ID, shopView, &store.Options{Expiration: 24 * time.Hour})
	return ctx, nil
}

// shopNewItemHandler is invoked when the user presses the new item button
func (bot *TipBot) shopNewItemHandler(ctx context.Context, c *tb.Callback) (context.Context, error) {
	log.Debugf("[shopNewItemHandler] %s", c.Data)
	user := LoadUser(ctx)
	shop, err := bot.getShop(ctx, c.Data)
	if err != nil {
		log.Errorf("[shopNewItemHandler] %s", err.Error())
		return ctx, err
	}
	if shop.Owner.Telegram.ID != c.Sender.ID {
		return ctx, errors.Create(errors.UnknownError)
	}
	if len(shop.Items) >= shop.MaxItems {
		bot.trySendMessage(c.Sender, fmt.Sprintf("🚫 You can only have %d items in this shop. Delete an item to add a new one.", shop.MaxItems))
		return ctx, errors.Create(errors.MaxReachedError)
	}

	// We need to save the pay state in the user state so we can load the payment in the next handler
	paramsJson, err := json.Marshal(shop)
	if err != nil {
		log.Errorf("[lnurlWithdrawHandler] Error: %s", err.Error())
		// bot.trySendMessage(m.Sender, err.Error())
		return ctx, err
	}
	SetUserState(user, bot, lnbits.UserStateShopItemSendPhoto, string(paramsJson))
	bot.sendStatusMessage(ctx, c.Sender, fmt.Sprintf("🌄 Send me a cover image."))
	return ctx, nil
}

// addShopItem is a helper function for creating a shop item in the database
func (bot *TipBot) addShopItem(ctx context.Context, shopId string) (*Shop, ShopItem, error) {
	log.Debugf("[addShopItem] shopId: %s", shopId)
	shop, err := bot.getShop(ctx, shopId)
	if err != nil {
		log.Errorf("[addShopItem] %s", err.Error())
		return shop, ShopItem{}, err
	}
	user := LoadUser(ctx)
	// onnly the correct user can press
	if shop.Owner.Telegram.ID != user.Telegram.ID {
		return shop, ShopItem{}, fmt.Errorf("not owner")
	}
	// err = shop.Lock(shop, bot.ShopBunt)
	// defer shop.Release(shop, bot.ShopBunt)

	itemId := fmt.Sprintf("item-%s-%s", shop.ID, RandStringRunes(8))
	item := ShopItem{
		ID:           itemId,
		ShopID:       shop.ID,
		Owner:        user,
		Type:         "photo",
		LanguageCode: shop.LanguageCode,
		MaxFiles:     MAX_FILES_PER_ITEM,
	}
	shop.Items[itemId] = item
	shop.ItemIds = append(shop.ItemIds, itemId)
	runtime.IgnoreError(shop.Set(shop, bot.ShopBunt))
	return shop, shop.Items[itemId], nil
}

// addShopItemPhoto is invoked when the users sends a photo as a new item
func (bot *TipBot) addShopItemPhoto(ctx context.Context, m *tb.Message) (context.Context, error) {
	log.Debugf("[addShopItemPhoto] <Photo>")
	user := LoadUser(ctx)
	if user.Wallet == nil {
		return ctx, errors.Create(errors.UserNoWalletError)
	}

	// read item from user.StateData
	var state_shop Shop
	err := json.Unmarshal([]byte(user.StateData), &state_shop)
	if err != nil {
		log.Errorf("[lnurlWithdrawHandlerWithdraw] Error: %s", err.Error())
		bot.trySendMessage(m.Sender, Translate(ctx, "errorTryLaterMessage"), Translate(ctx, "errorTryLaterMessage"))
		return ctx, err
	}
	if state_shop.Owner.Telegram.ID != m.Sender.ID {
		return ctx, errors.Create(errors.NotShopOwnerError)
	}
	if m.Photo == nil {
		bot.sendStatusMessageAndDelete(ctx, m.Sender, fmt.Sprintf("🚫 That didn't work. You need to send an image (not a file)."))
		ResetUserState(user, bot)
		return ctx, errors.Create(errors.NoPhotoError)
	}

	shop, item, err := bot.addShopItem(ctx, state_shop.ID)
	// err = shop.Lock(shop, bot.ShopBunt)
	// defer shop.Release(shop, bot.ShopBunt)
	item.TbPhoto = m.Photo
	item.Title = m.Caption
	shop.Items[item.ID] = item
	runtime.IgnoreError(shop.Set(shop, bot.ShopBunt))

	bot.tryDeleteMessage(m)
	bot.sendStatusMessageAndDelete(ctx, m.Sender, fmt.Sprintf("✅ Image added. You can now add files to this item. Don't forget to set a title and a price."))
	ResetUserState(user, bot)
	// go func() {
	// 	time.Sleep(time.Duration(5) * time.Second)
	// 	bot.shopViewDeleteAllStatusMsgs(ctx, user)
	// }()

	shopView, err := bot.getUserShopview(ctx, user)
	shopView.Page = len(shop.Items) - 1
	bot.Cache.Set(shopView.ID, shopView, &store.Options{Expiration: 24 * time.Hour})
	bot.displayShopItem(ctx, shopView.Message, shop)

	log.Infof("[🛍 shop] %s added an item %s:%s.", GetUserStr(user.Telegram), shop.ID, item.ID)
	return ctx, nil
}

// ------------------- item files ----------
// shopItemAddItemHandler is invoked when the user presses the new item button
func (bot *TipBot) shopItemAddItemHandler(ctx context.Context, c *tb.Callback) (context.Context, error) {
	log.Debugf("[shopItemAddItemHandler] %s", c.Data)
	user := LoadUser(ctx)
	if user.Wallet == nil {
		return ctx, errors.Create(errors.UserNoWalletError)
	}
	shopView, err := bot.getUserShopview(ctx, user)
	if err != nil {
		log.Errorf("[shopItemAddItemHandler] %s", err.Error())
		return ctx, err
	}

	shop, err := bot.getShop(ctx, shopView.ShopID)
	if err != nil {
		log.Errorf("[shopItemAddItemHandler] %s", err.Error())
		return ctx, err
	}

	itemID := c.Data

	item := shop.Items[itemID]

	if len(item.FileIDs) >= item.MaxFiles {
		bot.trySendMessage(c.Sender, fmt.Sprintf("🚫 You can only have %d files in this item.", item.MaxFiles))
		return ctx, errors.Create(errors.NoFileFoundError)
	}
	SetUserState(user, bot, lnbits.UserStateShopItemSendItemFile, c.Data)
	bot.sendStatusMessage(ctx, c.Sender, fmt.Sprintf("💾 Send me one or more files."))
	return ctx, err
}

// addItemFileHandler is invoked when the users sends a new file for the item
func (bot *TipBot) addItemFileHandler(ctx context.Context, m *tb.Message) (context.Context, error) {
	log.Debugf("[addItemFileHandler] <File>")
	user := LoadUser(ctx)
	if user.Wallet == nil {
		return ctx, errors.Create(errors.UserNoWalletError)
	}
	shopView, err := bot.getUserShopview(ctx, user)
	if err != nil {
		log.Errorf("[addItemFileHandler] %s", err.Error())
		return ctx, err
	}

	shop, err := bot.getShop(ctx, shopView.ShopID)
	if err != nil {
		log.Errorf("[shopNewItemHandler] %s", err.Error())
		return ctx, err
	}

	itemID := user.StateData

	item := shop.Items[itemID]
	if m.Photo != nil {
		item.FileIDs = append(item.FileIDs, m.Photo.FileID)
		item.FileTypes = append(item.FileTypes, "photo")
		bot.sendStatusMessageAndDelete(ctx, m.Sender, fmt.Sprintf("ℹ️ To send more than one photo at a time, send them as files."))
	} else if m.Document != nil {
		item.FileIDs = append(item.FileIDs, m.Document.FileID)
		item.FileTypes = append(item.FileTypes, "document")
	} else if m.Audio != nil {
		item.FileIDs = append(item.FileIDs, m.Audio.FileID)
		item.FileTypes = append(item.FileTypes, "audio")
	} else if m.Video != nil {
		item.FileIDs = append(item.FileIDs, m.Video.FileID)
		item.FileTypes = append(item.FileTypes, "video")
	} else if m.Voice != nil {
		item.FileIDs = append(item.FileIDs, m.Voice.FileID)
		item.FileTypes = append(item.FileTypes, "voice")
	} else if m.VideoNote != nil {
		item.FileIDs = append(item.FileIDs, m.VideoNote.FileID)
		item.FileTypes = append(item.FileTypes, "videonote")
	} else if m.Sticker != nil {
		item.FileIDs = append(item.FileIDs, m.Sticker.FileID)
		item.FileTypes = append(item.FileTypes, "sticker")
	} else {
		log.Errorf("[addItemFileHandler] no file found")
		return ctx, errors.Create(errors.NoFileFoundError)
	}
	shop.Items[item.ID] = item

	runtime.IgnoreError(shop.Set(shop, bot.ShopBunt))
	bot.tryDeleteMessage(m)
	bot.sendStatusMessageAndDelete(ctx, m.Sender, fmt.Sprintf("✅ File added."))

	// ticker := runtime.GetTicker(shop.ID, runtime.WithDuration(5*time.Second))
	// if !ticker.Started {
	// 	ticker.Do(func() {
	// 		bot.shopViewDeleteAllStatusMsgs(ctx, user)
	// 		// removing ticker asap done
	// 		runtime.RemoveTicker(shop.ID)
	// 	})
	// } else {
	// 	ticker.ResetChan <- struct{}{}
	// }

	// // start a ticker to check if the user has sent more files
	// if t, ok := fileStateResetTicker.Get(shop.ID); ok {
	// 	// state reset ticker found. resetting ticker.
	// 	t.(*runtime.ResettableFunctionTicker).ResetChan <- struct{}{}
	// } else {
	// 	// state reset ticker not found. creating new one.
	// 	ticker := runtime.NewResettableFunctionTicker(runtime.WithDuration(time.Second * 5))
	// 	// storing reset ticker in mem
	// 	fileStateResetTicker.Set(shop.ID, ticker)
	// 	go func() {
	// 		// starting ticker
	// 		ticker.Do(func() {
	// 			// time.Sleep(time.Duration(5) * time.Second)
	// 			bot.shopViewDeleteAllStatusMsgs(ctx, user)
	// 			// removing ticker asap done
	// 			fileStateResetTicker.Remove(shop.ID)
	// 		})
	// 	}()
	// }

	// go func() {
	// 	time.Sleep(time.Duration(5) * time.Second)
	// 	bot.shopViewDeleteAllStatusMsgs(ctx, user)
	// }()
	bot.displayShopItem(ctx, shopView.Message, shop)
	log.Infof("[🛍 shop] %s added a file to shop:item %s:%s.", GetUserStr(user.Telegram), shop.ID, item.ID)
	return ctx, nil
}

func (bot *TipBot) shopGetItemFilesHandler(ctx context.Context, c *tb.Callback) (context.Context, error) {
	log.Debugf("[shopGetItemFilesHandler] %s", c.Data)
	user := LoadUser(ctx)
	if user.Wallet == nil {
		return ctx, errors.Create(errors.UserNoWalletError)
	}
	shopView, err := bot.getUserShopview(ctx, user)
	if err != nil {
		log.Errorf("[addItemFileHandler] %s", err.Error())
		return ctx, err
	}
	shop, err := bot.getShop(ctx, shopView.ShopID)
	if err != nil {
		log.Errorf("[shopGetItemFilesHandler] %s", err.Error())
		return ctx, err
	}
	itemID := c.Data
	item := shop.Items[itemID]

	if item.Price <= 0 {
		bot.shopSendItemFilesToUser(ctx, user, itemID)
	} else {
		if item.TbPhoto != nil {
			item.TbPhoto.Caption = bot.getItemTitle(ctx, &item)
		}
		bot.tryEditMessage(shopView.Message, item.TbPhoto, bot.shopItemConfirmBuyMenu(ctx, shop, &item))
	}

	// // send the cover image
	// bot.sendFileByID(ctx, c.Sender, item.TbPhoto.FileID, "photo")
	// // and all other files
	// for i, fileID := range item.FileIDs {
	// 	bot.sendFileByID(ctx, c.Sender, fileID, item.FileTypes[i])
	// }
	// log.Infof("[🛍 shop] %s got %d items from %s's item %s (for %d sat).", GetUserStr(user.Telegram), len(item.FileIDs), GetUserStr(shop.Owner.Telegram), item.ID, item.Price)
	return ctx, nil
}

// shopConfirmBuyHandler is invoked when the user has confirmed to pay for an item
func (bot *TipBot) shopConfirmBuyHandler(ctx context.Context, c *tb.Callback) (context.Context, error) {
	log.Debugf("[shopConfirmBuyHandler] %s", c.Data)
	user := LoadUser(ctx)
	if user.Wallet == nil {
		return ctx, errors.Create(errors.UserNoWalletError)
	}
	shopView, err := bot.getUserShopview(ctx, user)
	if err != nil {
		log.Errorf("[shopConfirmBuyHandler] %s", err.Error())
		return ctx, err
	}
	shop, err := bot.getShop(ctx, shopView.ShopID)
	if err != nil {
		log.Errorf("[shopConfirmBuyHandler] %s", err.Error())
		return ctx, err
	}
	itemID := c.Data
	item := shop.Items[itemID]
	if item.Owner.ID != shop.Owner.ID {
		log.Errorf("[shopConfirmBuyHandler] Owners do not match.")
		return ctx, errors.Create(errors.NotShopOwnerError)
	}
	from := user
	to := shop.Owner

	// fromUserStr := GetUserStr(from.Telegram)
	// fromUserStrMd := GetUserStrMd(from.Telegram)
	toUserStr := GetUserStr(to.Telegram)
	toUserStrMd := GetUserStrMd(to.Telegram)
	amount := item.Price
	if amount <= 0 {
		log.Errorf("[shopConfirmBuyHandler] item has no price.")
		return ctx, errors.Create(errors.InvalidAmountError)
	}
	transactionMemo := fmt.Sprintf("Buy item %s (%d sat).", toUserStr, amount)
	t := NewTransaction(bot, from, to, amount, TransactionType("shop"))
	t.Memo = transactionMemo

	success, err := t.Send()
	if !success || err != nil {
		// bot.trySendMessage(c.Sender, sendErrorMessage)
		errmsg := fmt.Sprintf("[shop] Error: Transaction failed. %s", err.Error())
		log.Errorln(errmsg)
		ctx = context.WithValue(ctx, "callback_response", i18n.Translate(user.Telegram.LanguageCode, "sendErrorMessage"))
		// bot.trySendMessage(user.Telegram, i18n.Translate(user.Telegram.LanguageCode, "sendErrorMessage"), &tb.ReplyMarkup{})
		return ctx, errors.New(errors.UnknownError, err)
	}
	// bot.trySendMessage(user.Telegram, fmt.Sprintf("🛍 %d sat sent to %s.", amount, toUserStrMd), &tb.ReplyMarkup{})
	shopItemTitle := "an item"
	if len(item.Title) > 0 {
		shopItemTitle = fmt.Sprintf("%s", item.Title)
	}
	ctx = context.WithValue(ctx, "callback_response", "🛍 Purchase successful.")
	bot.trySendMessage(to.Telegram, fmt.Sprintf("🛍 Someone bought `%s` from your shop `%s` for `%d sat`.", str.MarkdownEscape(shopItemTitle), str.MarkdownEscape(shop.Title), amount))
	bot.trySendMessage(from.Telegram, fmt.Sprintf("🛍 You bought `%s` from %s's shop `%s` for `%d sat`.", str.MarkdownEscape(shopItemTitle), toUserStrMd, str.MarkdownEscape(shop.Title), amount))
	log.Infof("[🛍 shop] %s bought from %s shop: %s item: %s  for %d sat.", toUserStr, GetUserStr(to.Telegram), shop.Title, shopItemTitle, amount)
	bot.shopSendItemFilesToUser(ctx, user, itemID)
	return ctx, nil
}

// shopSendItemFilesToUser is a handler function to send itemID's files to the user
func (bot *TipBot) shopSendItemFilesToUser(ctx context.Context, toUser *lnbits.User, itemID string) {
	log.Debugf("[shopSendItemFilesToUser] %s -> %s", GetUserStr(toUser.Telegram), itemID)
	user := LoadUser(ctx)
	if user.Wallet == nil {
		return // errors.New("user has no wallet"), 0
	}
	shopView, err := bot.getUserShopview(ctx, user)
	if err != nil {
		log.Errorf("[shopGetItemFilesHandler] %s", err.Error())
		return
	}
	shop, err := bot.getShop(ctx, shopView.ShopID)
	if err != nil {
		log.Errorf("[addItemFileHandler] %s", err.Error())
		return
	}
	item := shop.Items[itemID]
	// send the cover image
	bot.sendFileByID(ctx, toUser.Telegram, item.TbPhoto.FileID, "photo")
	// and all other files
	for i, fileID := range item.FileIDs {
		bot.sendFileByID(ctx, toUser.Telegram, fileID, item.FileTypes[i])
	}
	log.Infof("[🛍 shop] %s got %d items from %s's item %s (for %d sat).", GetUserStr(user.Telegram), len(item.FileIDs), GetUserStr(shop.Owner.Telegram), item.ID, item.Price)

	// delete old shop and show again below the files
	if shopView.Message != nil {
		bot.tryDeleteMessage(shopView.Message)
	}
	shopView.Message = nil
	bot.Cache.Set(shopView.ID, shopView, &store.Options{Expiration: 24 * time.Hour})
	bot.displayShopItem(ctx, &tb.Message{}, shop)
}

func (bot *TipBot) sendFileByID(ctx context.Context, to tb.Recipient, fileId string, fileType string) {
	switch fileType {
	case "photo":
		sendable := &tb.Photo{File: tb.File{FileID: fileId}}
		bot.trySendMessage(to, sendable)
	case "document":
		sendable := &tb.Document{File: tb.File{FileID: fileId}}
		bot.trySendMessage(to, sendable)
	case "audio":
		sendable := &tb.Audio{File: tb.File{FileID: fileId}}
		bot.trySendMessage(to, sendable)
	case "video":
		sendable := &tb.Video{File: tb.File{FileID: fileId}}
		bot.trySendMessage(to, sendable)
	case "voice":
		sendable := &tb.Voice{File: tb.File{FileID: fileId}}
		bot.trySendMessage(to, sendable)
	case "videonote":
		sendable := &tb.VideoNote{File: tb.File{FileID: fileId}}
		bot.trySendMessage(to, sendable)
	case "sticker":
		sendable := &tb.Sticker{File: tb.File{FileID: fileId}}
		bot.trySendMessage(to, sendable)
	}
	return
}

// -------------- shops handler --------------
// var ShopsText = "*Welcome to %s shop.*\n%s\nThere are %d shops here.\n%s"
var ShopsText = ""
var ShopsTextWelcome = "*You are browsing %s shop.*"
var ShopsTextShopCount = "*Browse %d shops:*"
var ShopsTextHelp = "⚠️ Shops are still in beta. Expect bugs."
var ShopsNoShopsText = "*There are no shops here yet.*"

// shopsHandlerCallback is a warpper for shopsHandler for callbacks
func (bot *TipBot) shopsHandlerCallback(ctx context.Context, c *tb.Callback) (context.Context, error) {
	return bot.shopsHandler(ctx, c.Message)
}

// shopsHandler is invoked when the user enters /shops
func (bot *TipBot) shopsHandler(ctx context.Context, m *tb.Message) (context.Context, error) {
	log.Debugf("[shopsHandler] %s", GetUserStr(m.Sender))
	if !m.Private() {
		return ctx, errors.Create(errors.NoPrivateChatError)
	}
	user := LoadUser(ctx)
	shopOwner := user

	// if the user in the command, i.e. /shops @user
	if len(strings.Split(m.Text, " ")) > 1 && strings.HasPrefix(strings.Split(m.Text, " ")[0], "/shop") {
		toUserStrMention := ""
		toUserStrWithoutAt := ""

		// check for user in command, accepts user mention or plain username without @
		if len(m.Entities) > 1 && m.Entities[1].Type == "mention" {
			toUserStrMention = m.Text[m.Entities[1].Offset : m.Entities[1].Offset+m.Entities[1].Length]
			toUserStrWithoutAt = strings.TrimPrefix(toUserStrMention, "@")
		} else {
			var err error
			toUserStrWithoutAt, err = getArgumentFromCommand(m.Text, 1)
			if err != nil {
				log.Errorln(err.Error())
				return ctx, err
			}
			toUserStrWithoutAt = strings.TrimPrefix(toUserStrWithoutAt, "@")
			toUserStrMention = "@" + toUserStrWithoutAt
		}

		toUserDb, err := GetUserByTelegramUsername(toUserStrWithoutAt, *bot)
		if err != nil {
			NewMessage(m, WithDuration(0, bot))
			// cut username if it's too long
			if len(toUserStrMention) > 100 {
				toUserStrMention = toUserStrMention[:100]
			}
			bot.trySendMessage(m.Sender, fmt.Sprintf(Translate(ctx, "sendUserHasNoWalletMessage"), str.MarkdownEscape(toUserStrMention)))
			return ctx, err
		}
		// overwrite user with the one from db
		shopOwner = toUserDb
	} else if !strings.HasPrefix(strings.Split(m.Text, " ")[0], "/shop") {
		// otherwise, the user is returning to a shops view from a back button callback
		shopView, err := bot.getUserShopview(ctx, user)
		if err == nil {
			shopOwner = shopView.ShopOwner
		}
	}

	if shopOwner == nil {
		log.Error("[shopsHandler] shopOwner is nil")
		return ctx, errors.Create(errors.ShopNoOwnerError)
	}
	shops, err := bot.getUserShops(ctx, shopOwner)
	if err != nil && user.Telegram.ID == shopOwner.Telegram.ID {
		shops, err = bot.initUserShops(ctx, user)
		if err != nil {
			log.Errorf("[shopsHandler] %s", err.Error())
			return ctx, err
		}
	}

	if len(shops.Shops) == 0 && user.Telegram.ID != shopOwner.Telegram.ID {
		bot.trySendMessage(m.Chat, fmt.Sprintf("This user has no shops yet."))
		return ctx, errors.Create(errors.NoShopError)
	}

	// build shop list
	shopTitles := ""
	for _, shopId := range shops.Shops {
		shop, err := bot.getShop(ctx, shopId)
		if err != nil {
			log.Errorf("[shopsHandler] %s", err.Error())
			return ctx, err
		}
		shopTitles += fmt.Sprintf("\n· %s (%d items)", str.MarkdownEscape(shop.Title), len(shop.Items))

	}

	// build shop text

	// shows "your shop" or "@other's shop"
	shopOwnerText := "your"
	if shopOwner.Telegram.ID != user.Telegram.ID {
		shopOwnerText = fmt.Sprintf("%s's", GetUserStr(shopOwner.Telegram))
	}
	ShopsText = fmt.Sprintf(ShopsTextWelcome, shopOwnerText)
	if len(shops.Description) > 0 {
		ShopsText += fmt.Sprintf("\n\n%s\n", shops.Description)
	} else {
		ShopsText += "\n"
	}
	if len(shops.Shops) > 0 {
		ShopsText += fmt.Sprintf("\n%s\n", fmt.Sprintf(ShopsTextShopCount, len(shops.Shops)))
	} else {
		ShopsText += fmt.Sprintf("\n%s\n", ShopsNoShopsText)
	}

	if len(shops.Shops) > 0 {
		ShopsText += fmt.Sprintf("%s\n", shopTitles)
	}
	ShopsText += fmt.Sprintf("\n%s", ShopsTextHelp)

	// fmt.Sprintf(ShopsText, shopOwnerText, len(shops.Shops), shopTitles)

	// if the user used the command /shops, we will send a new message
	// if the user clicked a button and has a shopview set, we will edit an old message
	shopView, err := bot.getUserShopview(ctx, user)
	var shopsMsg *tb.Message
	if err == nil && !strings.HasPrefix(strings.Split(m.Text, " ")[0], "/shop") {
		// the user is returning to a shops view from a back button callback
		if shopView.Message != nil && shopView.Message.Photo == nil {
			shopsMsg, _ = bot.tryEditMessage(shopView.Message, ShopsText, bot.shopsMainMenu(ctx, shops))
		}
		if shopsMsg == nil {
			// if editing has failed, we will send a new message
			if shopView.Message != nil {
				bot.tryDeleteMessage(shopView.Message)
			}
			shopsMsg = bot.trySendMessage(m.Chat, ShopsText, bot.shopsMainMenu(ctx, shops))

		}
	} else {
		// the user has entered /shops or
		// the user has no shopview set, so we will send a new message
		if shopView.Message != nil {
			// delete any old shop message
			bot.tryDeleteMessage(shopView.Message)
		}
		shopsMsg = bot.trySendMessage(m.Chat, ShopsText, bot.shopsMainMenu(ctx, shops))
	}
	shopViewNew := ShopView{
		ID:             fmt.Sprintf("shopview-%d", user.Telegram.ID),
		Message:        shopsMsg,
		ShopOwner:      shopOwner,
		StatusMessages: shopView.StatusMessages, // keep the old status messages
	}
	bot.Cache.Set(shopViewNew.ID, shopViewNew, &store.Options{Expiration: 24 * time.Hour})
	return ctx, nil
}

// shopsDeleteShopBrowser is invoked when the user clicks on "delete shops" and makes a list of all shops
func (bot *TipBot) shopsDeleteShopBrowser(ctx context.Context, c *tb.Callback) (context.Context, error) {
	log.Debugf("[shopsDeleteShopBrowser] %s", c.Data)
	user := LoadUser(ctx)
	shops, err := bot.getUserShops(ctx, user)
	if err != nil {
		return ctx, err
	}
	var s []*Shop
	for _, shopId := range shops.Shops {
		shop, _ := bot.getShop(ctx, shopId)
		if shop.Owner.Telegram.ID != c.Sender.ID {
			return ctx, errors.Create(errors.UnknownError)
		}
		s = append(s, shop)
	}
	shopShopsButton := shopKeyboard.Data("⬅️ Back", "shops_shops", shops.ID)

	shopResetShopAskButton = shopKeyboard.Data("⚠️ Delete all shops", "shops_reset_ask", shops.ID)
	shopKeyboard.Inline(buttonWrapper(append(bot.makseShopSelectionButtons(s, "delete_shop"), shopResetShopAskButton, shopShopsButton), shopKeyboard, 1)...)
	_, err = bot.tryEditMessage(c.Message, "Which shop do you want to delete?", shopKeyboard)
	return ctx, err
}

func (bot *TipBot) shopsAskDeleteAllShopsHandler(ctx context.Context, c *tb.Callback) (context.Context, error) {
	log.Debugf("[shopsAskDeleteAllShopsHandler] %s", c.Data)
	shopResetShopButton := shopKeyboard.Data("⚠️ Delete all shops", "shops_reset", c.Data)
	buttons := []tb.Row{
		shopKeyboard.Row(shopResetShopButton),
		shopKeyboard.Row(shopShopsButton),
	}
	shopKeyboard.Inline(
		buttons...,
	)
	bot.tryEditMessage(c.Message, "Are you sure you want  to delete all shops?\nYou will lose all items as well.", shopKeyboard)
	return ctx, nil
}

// shopsLinkShopBrowser is invoked when the user clicks on "shop links" and makes a list of all shops
func (bot *TipBot) shopsLinkShopBrowser(ctx context.Context, c *tb.Callback) (context.Context, error) {
	log.Debugf("[shopsLinkShopBrowser] %s", c.Data)
	user := LoadUser(ctx)
	shops, err := bot.getUserShops(ctx, user)
	if err != nil {
		return ctx, err
	}
	var s []*Shop
	for _, shopId := range shops.Shops {
		shop, _ := bot.getShop(ctx, shopId)
		if shop.Owner.Telegram.ID != c.Sender.ID {
			return ctx, errors.Create(errors.UnknownError)
		}
		s = append(s, shop)
	}
	shopShopsButton := shopKeyboard.Data("⬅️ Back", "shops_shops", shops.ID)
	shopKeyboard.Inline(buttonWrapper(append(bot.makseShopSelectionButtons(s, "link_shop"), shopShopsButton), shopKeyboard, 1)...)
	_, err = bot.tryEditMessage(c.Message, "Select the shop you want to get the link of.", shopKeyboard)
	return ctx, err
}

// shopSelectLink is invoked when the user has chosen a shop to get the link of
func (bot *TipBot) shopSelectLink(ctx context.Context, c *tb.Callback) (context.Context, error) {
	log.Debugf("[shopSelectLink] %s", c.Data)
	shop, _ := bot.getShop(ctx, c.Data)
	if shop.Owner.Telegram.ID != c.Sender.ID {
		return ctx, errors.Create(errors.UnknownError)
	}
	bot.trySendMessage(c.Sender, fmt.Sprintf("*%s*: `/shop %s`", shop.Title, shop.ID))
	return ctx, nil
}

// shopsLinkShopBrowser is invoked when the user clicks on "shop links" and makes a list of all shops
func (bot *TipBot) shopsRenameShopBrowser(ctx context.Context, c *tb.Callback) (context.Context, error) {
	log.Debugf("[shopsRenameShopBrowser] %s", c.Data)
	user := LoadUser(ctx)
	shops, err := bot.getUserShops(ctx, user)
	if err != nil {
		return ctx, err
	}
	var s []*Shop
	for _, shopId := range shops.Shops {
		shop, _ := bot.getShop(ctx, shopId)
		if shop.Owner.Telegram.ID != c.Sender.ID {
			return ctx, errors.Create(errors.UnknownError)
		}
		s = append(s, shop)
	}
	shopShopsButton := shopKeyboard.Data("⬅️ Back", "shops_shops", shops.ID)
	shopKeyboard.Inline(buttonWrapper(append(bot.makseShopSelectionButtons(s, "rename_shop"), shopShopsButton), shopKeyboard, 1)...)
	_, err = bot.tryEditMessage(c.Message, "Select the shop you want to rename.", shopKeyboard)
	return ctx, err
}

// shopSelectLink is invoked when the user has chosen a shop to get the link of
func (bot *TipBot) shopSelectRename(ctx context.Context, c *tb.Callback) (context.Context, error) {
	log.Debugf("[shopSelectRename] %s", c.Data)
	user := LoadUser(ctx)
	shop, _ := bot.getShop(ctx, c.Data)
	if shop.Owner.Telegram.ID != c.Sender.ID {
		return ctx, errors.Create(errors.UnknownError)
	}
	// We need to save the pay state in the user state so we can load the payment in the next handler
	SetUserState(user, bot, lnbits.UserEnterShopTitle, shop.ID)
	bot.sendStatusMessage(ctx, c.Sender, fmt.Sprintf("⌨️ Enter the name of your shop."), tb.ForceReply)
	return ctx, nil
}

// shopsDescriptionHandler is invoked when the user clicks on "description" to set a shop description
func (bot *TipBot) shopsDescriptionHandler(ctx context.Context, c *tb.Callback) (context.Context, error) {
	log.Debugf("[shopsDescriptionHandler] %s", c.Data)
	user := LoadUser(ctx)
	shops, err := bot.getUserShops(ctx, user)
	if err != nil {
		log.Errorf("[shopsDescriptionHandler] %s", err.Error())
		return ctx, err
	}
	SetUserState(user, bot, lnbits.UserEnterShopsDescription, shops.ID)
	bot.sendStatusMessage(ctx, c.Sender, fmt.Sprintf("⌨️ Enter a description."), tb.ForceReply)
	return ctx, nil
}

// enterShopsDescriptionHandler is invoked when the user enters the shop title
func (bot *TipBot) enterShopsDescriptionHandler(ctx context.Context, m *tb.Message) (context.Context, error) {
	log.Debugf("[enterShopsDescriptionHandler] %s", m.Text)
	user := LoadUser(ctx)
	shops, err := bot.getUserShops(ctx, user)
	if err != nil {
		log.Errorf("[enterShopsDescriptionHandler] %s", err.Error())
		return ctx, err
	}
	if shops.Owner.Telegram.ID != m.Sender.ID {
		return ctx, errors.Create(errors.NotShopOwnerError)
	}
	if len(m.Text) == 0 {
		ResetUserState(user, bot)
		bot.sendStatusMessageAndDelete(ctx, m.Sender, "🚫 Action cancelled.")
		go func() {
			time.Sleep(time.Duration(5) * time.Second)
			bot.shopViewDeleteAllStatusMsgs(ctx, user)
		}()
		return ctx, errors.Create(errors.InvalidSyntaxError)
	}

	// crop shop title
	if len(m.Text) > SHOPS_DESCRIPTION_MAX_LENGTH {
		m.Text = m.Text[:SHOPS_DESCRIPTION_MAX_LENGTH]
	}
	shops.Description = m.Text
	runtime.IgnoreError(shops.Set(shops, bot.ShopBunt))
	bot.sendStatusMessageAndDelete(ctx, m.Sender, fmt.Sprintf("✅ Description set."))
	ResetUserState(user, bot)
	// go func() {
	// 	time.Sleep(time.Duration(5) * time.Second)
	// 	bot.shopViewDeleteAllStatusMsgs(ctx, user)
	// }()
	bot.shopsHandler(ctx, m)
	bot.tryDeleteMessage(m)
	return ctx, nil
}

// shopsResetHandler is invoked when the user clicks button to reset shops completely
func (bot *TipBot) shopsResetHandler(ctx context.Context, c *tb.Callback) (context.Context, error) {
	log.Debugf("[shopsResetHandler] %s", c.Data)
	user := LoadUser(ctx)
	shops, err := bot.getUserShops(ctx, user)
	if err != nil {
		log.Errorf("[shopsResetHandler] %s", err.Error())
		return ctx, err
	}
	if shops.Owner.Telegram.ID != c.Sender.ID {
		return ctx, errors.Create(errors.UnknownError)
	}
	runtime.IgnoreError(shops.Delete(shops, bot.ShopBunt))
	bot.sendStatusMessageAndDelete(ctx, c.Sender, fmt.Sprintf("✅ Shops reset."))
	// go func() {
	// 	time.Sleep(time.Duration(5) * time.Second)
	// 	bot.shopViewDeleteAllStatusMsgs(ctx, user)
	// }()
	return bot.shopsHandlerCallback(ctx, c)
}

// shopSelect is invoked when the user has selected a shop to browse
func (bot *TipBot) shopSelect(ctx context.Context, c *tb.Callback) (context.Context, error) {
	log.Debugf("[shopSelect] %s", c.Data)
	shop, _ := bot.getShop(ctx, c.Data)
	user := LoadUser(ctx)
	shopView, err := bot.getUserShopview(ctx, user)
	if err != nil {
		shopView = ShopView{
			ID:     fmt.Sprintf("shopview-%d", c.Sender.ID),
			ShopID: shop.ID,
			Page:   0,
		}
		return ctx, bot.Cache.Set(shopView.ID, shopView, &store.Options{Expiration: 24 * time.Hour})
	}
	shopView.Page = 0
	shopView.ShopID = shop.ID

	// var shopMessage *tb.Message
	shopMessage := bot.displayShopItem(ctx, c.Message, shop)
	// if len(shop.ItemIds) > 0 {
	// 	bot.tryDeleteMessage(c.Message)
	// 	shopMessage = bot.displayShopItem(ctx, c.Message, shop)
	// } else {
	// 	shopMessage = bot.tryEditMessage(c.Message, "There are no items in this shop yet.", bot.shopMenu(ctx, shop, &ShopItem{}))
	// }
	shopView.Message = shopMessage
	log.Infof("[🛍 shop] %s erntering shop %s.", GetUserStr(user.Telegram), shop.ID)
	ctx = context.WithValue(ctx, "callback_response", fmt.Sprintf("🛍 You are browsing %s", shop.Title))
	return ctx, bot.Cache.Set(shopView.ID, shopView, &store.Options{Expiration: 24 * time.Hour})
}

// shopSelectDelete is invoked when the user has chosen a shop to delete
func (bot *TipBot) shopSelectDelete(ctx context.Context, c *tb.Callback) (context.Context, error) {
	log.Debugf("[shopSelectDelete] %s", c.Data)
	shop, _ := bot.getShop(ctx, c.Data)
	user := LoadUser(ctx)
	shops, err := bot.getUserShops(ctx, user)
	if err != nil {
		return ctx, err
	}
	// first, delete from Shops
	for i, shopId := range shops.Shops {
		if shopId == shop.ID {
			if i == len(shops.Shops)-1 {
				shops.Shops = shops.Shops[:i]
			} else {
				shops.Shops = append(shops.Shops[:i], shops.Shops[i+1:]...)
			}
			break
		}
	}
	runtime.IgnoreError(shops.Set(shops, bot.ShopBunt))

	// then, delete shop
	runtime.IgnoreError(shop.Delete(shop, bot.ShopBunt))

	log.Infof("[🛍 shop] %s deleted shop %s.", GetUserStr(user.Telegram), shop.ID)
	// then update buttons
	return bot.shopsDeleteShopBrowser(ctx, c)
}

// shopsBrowser makes a button list of all shops the user can browse
func (bot *TipBot) shopsBrowser(ctx context.Context, c *tb.Callback) (context.Context, error) {
	log.Debugf("[shopsBrowser] %s", c.Data)
	user := LoadUser(ctx)
	shopView, err := bot.getUserShopview(ctx, user)
	if err != nil {
		return ctx, err
	}
	shops, err := bot.getUserShops(ctx, shopView.ShopOwner)
	if err != nil {
		return ctx, err
	}
	var s []*Shop
	for _, shopId := range shops.Shops {
		shop, _ := bot.getShop(ctx, shopId)
		s = append(s, shop)
	}
	shopShopsButton := shopKeyboard.Data("⬅️ Back", "shops_shops", shops.ID)
	shopKeyboard.Inline(buttonWrapper(append(bot.makseShopSelectionButtons(s, "select_shop"), shopShopsButton), shopKeyboard, 1)...)
	shopMessage, _ := bot.tryEditMessage(c.Message, "Select a shop you want to browse.", shopKeyboard)
	shopView, err = bot.getUserShopview(ctx, user)
	if err != nil {
		shopView.Message = shopMessage
		// todo -- check if this is possible (me like)
		return ctx, fmt.Errorf("%v:%v", err, bot.Cache.Set(shopView.ID, shopView, &store.Options{Expiration: 24 * time.Hour}))
	}
	return ctx, nil
}

// shopItemSettingsHandler is invoked when the user presses the shop settings button
func (bot *TipBot) shopSettingsHandler(ctx context.Context, c *tb.Callback) (context.Context, error) {
	log.Debugf("[shopSettingsHandler] %s", c.Data)
	user := LoadUser(ctx)
	shopView, err := bot.getUserShopview(ctx, user)
	if err != nil {
		return ctx, err
	}
	shops, err := bot.getUserShops(ctx, user)
	if err != nil {
		return ctx, err
	}
	if shops.ID != c.Data || shops.Owner.Telegram.ID != user.Telegram.ID {
		log.Error("[shopSettingsHandler] item id mismatch")
		return ctx, errors.Create(errors.ItemIdMismatchError)
	}
	_, err = bot.tryEditMessage(shopView.Message, shopView.Message.Text, bot.shopsSettingsMenu(ctx, shops))
	return ctx, err
}

// shopNewShopHandler is invoked when the user presses the new shop button
func (bot *TipBot) shopNewShopHandler(ctx context.Context, c *tb.Callback) (context.Context, error) {
	log.Debugf("[shopNewShopHandler] %s", c.Data)
	user := LoadUser(ctx)
	shops, err := bot.getUserShops(ctx, user)
	if err != nil {
		log.Errorf("[shopNewShopHandler] %s", err.Error())
		return ctx, err
	}
	if len(shops.Shops) >= shops.MaxShops {
		bot.trySendMessage(c.Sender, fmt.Sprintf("🚫 You can only have %d shops. Delete a shop to create a new one.", shops.MaxShops))
		return ctx, errors.Create(errors.MaxReachedError)
	}
	shop, err := bot.addUserShop(ctx, user)
	// We need to save the pay state in the user state so we can load the payment in the next handler
	SetUserState(user, bot, lnbits.UserEnterShopTitle, shop.ID)
	bot.sendStatusMessage(ctx, c.Sender, fmt.Sprintf("⌨️ Enter the name of your shop."), tb.ForceReply)
	return ctx, nil
}

// enterShopTitleHandler is invoked when the user enters the shop title
func (bot *TipBot) enterShopTitleHandler(ctx context.Context, m *tb.Message) (context.Context, error) {
	log.Debugf("[enterShopTitleHandler] %s", m.Text)
	user := LoadUser(ctx)
	// read item from user.StateData
	shop, err := bot.getShop(ctx, user.StateData)
	if err != nil {
		return ctx, errors.Create(errors.NoShopError)
	}
	if shop.Owner.Telegram.ID != m.Sender.ID {
		return ctx, errors.Create(errors.ShopNoOwnerError)
	}
	if len(m.Text) == 0 {
		ResetUserState(user, bot)
		bot.sendStatusMessageAndDelete(ctx, m.Sender, "🚫 Action cancelled.")
		go func() {
			time.Sleep(time.Duration(5) * time.Second)
			bot.shopViewDeleteAllStatusMsgs(ctx, user)
		}()
		return ctx, errors.Create(errors.InvalidSyntaxError)
	}
	// crop shop title
	m.Text = strings.Replace(m.Text, "\n", " ", -1)
	if len(m.Text) > SHOP_TITLE_MAX_LENGTH {
		m.Text = m.Text[:SHOP_TITLE_MAX_LENGTH]
	}
	shop.Title = m.Text
	runtime.IgnoreError(shop.Set(shop, bot.ShopBunt))
	bot.sendStatusMessageAndDelete(ctx, m.Sender, fmt.Sprintf("✅ Shop added."))
	ResetUserState(user, bot)
	// go func() {
	// 	time.Sleep(time.Duration(5) * time.Second)
	// 	bot.shopViewDeleteAllStatusMsgs(ctx, user)
	// }()
	bot.shopsHandler(ctx, m)
	bot.tryDeleteMessage(m)
	log.Infof("[🛍 shop] %s added new shop %s.", GetUserStr(user.Telegram), shop.ID)
	return ctx, nil
}
