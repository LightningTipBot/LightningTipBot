package telegram

import (
	"errors"
	"strconv"
	"strings"

	"github.com/LightningTipBot/LightningTipBot/internal/price"
	log "github.com/sirupsen/logrus"
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
	if strings.HasSuffix(strings.ToLower(input), "k") {
		fmount, err := strconv.ParseFloat(strings.TrimSpace(input[:len(input)-1]), 64)
		if err != nil {
			return 0, err
		}
		amount = int(fmount * 1000)
		return amount, err
	}

	// convert fiat currencies to satoshis
	for currency, symbol := range price.P.Currencies {
		if strings.HasPrefix(input, symbol) || strings.HasSuffix(input, symbol) || // for 1$ and $1
			strings.HasPrefix(strings.ToLower(input), strings.ToLower(currency)) || // for USD1
			strings.HasSuffix(strings.ToLower(input), strings.ToLower(currency)) { // for 1USD
			numeric_string := ""
			numeric_string = strings.Replace(input, symbol, "", 1)                                              // for symbol like $
			numeric_string = strings.Replace(strings.ToLower(numeric_string), strings.ToLower(currency), "", 1) // for 1USD
			fmount, err := strconv.ParseFloat(numeric_string, 64)
			if err != nil {
				log.Errorln(err)
				return 0, err
			}
			if !(price.Price[currency] > 0) {
				return 0, errors.New("price is zero")
			}
			amount = int(fmount / price.Price[currency] * float64(100_000_000))
			return amount, nil
		}
	}

	// use plain integer as satoshis
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
