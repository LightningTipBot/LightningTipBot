package telegram

import (
	"context"
	"fmt"
	"time"

	"github.com/LightningTipBot/LightningTipBot/internal/lnbits"
	"github.com/LightningTipBot/LightningTipBot/internal/runtime"
	"github.com/LightningTipBot/LightningTipBot/internal/storage/transaction"
	"github.com/eko/gocache/store"
	log "github.com/sirupsen/logrus"
	tb "gopkg.in/lightningtipbot/telebot.v2"
)

func (bot TipBot) shopsMainMenu(ctx context.Context, shops *Shops) *tb.ReplyMarkup {
	browseShopButton := shopKeyboard.Data("🛍 Browse shops", "shops_browse", shops.ID)
	shopNewShopButton := shopKeyboard.Data("✅ New Shop", "shops_newshop", shops.ID)
	shopSettingsButton := shopKeyboard.Data("⚙️ Settings", "shops_settings", shops.ID)
	user := LoadUser(ctx)

	buttons := []tb.Row{}
	if len(shops.Shops) > 0 {
		buttons = append(buttons, shopKeyboard.Row(browseShopButton))
	}
	if user.Telegram.ID == shops.Owner.Telegram.ID {
		buttons = append(buttons, shopKeyboard.Row(shopNewShopButton, shopSettingsButton))
	}
	shopKeyboard.Inline(
		buttons...,
	)
	return shopKeyboard
}

func (bot TipBot) shopsSettingsMenu(ctx context.Context, shops *Shops) *tb.ReplyMarkup {
	shopShopsButton := shopKeyboard.Data("⬅️ Back", "shops_shops", shops.ID)
	shopLinkShopButton := shopKeyboard.Data("🔗 Shop links", "shops_linkshop", shops.ID)
	shopDeleteShopButton := shopKeyboard.Data("🚫 Delete one shop", "shops_deleteshop", shops.ID)
	shopDescriptionShopButton := shopKeyboard.Data("💬 Description", "shops_description", shops.ID)
	shopResetShopButton := shopKeyboard.Data("⚠️ Delete all shops", "shops_reset", shops.ID)
	buttons := []tb.Row{
		shopKeyboard.Row(shopLinkShopButton),
		shopKeyboard.Row(shopDescriptionShopButton),
		shopKeyboard.Row(shopDeleteShopButton),
		shopKeyboard.Row(shopResetShopButton),
		shopKeyboard.Row(shopShopsButton),
	}
	shopKeyboard.Inline(
		buttons...,
	)
	return shopKeyboard
}

// shopItemSettingsMenu builds the buttons of the item settings
func (bot TipBot) shopItemSettingsMenu(ctx context.Context, shop *Shop, item *ShopItem) *tb.ReplyMarkup {
	shopItemPriceButton = shopKeyboard.Data("💯 Set price", "shop_itemprice", item.ID)
	shopItemDeleteButton = shopKeyboard.Data("🚫 Delete item", "shop_itemdelete", item.ID)
	shopItemTitleButton = shopKeyboard.Data("⌨️ Set title", "shop_itemtitle", item.ID)
	shopItemAddFileButton = shopKeyboard.Data("💾 Add file", "shop_itemaddfile", item.ID)
	shopItemSettingsBackButton = shopKeyboard.Data("⬅️ Back", "shop_itemsettingsback", item.ID)
	user := LoadUser(ctx)
	buttons := []tb.Row{}
	if user.Telegram.ID == shop.Owner.Telegram.ID {
		buttons = append(buttons, shopKeyboard.Row(shopItemDeleteButton, shopItemSettingsBackButton))
		buttons = append(buttons, shopKeyboard.Row(shopItemTitleButton, shopItemPriceButton))
		buttons = append(buttons, shopKeyboard.Row(shopItemAddFileButton))
	}
	shopKeyboard.Inline(
		buttons...,
	)
	return shopKeyboard
}

// shopMenu builds the buttons in the item browser
func (bot TipBot) shopMenu(ctx context.Context, shop *Shop, item *ShopItem) *tb.ReplyMarkup {
	user := LoadUser(ctx)
	shopView, err := bot.getUserShopview(ctx, user)
	if err != nil {
		return nil
	}

	shopShopsButton := shopKeyboard.Data("⬅️ Back", "shops_shops", shop.ShopsID)
	shopAddItemButton = shopKeyboard.Data("✅ New item", "shop_additem", shop.ID)
	shopItemSettingsButton = shopKeyboard.Data("⚙️ Settings", "shop_itemsettings", item.ID)
	shopNextitemButton = shopKeyboard.Data(">", "shop_nextitem", shop.ID)
	shopPrevitemButton = shopKeyboard.Data("<", "shop_previtem", shop.ID)
	buyButtonText := "📩 Get"
	if item.Price > 0 {
		buyButtonText = fmt.Sprintf("💸 Buy (%d sat)", item.Price)
	}
	shopBuyitemButton = shopKeyboard.Data(buyButtonText, "shop_buyitem", item.ID)

	buttons := []tb.Row{}
	if user.Telegram.ID == shop.Owner.Telegram.ID {
		if len(shop.Items) == 0 {
			buttons = append(buttons, shopKeyboard.Row(shopAddItemButton))
		} else {
			buttons = append(buttons, shopKeyboard.Row(shopAddItemButton, shopItemSettingsButton))
		}
	}
	// publicButtons := []tb.Row{}
	if len(shop.Items) > 0 {
		if shopView.Page == len(shop.Items)-1 {
			// last page
			shopNextitemButton = shopKeyboard.Data("x", "shop_nextitem", shop.ID)
		}
		buttons = append(buttons, shopKeyboard.Row(shopPrevitemButton, shopBuyitemButton, shopNextitemButton))
	}
	buttons = append(buttons, shopKeyboard.Row(shopShopsButton))
	shopKeyboard.Inline(
		buttons...,
	)
	return shopKeyboard
}

// makseShopSelectionButtons produces a list of all buttons with a uniqueString ID
func (bot *TipBot) makseShopSelectionButtons(shops []*Shop, uniqueString string) []tb.Btn {
	var buttons []tb.Btn
	for _, shop := range shops {
		buttons = append(buttons, shopKeyboard.Data(shop.Title, uniqueString, shop.ID))
	}
	return buttons
}

// -------------- ShopView --------------

// getUserShopview returns ShopView object from cache that holds information about the user's current browsing view
func (bot *TipBot) getUserShopview(ctx context.Context, user *lnbits.User) (shopView ShopView, err error) {
	sv, err := bot.Cache.Get(fmt.Sprintf("shopview-%d", user.Telegram.ID))
	if err != nil {
		return
	}
	shopView = sv.(ShopView)
	return
}
func (bot *TipBot) shopViewDeleteAllStatusMsgs(ctx context.Context, user *lnbits.User) (shopView ShopView, err error) {
	shopView, err = bot.getUserShopview(ctx, user)
	if err != nil {
		return
	}
	for _, msg := range shopView.StatusMessages {
		bot.tryDeleteMessage(msg)
	}
	bot.Cache.Set(shopView.ID, shopView, &store.Options{Expiration: 24 * time.Hour})
	return
}

func (bot *TipBot) sendStatusMessage(ctx context.Context, to tb.Recipient, what interface{}, options ...interface{}) (msg *tb.Message) {
	user := LoadUser(ctx)
	shopView, err := bot.getUserShopview(ctx, user)
	if err != nil {
		return nil
	}
	statusMsg := bot.trySendMessage(to, what)
	shopView.StatusMessages = append(shopView.StatusMessages, statusMsg)
	bot.Cache.Set(shopView.ID, shopView, &store.Options{Expiration: 24 * time.Hour})
	return statusMsg
}

// --------------- Shop ---------------

// initUserShops is a helper function for creating a Shops for the user in the database
func (bot *TipBot) initUserShops(ctx context.Context, user *lnbits.User) (*Shops, error) {
	id := fmt.Sprintf("shops-%d", user.Telegram.ID)
	shops := &Shops{
		Base:     transaction.New(transaction.ID(id)),
		ID:       id,
		Owner:    user,
		Shops:    []string{},
		MaxShops: MAX_SHOPS,
	}
	runtime.IgnoreError(shops.Set(shops, bot.Bunt))
	return shops, nil
}

// getUserShops returns the Shops for the user
func (bot *TipBot) getUserShops(ctx context.Context, user *lnbits.User) (*Shops, error) {
	tx := &Shops{Base: transaction.New(transaction.ID(fmt.Sprintf("shops-%d", user.Telegram.ID)))}
	sn, err := tx.Get(tx, bot.Bunt)
	if err != nil {
		log.Errorf("[getUserShops] User: %s (%d): %s", GetUserStr(user.Telegram), user.Telegram.ID, err)
		return &Shops{}, err
	}
	shops := sn.(*Shops)
	return shops, nil
}

// addUserShop adds a new Shop to the Shops of a user
func (bot *TipBot) addUserShop(ctx context.Context, user *lnbits.User) (*Shop, error) {
	shops, err := bot.getUserShops(ctx, user)
	if err != nil {
		return &Shop{}, err
	}
	shopId := fmt.Sprintf("shop-%s", RandStringRunes(10))
	shop := &Shop{
		Base:         transaction.New(transaction.ID(shopId)),
		ID:           shopId,
		Title:        fmt.Sprintf("Shop %d (%s)", len(shops.Shops)+1, shopId),
		Owner:        user,
		Type:         "photo",
		Items:        make(map[string]ShopItem),
		LanguageCode: ctx.Value("publicLanguageCode").(string),
		ShopsID:      shops.ID,
		MaxItems:     MAX_ITEMS_PER_SHOP,
	}
	runtime.IgnoreError(shop.Set(shop, bot.Bunt))
	shops.Shops = append(shops.Shops, shopId)
	runtime.IgnoreError(shops.Set(shops, bot.Bunt))
	return shop, nil
}

// getShop returns the Shop for the given ID
func (bot *TipBot) getShop(ctx context.Context, shopId string) (*Shop, error) {
	tx := &Shop{Base: transaction.New(transaction.ID(shopId))}
	sn, err := tx.Get(tx, bot.Bunt)
	// immediatelly set intransaction to block duplicate calls
	if err != nil {
		log.Errorf("[getShop] %s", err)
		return &Shop{}, err
	}
	shop := sn.(*Shop)
	if shop.Owner == nil {
		return &Shop{}, fmt.Errorf("shop has no owner")
	}
	return shop, nil
}
