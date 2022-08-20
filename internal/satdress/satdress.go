package satdress

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/LightningTipBot/LightningTipBot/internal/network"
	log "github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Much of this is from github.com/fiatjaf/makeinvoice
// but with added "checkInvoice" and http proxy support

type LNDParams struct {
	Cert       []byte `json:"cert" gorm:"-"`
	CertString string `json:"certstring"`
	Host       string `json:"host"`
	Macaroon   string `json:"macaroon"`
}

func (l LNDParams) getCert() []byte { return l.Cert }
func (l LNDParams) isLocal() bool   { return strings.HasPrefix(l.Host, "https://127.0.0.1") }
func (l LNDParams) isTor() bool     { return strings.Index(l.Host, ".onion") != -1 }

type LNBitsParams struct {
	Cert string `json:"certstring"`
	Host string `json:"host"`
	Key  string `json:"key"`
}

func (l LNBitsParams) getCert() []byte { return []byte(l.Cert) }
func (l LNBitsParams) isTor() bool     { return strings.Index(l.Host, ".onion") != -1 }
func (l LNBitsParams) isLocal() bool   { return strings.HasPrefix(l.Host, "https://127.0.0.1") }

type BackendParams interface {
	getCert() []byte
	isTor() bool
	isLocal() bool
}

type Params struct {
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

func SetupHttpClient(useProxy bool, cert []byte) (*http.Client, error) {
	var client *http.Client
	if !useProxy {
		client = &http.Client{
			Timeout: 10 * time.Second,
		}
	} else {
		var err error
		client, err = network.GetClient(network.ClientTypeTor)
		if err != nil {
			return nil, err
		}

	}
	// use a cert or skip TLS verification?
	if len(cert) > 0 {
		caCertPool := x509.NewCertPool()
		ok := caCertPool.AppendCertsFromPEM(cert)
		if !ok {
			return client, fmt.Errorf("invalid root certificate")
		}
		client.Transport.(*http.Transport).TLSClientConfig = &tls.Config{RootCAs: caCertPool}
	} else {
		client.Transport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
	return client, nil
}

func MakeInvoice(params Params) (CheckInvoiceParams, error) {
	// defer func(prevTransport http.RoundTripper) {
	// 	Client.Transport = prevTransport
	// }(Client.Transport)

	if params.Backend == nil {
		return CheckInvoiceParams{}, errors.New("no backend specified")
	}

	var err error
	Client, err := SetupHttpClient(!params.Backend.isLocal(), params.Backend.getCert())
	if err != nil {
		return CheckInvoiceParams{}, err
	}

	// description hash?
	var hexh, b64h string
	if params.DescriptionHash != nil {
		hexh = hex.EncodeToString(params.DescriptionHash)
		b64h = base64.StdEncoding.EncodeToString(params.DescriptionHash)
	}

	switch backend := params.Backend.(type) {
	case LNDParams:
		log.WithFields(log.Fields{
			"module": "satdress",
			"func":   "MakeInvoice",
			"host":   backend.Host}).Debugf("LND invoice")
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
		if len(checkInvoiceParams.PR) == 0 {
			return CheckInvoiceParams{}, errors.New("could not create invoice")
		}
		return checkInvoiceParams, nil

	case LNBitsParams:
		log.WithFields(log.Fields{
			"module": "satdress",
			"func":   "MakeInvoice",
			"host":   backend.Host}).Debug("LNBits invoice")
		body, _ := sjson.Set("{}", "amount", params.Msatoshi/1000)
		body, _ = sjson.Set(body, "out", false)

		if params.DescriptionHash == nil {
			if params.Description == "" {
				body, _ = sjson.Set(body, "memo", "invoice")
			} else {
				body, _ = sjson.Set(body, "memo", params.Description)
			}
		} else {
			body, _ = sjson.Set(body, "description_hash", hexh)
		}

		req, err := http.NewRequest("POST",
			backend.Host+"/api/v1/payments",
			bytes.NewBufferString(body),
		)
		if err != nil {
			return CheckInvoiceParams{}, err
		}

		req.Header.Set("X-Api-Key", backend.Key)
		req.Header.Set("Content-Type", "application/json")
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
			return CheckInvoiceParams{}, fmt.Errorf("call to lnbits failed (%d): %s", resp.StatusCode, text)
		}
		b, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return CheckInvoiceParams{}, err
		}
		checkInvoiceParams := CheckInvoiceParams{
			Backend: params.Backend,
			PR:      gjson.ParseBytes(b).Get("payment_request").String(),
			Hash:    []byte(gjson.ParseBytes(b).Get("payment_hash").String()),
			Status:  "OPEN",
		}
		if len(checkInvoiceParams.PR) == 0 {
			return CheckInvoiceParams{}, errors.New("could not create invoice")
		}
		return checkInvoiceParams, nil
	default:
		return CheckInvoiceParams{}, errors.New("wrong backend type")
	}
}

func CheckInvoice(params CheckInvoiceParams) (CheckInvoiceParams, error) {
	// defer func(prevTransport http.RoundTripper) {
	// 	Client.Transport = prevTransport
	// }(Client.Transport)

	if params.Backend == nil {
		return CheckInvoiceParams{}, errors.New("no backend specified")
	}

	var err error
	Client, err := SetupHttpClient(!params.Backend.isLocal(), params.Backend.getCert())
	if err != nil {
		return CheckInvoiceParams{}, err
	}

	switch backend := params.Backend.(type) {
	case LNDParams:
		log.WithFields(log.Fields{
			"module":  "satdress",
			"func":    "CheckInvoice",
			"invoice": base64.StdEncoding.EncodeToString(params.Hash),
			"host":    backend.Host}).Debugf("[CheckInvoice] LND invoice")
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
		params.Status = gjson.ParseBytes(b).Get("state").String()
		return params, nil

	case LNBitsParams:
		log.WithFields(log.Fields{
			"module":  "satdress",
			"func":    "CheckInvoice",
			"invoice": base64.StdEncoding.EncodeToString(params.Hash),
			"host":    backend.Host}).Debugf("LNBits invoice")
		log.WithFields(log.Fields{
			"module":  "satdress",
			"func":    "CheckInvoice",
			"invoice": base64.StdEncoding.EncodeToString(params.Hash),
			"host":    backend.Host + "/api/v1/payments/" + string(params.Hash)}).Debug("Fetching GET")
		req, err := http.NewRequest("GET", backend.Host+"/api/v1/payments/"+string(params.Hash), nil)
		if err != nil {
			return CheckInvoiceParams{}, err
		}

		req.Header.Set("X-Api-Key", backend.Key)
		req.Header.Set("Content-Type", "application/json")
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
			return CheckInvoiceParams{}, fmt.Errorf("call to lnbits failed (%d): %s", resp.StatusCode, text)
		}
		b, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return CheckInvoiceParams{}, err
		}
		status := strings.ToLower(gjson.ParseBytes(b).Get("paid").String())
		if status == "true" {
			params.Status = "SETTLED"
		} else {
			params.Status = "OPEN"
		}
		return params, nil
	default:
		return CheckInvoiceParams{}, errors.New("missing backend params")
	}
}
