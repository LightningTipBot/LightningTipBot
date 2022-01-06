package api

import (
	"net/http"
	"net/http/httputil"

	log "github.com/sirupsen/logrus"
)

func LoggingMiddleware(prefix string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Debugf("[%s] %s %s", prefix, r.Method, r.URL.Path)
		log.Tracef("[%s]\n%s", prefix, dump(r))
		r.BasicAuth()
		next.ServeHTTP(w, r)
	}
}

func dump(r *http.Request) string {
	x, err := httputil.DumpRequest(r, true)
	if err != nil {
		return ""
	}
	return string(x)
}
