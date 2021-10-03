package errors

import "encoding/json"

type TipBotErrorType int

const (
	DecodeAmountError TipBotErrorType = 1000 + iota
	DecodePerUserAmountError
	InvalidAmountError
	InvalidAmountPerUserError
	GetBalanceError
	BalanceToLowError
)

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
