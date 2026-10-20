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
	r.HandleFunc("/proxy/{rest:.*}", proxy).Methods("GET")
	r.HandleFunc("/blobs/{key:[a-z/]+}", serve).Methods("GET")
	r.HandleFunc("/images/{file:.+\\.png}", serve).Methods("GET")
	r.HandleFunc("/repos/{p:[a-z]+/raw}", serve).Methods("GET")
	r.HandleFunc("/tail/{rest:.{1,}}", proxy).Methods("GET")
	r.HandleFunc("/codes/{id:[0-9]{3}}", code).Methods("GET")
	v2 := mux.NewRouter()
	v2.HandleFunc("/v2/items", itemsV2).Methods("GET")
	r.PathPrefix("/v2").Methods("GET").Handler(v2)
	s := &server{r: r}
	s.r.NotFoundHandler = http.HandlerFunc(proxy)
	if strict {
		s.r.MethodNotAllowedHandler = http.HandlerFunc(refuse)
	}
	return r
}

func health(w http.ResponseWriter, r *http.Request)  {}
func items(w http.ResponseWriter, r *http.Request)   {}
func itemsV2(w http.ResponseWriter, r *http.Request) {}
func code(w http.ResponseWriter, r *http.Request)    {}
func serve(w http.ResponseWriter, r *http.Request)   {}
func proxy(w http.ResponseWriter, r *http.Request)   {}
func refuse(w http.ResponseWriter, r *http.Request)  {}
