package unused

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// Register registers on a router it is given; nothing in the module calls
// it.
func Register(r chi.Router) {
	r.Get("/orphans", orphan)
}

// Imported registers nothing.
var Imported chi.Router

func orphan(w http.ResponseWriter, r *http.Request) {}
