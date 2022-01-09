package errors

import "fmt"

type TipBotErrorType int

const (
	UnknownError TipBotErrorType = iota
	NoReplyMessageError
	InvalidSyntaxError
	MaxReachedError
	NoPhotoError
	NoFileFoundError
	NotActiveError
	InvalidTypeError
)

const (
	UserNoWalletError TipBotErrorType = 2000 + iota
	BalanceToLowError
	SelfPaymentError
	NoPrivateChatError
	GetBalanceError
	DecodeAmountError
	DecodePerUserAmountError
	InvalidAmountError
	InvalidAmountPerUserError
)

const (
	NoShopError TipBotErrorType = 3000 + iota
	NotShopOwnerError
	ShopNoOwnerError
	ItemIdMismatchError
)

var errMap = map[TipBotErrorType]TipBotError{
	UserNoWalletError:         userNoWallet,
	NoReplyMessageError:       noReplyMessage,
	InvalidSyntaxError:        invalidSyntax,
	InvalidAmountPerUserError: invalidAmount,
	InvalidAmountError:        invalidAmountPerUser,
	NoPrivateChatError:        noPrivateChat,
	ShopNoOwnerError:          shopNoOwner,
	NotShopOwnerError:         notShopOwner,
	MaxReachedError:           maxReached,
	NoShopError:               noShop,
	SelfPaymentError:          selfPayment,
	NoPhotoError:              noPhoto,
	ItemIdMismatchError:       itemIdMismatch,
	NoFileFoundError:          noFileFound,
	UnknownError:              unknown,
	NotActiveError:            notActive,
	InvalidTypeError:          invalidType,
}

var (
	userNoWallet         = TipBotError{Err: fmt.Errorf("user has no wallet")}
	noReplyMessage       = TipBotError{Err: fmt.Errorf("no reply message")}
	invalidSyntax        = TipBotError{Err: fmt.Errorf("invalid syntax")}
	invalidAmount        = TipBotError{Err: fmt.Errorf("invalid amount")}
	invalidAmountPerUser = TipBotError{Err: fmt.Errorf("invalid amount per user")}
	noPrivateChat        = TipBotError{Err: fmt.Errorf("no private chat")}
	shopNoOwner          = TipBotError{Err: fmt.Errorf("shop has no owner")}
	notShopOwner         = TipBotError{Err: fmt.Errorf("user is not shop owner")}
	maxReached           = TipBotError{Err: fmt.Errorf("maximum reached")}
	noShop               = TipBotError{Err: fmt.Errorf("user has no shop")}
	selfPayment          = TipBotError{Err: fmt.Errorf("can't pay yourself")}
	noPhoto              = TipBotError{Err: fmt.Errorf("no photo in message")}
	itemIdMismatch       = TipBotError{Err: fmt.Errorf("item id mismatch")}
	noFileFound          = TipBotError{Err: fmt.Errorf("no file found")}
	unknown              = TipBotError{Err: fmt.Errorf("unknown error")}
	notActive            = TipBotError{Err: fmt.Errorf("element not active")}
	invalidType          = TipBotError{Err: fmt.Errorf("invalid type")}
)
