package lightning

import (
	"net/mail"
	"strings"
)

// IsInvoice is used to check if a string matches lnbc invoide pattern.
// todo -- probably should add regex and validate length
func IsInvoice(message string) bool {
	message = strings.ToLower(message)
	// invoice string must start with lnbc or lightning:lnbc
	if strings.HasPrefix(message, "lnbc") || strings.HasPrefix(message, "lightning:lnbc") {
		// invoice string must be a single word
		if !strings.Contains(message, " ") {
			return true
		}
	}
	return false
}

func IsLnurl(message string) bool {
	message = strings.ToLower(message)
	if strings.HasPrefix(message, "lnurl") {
		// string must be a single word
		if !strings.Contains(message, " ") {
			return true
		}
	}
	return false
}

func IsLightningAddress(address string) bool {
	_, err := mail.ParseAddress(address)
	return err == nil
}
