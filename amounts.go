package main

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
	amount, err = getAmount(input)
	return amount, err
}

func getAmount(input string) (amount int, err error) {
	amount, err = strconv.Atoi(strings.Split(input, " ")[1])
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
