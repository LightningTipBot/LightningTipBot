package api

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	log "github.com/sirupsen/logrus"
)

func Proxy(wr http.ResponseWriter, req *http.Request, rawUrl string) error {

	client := &http.Client{Timeout: time.Second * 30}

	//http: Request.RequestURI can't be set in client requests.
	//http://golang.org/src/pkg/net/http/client.go
	req.RequestURI = ""

	u, err := url.Parse(rawUrl)
	if err != nil {
		http.Error(wr, "Server Error", http.StatusInternalServerError)
		log.Println("ServeHTTP:", err)
		return err
	}
	req.URL.Host = u.Host
	req.URL.Scheme = u.Scheme
	req.Host = req.URL.Host
	resp, err := client.Do(req)
	if err != nil {
		http.Error(wr, "Server Error", http.StatusInternalServerError)
		log.Println("ServeHTTP:", err)
		return err
	}
	defer resp.Body.Close()
	log.Tracef("[Proxy] Proxy request status: %s", resp.Status)
	if resp.StatusCode > 300 {
		return fmt.Errorf("invalid response")
	}
	delHopHeaders(resp.Header)
	copyHeader(wr.Header(), resp.Header)
	wr.WriteHeader(resp.StatusCode)
	_, err = io.Copy(wr, resp.Body)
	return err
}

// Hop-by-hop headers. These are removed when sent to the backend.
// http://www.w3.org/Protocols/rfc2616/rfc2616-sec13.html
var hopHeaders = []string{
	"Connection",
	"Keep-Alive",
	"Proxy-Authenticate",
	"Proxy-Authorization",
	"Te", // canonicalized version of "TE"
	"Trailers",
	"Transfer-Encoding",
	"Upgrade",
}

func copyHeader(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

func delHopHeaders(header http.Header) {
	for _, h := range hopHeaders {
		header.Del(h)
	}
}
