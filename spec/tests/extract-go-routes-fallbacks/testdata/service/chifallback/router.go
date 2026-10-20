package chifallback

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// Router serves the health route itself and forwards the rest.
func Router() http.Handler {
	r := chi.NewRouter()
	r.Get("/ready", health)
	r.Handle("/assets/*", http.HandlerFunc(serve))
	r.NotFound(proxy)
	r.MethodNotAllowed(refuse)
	return r
}

func health(w http.ResponseWriter, r *http.Request) {}
func serve(w http.ResponseWriter, r *http.Request)  {}
func proxy(w http.ResponseWriter, r *http.Request)  {}
func refuse(w http.ResponseWriter, r *http.Request) {}
