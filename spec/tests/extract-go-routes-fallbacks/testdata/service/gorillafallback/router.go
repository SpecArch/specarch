package gorillafallback

import (
	"net/http"

	"github.com/gorilla/mux"
)

// Router serves the health route itself and forwards the rest.
func Router() *mux.Router {
	r := mux.NewRouter()
	r.HandleFunc("/started", health).Methods("GET")
	r.NotFoundHandler = http.HandlerFunc(proxy)
	r.MethodNotAllowedHandler = http.HandlerFunc(refuse)
	return r
}

func health(w http.ResponseWriter, r *http.Request) {}
func proxy(w http.ResponseWriter, r *http.Request)  {}
func refuse(w http.ResponseWriter, r *http.Request) {}
