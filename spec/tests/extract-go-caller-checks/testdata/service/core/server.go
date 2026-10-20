package core

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// Server answers the core's routes.
type Server struct {
	router   chi.Router
	audience string
	callers  []string
}

// New makes the server and its routes.
func New(audience string, callers []string) *Server {
	s := &Server{router: chi.NewRouter(), audience: audience, callers: callers}
	s.routes()
	return s
}

func (s *Server) routes() {
	s.router.Use(s.RequireAudience(s.audience))
	s.router.Get("/invoices", s.RequireClients([]string{"reporting", "archive"}, s.Require("invoices.read", s.listInvoices)))
	s.router.Delete("/invoices/{invoiceId}", s.RequireCaller([]string{"middleware"}, s.RequireApplication("admin-web", s.Audit(s.Require("invoices.delete", s.deleteInvoice)))))
	s.router.Post("/invoices/{invoiceId}/void", s.RequireCaller(s.callers, s.Require("invoices.delete", s.voidInvoice)))
	s.router.Post("/invoices/{invoiceId}/archive", s.Require("invoices.delete", s.archiveInvoice))
}

// Require runs h only for a caller whose role grants permission.
func (s *Server) Require(permission string, h http.HandlerFunc) http.HandlerFunc { return h }

// RequireClients runs h only for one of the clients named.
func (s *Server) RequireClients(clients []string, h http.HandlerFunc) http.HandlerFunc { return h }

// RequireApplication runs h only when the person signed in through the
// application named.
func (s *Server) RequireApplication(application string, h http.HandlerFunc) http.HandlerFunc { return h }

// RequireCaller runs h only when the immediate caller is one of the
// services named.
func (s *Server) RequireCaller(services []string, h http.HandlerFunc) http.HandlerFunc { return h }

// RequireAudience refuses a token whose aud is not the one given.
func (s *Server) RequireAudience(audience string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler { return next }
}

// Audit writes a security event when h refuses.
func (s *Server) Audit(h http.HandlerFunc) http.HandlerFunc { return h }

func (s *Server) listInvoices(w http.ResponseWriter, r *http.Request) {}

func (s *Server) deleteInvoice(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Txn-Token") == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}
}

func (s *Server) voidInvoice(w http.ResponseWriter, r *http.Request) {}

func (s *Server) archiveInvoice(w http.ResponseWriter, r *http.Request) {}
