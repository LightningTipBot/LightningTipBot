package api

import (
	log "github.com/sirupsen/logrus"
	"net/http"
)

func LoggingMiddleware(prefix string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Tracef("[%s] %s %s", prefix, r.Method, r.URL.String())
		next.ServeHTTP(w, r)
	}
}
