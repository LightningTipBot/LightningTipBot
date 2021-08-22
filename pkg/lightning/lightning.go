package lightning

import (
	"strings"
)

// IsInvoice is used to check if a string matches lnbc invoide pattern.
// todo -- probably should add regex and validate length
func IsInvoice(invoice string) bool {
	// invoice string must start with lnbc or lightning:lnbc
	if strings.HasPrefix(invoice, "lnbc") || strings.HasPrefix(invoice, "lightning:lnbc") {
		// invoice string must be a single word
		if !strings.Contains(invoice, " ") {
			return true
		}
	}
	return false
}
