package api

import (
	log "github.com/sirupsen/logrus"
	"net/http"
	"net/http/httputil"
)

func LoggingMiddleware(prefix string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Tracef("[%s]\n%s", prefix, dump(r))
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
