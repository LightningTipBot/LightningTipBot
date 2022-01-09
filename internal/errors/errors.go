package errors

import (
	"encoding/json"
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
