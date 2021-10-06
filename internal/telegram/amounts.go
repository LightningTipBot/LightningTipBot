package telegram

import (
	"errors"
	"strconv"
	"strings"
)

func getArgumentFromCommand(input string, which int) (output string, err error) {
	if len(strings.Split(input, " ")) < which+1 {
		return "", errors.New("message doesn't contain enough arguments")
	}
	output = strings.Split(input, " ")[which]
	return output, nil
}

func decodeAmountFromCommand(input string) (amount int, err error) {
	if len(strings.Split(input, " ")) < 2 {
		errmsg := "message doesn't contain any amount"
		// log.Errorln(errmsg)
		return 0, errors.New(errmsg)
	}
	amount, err = getAmount(strings.Split(input, " ")[1])
	return amount, err
}

func getAmount(input string) (amount int, err error) {
	// convert something like 1.2k into 1200
	if strings.HasSuffix(input, "k") {
		fmount, err := strconv.ParseFloat(strings.TrimSpace(input[:len(input)-1]), 64)
		if err != nil {
			return 0, err
		}
		amount = int(fmount * 1000)
		return amount, err
	}

	amount, err = strconv.Atoi(input)
	if err != nil {
		return 0, err
	}
	if amount < 1 {
		errmsg := "error: Amount must be greater than 0"
		// log.Errorln(errmsg)
		return 0, errors.New(errmsg)
	}
	return amount, err
}
