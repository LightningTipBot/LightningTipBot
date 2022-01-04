package api

import (
	log "github.com/sirupsen/logrus"
	"net/http"
)

func LoggingMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
		log.Tracef("[API] %s %s", r.Method, r.URL.String())
	}
}
