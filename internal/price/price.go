package price

import (
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
)

type PriceWatcher struct {
	client         *http.Client
	UpdateInterval time.Duration
	Currencies     map[string]string
	Exchanges      map[string]func(string) (float64, error)
}

var (
	Price map[string]float64
	P     *PriceWatcher
)

func NewPriceWatcher() *PriceWatcher {
	pricewatcher := &PriceWatcher{
		client: &http.Client{
			Timeout: time.Second * time.Duration(5),
		},
		Currencies:     map[string]string{"EUR": "€", "GBP": "£", "JPY": "¥", "BRL": "R$", "USD": "$", "RUB": "₽", "TRY": "₺"},
		Exchanges:      make(map[string]func(string) (float64, error), 0),
		UpdateInterval: time.Second * time.Duration(30),
	}
	pricewatcher.Exchanges["coinbase"] = pricewatcher.GetCoinbasePrice
	pricewatcher.Exchanges["bitfinex"] = pricewatcher.GetBitfinexPrice
	Price = make(map[string]float64, 0)
	log.Infof("[PriceWatcher] Watcher started")
	P = pricewatcher
	return pricewatcher
}

func (p *PriceWatcher) Start() {
	go p.Watch()
}

func (p *PriceWatcher) Watch() error {
	for {
		for currency, _ := range p.Currencies {
			avg_price := 0.0
			n_responses := 0
			for exchange, getPrice := range p.Exchanges {
				fprice, err := getPrice(currency)
				if err != nil {
					// log.Debug(err)
					// if one exchanges is down, use the next
					continue
				}
				n_responses++
				avg_price += fprice
				log.Debugf("[PriceWatcher] %s %s price: %f", exchange, currency, fprice)
				time.Sleep(time.Second * time.Duration(2))
			}
			Price[currency] = avg_price / float64(n_responses)
			log.Debugf("[PriceWatcher] Average %s price: %f", currency, Price[currency])
		}
		time.Sleep(p.UpdateInterval)
	}
}

func (p *PriceWatcher) GetCoinbasePrice(currency string) (float64, error) {
	coinbaseEndpoint, err := url.Parse(fmt.Sprintf("https://api.coinbase.com/v2/prices/spot?currency=%s", currency))
	response, err := p.client.Get(coinbaseEndpoint.String())
	if err, ok := err.(net.Error); ok && err.Timeout() {
		return 0, err
	}
	bodyBytes, err := ioutil.ReadAll(response.Body)
	if err != nil {
		log.Debug(err)
		return 0, err
	}
	price := gjson.Get(string(bodyBytes), "data.amount")
	fprice, err := strconv.ParseFloat(strings.TrimSpace(price.String()), 64)
	if err != nil {
		log.Debug(err)
		return 0, err
	}
	return fprice, nil
}

func (p *PriceWatcher) GetBitfinexPrice(currency string) (float64, error) {
	var bitfinexCurrencyToPair = map[string]string{"USD": "btcusd", "EUR": "btceur", "GBP": "btcusd", "JPY": "btcjpy", "BRL": "btcbrl", "RUB": "btcrub", "TRY": "btctry"}
	pair := bitfinexCurrencyToPair[currency]
	bitfinexEndpoint, err := url.Parse(fmt.Sprintf("https://api.bitfinex.com/v1/pubticker/%s", pair))
	response, err := p.client.Get(bitfinexEndpoint.String())
	if err, ok := err.(net.Error); ok && err.Timeout() {
		return 0, err
	}
	bodyBytes, err := ioutil.ReadAll(response.Body)
	if err != nil {
		log.Debug(err)
		return 0, err
	}
	price := gjson.Get(string(bodyBytes), "last_price")
	fprice, err := strconv.ParseFloat(strings.TrimSpace(price.String()), 64)
	if err != nil {
		log.Debug(err)
		return 0, err
	}
	return fprice, nil
}
