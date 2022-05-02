package network

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/LightningTipBot/LightningTipBot/internal"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/proxy"
)

func GetHttpClient() (*http.Client, error) {
	client := http.Client{
		Timeout: 10 * time.Second,
	}
	if internal.Configuration.Bot.HttpProxy != "" {
		proxyUrl, err := url.Parse(internal.Configuration.Bot.HttpProxy)
		if err != nil {
			log.Errorln(err)
			return nil, err
		}
		client.Transport = &http.Transport{Proxy: http.ProxyURL(proxyUrl)}
	}
	return &client, nil
}
func GetSocksClient() (*http.Client, error) {
	client := http.Client{
		Timeout: 10 * time.Second,
	}
	if internal.Configuration.Bot.SocksProxy != "" {
		proxyURL, _ := url.Parse(internal.Configuration.Bot.SocksProxy)
		specialTransport := &http.Transport{}
		specialTransport.Proxy = http.ProxyURL(proxyURL)
		d, err := proxy.SOCKS5("tcp", internal.Configuration.Bot.SocksProxy, nil, &net.Dialer{
			Timeout:   20 * time.Second,
			Deadline:  time.Now().Add(time.Second * 10),
			KeepAlive: -1,
		})
		if err != nil {
			log.Errorln(err)
			return &client, nil
		}
		specialTransport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			return d.Dial(network, addr)
		}
		client.Transport = specialTransport
	}
	return &client, nil
}
