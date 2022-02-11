package str

import (
	"crypto/sha256"
	"fmt"
	"hash/fnv"
	"strings"

	"github.com/LightningTipBot/LightningTipBot/internal/lnbits"
)

var markdownV2Escapes = []string{"_", "[", "]", "(", ")", "~", "`", ">", "#", "+", "-", "=", "|", "{", "}", ".", "!"}
var markdownEscapes = []string{"_", "*", "`", "["}

func MarkdownV2Escape(s string) string {
	for _, esc := range markdownV2Escapes {
		if strings.Contains(s, esc) {
			s = strings.Replace(s, esc, fmt.Sprintf("\\%s", esc), -1)
		}
	}
	return s
}

func MarkdownEscape(s string) string {
	for _, esc := range markdownEscapes {
		if strings.Contains(s, esc) {
			s = strings.Replace(s, esc, fmt.Sprintf("\\%s", esc), -1)
		}
	}
	return s
}

func Int32Hash(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}

func Int64Hash(s string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(s))
	return h.Sum64()
}

func AnonIdSha256(u *lnbits.User) string {
	h := sha256.Sum256([]byte(u.Wallet.ID))
	hash := fmt.Sprintf("%x", h)
	anon_id := fmt.Sprintf("0x%s", hash[:16]) // starts with 0x because that can't be a valid telegram username
	return anon_id
}

func UUIDSha256(u *lnbits.User) string {
	h := sha256.Sum256([]byte(u.Wallet.ID))
	hash := fmt.Sprintf("%x", h)
	anon_id := fmt.Sprintf("1x%s", hash[len(hash)-16:]) // starts with 1x because that can't be a valid telegram username
	return anon_id
}
