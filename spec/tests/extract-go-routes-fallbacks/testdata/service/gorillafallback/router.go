package gorillafallback

import (
	"net/http"

	"github.com/gorilla/mux"
)

// server keeps the router in a field.
type server struct{ r *mux.Router }

var strict = true

// Router serves the health route itself, hands the API's routes to its
// own router, serves files under a prefix and forwards the rest.
func Router() *mux.Router {
	r := mux.NewRouter()
	r.HandleFunc("/started", health).Methods("GET")
	api := mux.NewRouter()
	api.HandleFunc("/api/items", items).Methods("GET")
	r.PathPrefix("/api").Handler(api)
	r.PathPrefix("/files").Handler(http.HandlerFunc(serve))
	s := &server{r: r}
	s.r.NotFoundHandler = http.HandlerFunc(proxy)
	if strict {
		s.r.MethodNotAllowedHandler = http.HandlerFunc(refuse)
	}
	return r
}

func health(w http.ResponseWriter, r *http.Request) {}
func items(w http.ResponseWriter, r *http.Request)  {}
func serve(w http.ResponseWriter, r *http.Request)  {}
func proxy(w http.ResponseWriter, r *http.Request)  {}
func refuse(w http.ResponseWriter, r *http.Request) {}
