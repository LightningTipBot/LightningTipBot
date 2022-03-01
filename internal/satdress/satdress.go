package satdress

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/LightningTipBot/LightningTipBot/internal"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// from github.com/fiatjaf/makeinvoice

var HttpProxyURL = internal.Configuration.Bot.HttpProxy

var Client = &http.Client{
	Timeout: 10 * time.Second,
}

type LNDParams struct {
	Cert       []byte `json:"to" gorm:"-"`
	CertString string `json:"certstring"`
	Host       string `json:"host"`
	Macaroon   string `json:"macaroon"`
}

func (l LNDParams) getCert() []byte { return l.Cert }
func (l LNDParams) isLocal() bool   { return strings.HasPrefix(l.Host, "https://127.0.0.1") }
func (l LNDParams) isTor() bool     { return strings.Index(l.Host, ".onion") != -1 }

type BackendParams interface {
	getCert() []byte
	isTor() bool
	isLocal() bool
}

type GetInvoiceParams struct {
	Backend         BackendParams
	Msatoshi        int64
	Description     string
	DescriptionHash []byte

	Label string // only used for c-lightning
}

type CheckInvoiceParams struct {
	Backend BackendParams
	PR      string
	Hash    []byte
	Status  string
}

func GetInvoice(params GetInvoiceParams) (CheckInvoiceParams, error) {
	defer func(prevTransport http.RoundTripper) {
		Client.Transport = prevTransport
	}(Client.Transport)

	specialTransport := &http.Transport{}

	// use a cert or skip TLS verification?
	if len(params.Backend.getCert()) > 0 {
		caCertPool := x509.NewCertPool()
		ok := caCertPool.AppendCertsFromPEM([]byte(params.Backend.getCert()))
		if !ok {
			return CheckInvoiceParams{}, fmt.Errorf("invalid root certificate")
		}
		specialTransport.TLSClientConfig = &tls.Config{RootCAs: caCertPool}
	} else {
		specialTransport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}

	// use a proxy?
	if !params.Backend.isLocal() && len(HttpProxyURL) > 0 {
		proxyURL, _ := url.Parse(HttpProxyURL)
		specialTransport.Proxy = http.ProxyURL(proxyURL)
	}

	Client.Transport = specialTransport

	// description hash?
	var _, b64h string
	if params.DescriptionHash != nil {
		_ = hex.EncodeToString(params.DescriptionHash)
		b64h = base64.StdEncoding.EncodeToString(params.DescriptionHash)
	}

	switch backend := params.Backend.(type) {
	case LNDParams:
		body, _ := sjson.Set("{}", "value_msat", params.Msatoshi)

		if params.DescriptionHash == nil {
			body, _ = sjson.Set(body, "memo", params.Description)
		} else {
			body, _ = sjson.Set(body, "description_hash", b64h)
		}

		req, err := http.NewRequest("POST",
			backend.Host+"/v1/invoices",
			bytes.NewBufferString(body),
		)
		if err != nil {
			return CheckInvoiceParams{}, err
		}

		// macaroon must be hex, so if it is on base64 we adjust that
		if b, err := base64.StdEncoding.DecodeString(backend.Macaroon); err == nil {
			backend.Macaroon = hex.EncodeToString(b)
		}

		req.Header.Set("Grpc-Metadata-macaroon", backend.Macaroon)
		resp, err := Client.Do(req)
		if err != nil {
			return CheckInvoiceParams{}, err
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 300 {
			body, _ := ioutil.ReadAll(resp.Body)
			text := string(body)
			if len(text) > 300 {
				text = text[:300]
			}
			return CheckInvoiceParams{}, fmt.Errorf("call to lnd failed (%d): %s", resp.StatusCode, text)
		}

		b, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return CheckInvoiceParams{}, err
		}

		// bot.Cache.Set(shopView.ID, shopView, &store.Options{Expiration: 24 * time.Hour})
		checkInvoiceParams := CheckInvoiceParams{
			Backend: params.Backend,
			PR:      gjson.ParseBytes(b).Get("payment_request").String(),
			Hash:    []byte(gjson.ParseBytes(b).Get("r_hash").String()),
			Status:  "OPEN",
		}
		return checkInvoiceParams, nil
	}
	return CheckInvoiceParams{}, errors.New("missing backend params")
}

func CheckInvoice(params CheckInvoiceParams) (CheckInvoiceParams, error) {
	defer func(prevTransport http.RoundTripper) {
		Client.Transport = prevTransport
	}(Client.Transport)

	specialTransport := &http.Transport{}

	// use a cert or skip TLS verification?
	if len(params.Backend.getCert()) > 0 {
		caCertPool := x509.NewCertPool()
		ok := caCertPool.AppendCertsFromPEM(params.Backend.getCert())
		if !ok {
			return CheckInvoiceParams{}, fmt.Errorf("invalid root certificate")
		}
		specialTransport.TLSClientConfig = &tls.Config{RootCAs: caCertPool}
	} else {
		specialTransport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}

	// use a proxy?
	if !params.Backend.isLocal() && len(HttpProxyURL) > 0 {
		proxyURL, _ := url.Parse(HttpProxyURL)
		specialTransport.Proxy = http.ProxyURL(proxyURL)
	}

	Client.Transport = specialTransport

	switch backend := params.Backend.(type) {
	case LNDParams:
		fmt.Printf("%s", base64.StdEncoding.EncodeToString(params.Hash))
		p, err := base64.StdEncoding.DecodeString(string(params.Hash))
		if err != nil {
			return CheckInvoiceParams{}, fmt.Errorf("invalid hash")
		}
		hexHash := hex.EncodeToString(p)
		requestUrl, err := url.Parse(fmt.Sprintf("%s/v1/invoice/%s?r_hash=%s", backend.Host, hexHash, base64.StdEncoding.EncodeToString(params.Hash)))
		if err != nil {
			return CheckInvoiceParams{}, err
		}
		requestUrl.Scheme = "https"
		req, err := http.NewRequest("GET",
			requestUrl.String(), nil)
		if err != nil {
			return CheckInvoiceParams{}, err
		}
		// macaroon must be hex, so if it is on base64 we adjust that
		if b, err := base64.StdEncoding.DecodeString(backend.Macaroon); err == nil {
			backend.Macaroon = hex.EncodeToString(b)
		}

		req.Header.Set("Grpc-Metadata-macaroon", backend.Macaroon)
		resp, err := Client.Do(req)
		if err != nil {
			return CheckInvoiceParams{}, err
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 300 {
			body, _ := ioutil.ReadAll(resp.Body)
			text := string(body)
			if len(text) > 300 {
				text = text[:300]
			}
			return CheckInvoiceParams{}, fmt.Errorf("call to lnd failed (%d): %s", resp.StatusCode, text)
		}

		b, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return CheckInvoiceParams{}, err
		}
		// bot.Cache.Set(shopView.ID, shopView, &store.Options{Expiration: 24 * time.Hour})
		params.Status = gjson.ParseBytes(b).Get("state").String()
		return params, nil
	}
	return CheckInvoiceParams{}, errors.New("missing backend params")
}
