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
)

var errMap = map[TipBotErrorType]TipBotError{InvalidSyntaxError: InvalidSyntax}
var (
	UserNoWallet   = TipBotError{Err: fmt.Errorf("user has no wallet")}
	NoReplyMessage = TipBotError{Err: fmt.Errorf("no reply message")}
	InvalidSyntax  = TipBotError{Err: fmt.Errorf("invalid syntax")}
	InvalidAmount  = TipBotError{Err: fmt.Errorf("invalid amount")}
)

func Create(code TipBotErrorType) TipBotError {
	return errMap[code]
}
func New(code TipBotErrorType, err error) TipBotError {
	return TipBotError{Err: err, Message: err.Error(), Code: code}
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
