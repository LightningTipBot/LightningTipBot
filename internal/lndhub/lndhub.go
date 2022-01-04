package lndhub

import (
	"github.com/LightningTipBot/LightningTipBot/internal"
	"github.com/LightningTipBot/LightningTipBot/internal/api"
	"net/http"
)

type LndHub struct {
}

func New() LndHub {
	return LndHub{}
}
func (w LndHub) Handle(writer http.ResponseWriter, request *http.Request) {
	api.Proxy(writer, request, internal.Configuration.Lnbits.Url)
}
