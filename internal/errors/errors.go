package errors

import (
	"encoding/json"
	"fmt"
)

type TipBotErrorType int

const (
	DecodeAmountError TipBotErrorType = 1000 + iota
	DecodePerUserAmountError
	InvalidAmountError
	InvalidAmountPerUserError
	GetBalanceError
	BalanceToLowError
	NoWalletError
	NoReplyMessageError
	InvalidSyntaxError
	NoPrivateChatError
	ShopNoOwnerError
	MaxReachedError
	NotShopOwnerError
	NoShopError
	SelfPaymentError
	NoPhotoError
	ItemIdMismatchError
	NoFileFoundError
	UnknownError
	NotActiveError
)

var errMap = map[TipBotErrorType]TipBotError{
	InvalidSyntaxError:  InvalidSyntax,
	NoPrivateChatError:  NoPrivateChat,
	ShopNoOwnerError:    ShopNoOwner,
	MaxReachedError:     MaxReached,
	NotShopOwnerError:   NotShopOwner,
	NoShopError:         NoShop,
	SelfPaymentError:    SelfPayment,
	NoPhotoError:        NoPhoto,
	ItemIdMismatchError: ItemIdMismatch,
	NoFileFoundError:    NoFileFound,
	UnknownError:        Unknown,
	NotActiveError:      NotActive,
}
var (
	UserNoWallet   = TipBotError{Err: fmt.Errorf("user has no wallet")}
	NoReplyMessage = TipBotError{Err: fmt.Errorf("no reply message")}
	InvalidSyntax  = TipBotError{Err: fmt.Errorf("invalid syntax")}
	InvalidAmount  = TipBotError{Err: fmt.Errorf("invalid amount")}
	NoPrivateChat  = TipBotError{Err: fmt.Errorf("no private chat")}
	ShopNoOwner    = TipBotError{Err: fmt.Errorf("shop has no owner")}
	NotShopOwner   = TipBotError{Err: fmt.Errorf("user is not shop owner")}
	MaxReached     = TipBotError{Err: fmt.Errorf("maximum reached")}
	NoShop         = TipBotError{Err: fmt.Errorf("user has no shop")}
	SelfPayment    = TipBotError{Err: fmt.Errorf("can't pay yourself")}
	NoPhoto        = TipBotError{Err: fmt.Errorf("no photo in message")}
	ItemIdMismatch = TipBotError{Err: fmt.Errorf("item id mismatch")}
	NoFileFound    = TipBotError{Err: fmt.Errorf("no file found")}
	Unknown        = TipBotError{Err: fmt.Errorf("unknown error")}
	NotActive      = TipBotError{Err: fmt.Errorf("element not active")}
)

func Create(code TipBotErrorType) TipBotError {
	return errMap[code]
}
func New(code TipBotErrorType, err error) TipBotError {
	if err != nil {
		return TipBotError{Err: err, Message: err.Error(), Code: code}
	}
	return Create(code)
}

type TipBotError struct {
	Message string `json:"message"`
	Err     error
	Code    TipBotErrorType `json:"code"`
}

func (e TipBotError) Error() string {
	j, err := json.Marshal(&e)
	if err != nil {
		return e.Message
	}
	return string(j)
}
