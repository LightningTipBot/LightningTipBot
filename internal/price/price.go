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
	Currencies     []string
	Exchanges      map[string]func(string) (float64, error)
	Price          map[string]float64
}

func NewPriceWatcher() *PriceWatcher {
	pricewatcher := &PriceWatcher{
		client: &http.Client{
			Timeout: time.Second * time.Duration(5),
		},
		Currencies:     []string{"USD", "EUR"},
		Price:          make(map[string]float64, 0),
		Exchanges:      make(map[string]func(string) (float64, error), 0),
		UpdateInterval: time.Second * time.Duration(30),
	}
	pricewatcher.Exchanges["coinbase"] = pricewatcher.GetCoinbasePrice
	pricewatcher.Exchanges["bitfinex"] = pricewatcher.GetBitfinexPrice
	log.Infof("[PriceWatcher] Watcher started")
	return pricewatcher
}

func (p *PriceWatcher) Start() {
	go p.Watch()
}

func (p *PriceWatcher) Watch() error {
	for {
		time.Sleep(p.UpdateInterval)
		for _, currency := range p.Currencies {
			avg_price := 0.0
			n_responses := 0
			for exchange, getPrice := range p.Exchanges {
				fprice, err := getPrice(currency)
				if err != nil {
					log.Error(err)
					// if one exchanges is down, use the next
					continue
				}
				n_responses++
				avg_price += fprice
				log.Debugf("[PriceWatcher] %s %s price: %f", exchange, currency, fprice)
			}
			p.Price[currency] = avg_price / float64(n_responses)
			log.Debugf("[PriceWatcher] Average %s price: %f", currency, p.Price[currency])
		}
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
		log.Fatal(err)
	}
	price := gjson.Get(string(bodyBytes), "data.amount")
	fprice, err := strconv.ParseFloat(strings.TrimSpace(price.String()), 64)
	if err != nil {
		log.Fatal(err)
	}
	return fprice, nil
}

func (p *PriceWatcher) GetBitfinexPrice(currency string) (float64, error) {
	var bitfinexCurrencyToPair = map[string]string{"USD": "btcusd", "EUR": "btceur"}
	pair := bitfinexCurrencyToPair[currency]
	bitfinexEndpoint, err := url.Parse(fmt.Sprintf("https://api.bitfinex.com/v1/pubticker/%s", pair))
	response, err := p.client.Get(bitfinexEndpoint.String())
	if err, ok := err.(net.Error); ok && err.Timeout() {
		return 0, err
	}
	bodyBytes, err := ioutil.ReadAll(response.Body)
	if err != nil {
		log.Fatal(err)
	}
	price := gjson.Get(string(bodyBytes), "last_price")
	fprice, err := strconv.ParseFloat(strings.TrimSpace(price.String()), 64)
	if err != nil {
		log.Fatal(err)
	}
	return fprice, nil
}
