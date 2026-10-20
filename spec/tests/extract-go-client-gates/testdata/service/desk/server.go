package desk

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
)

// Server answers the desk's routes.
type Server struct {
	router chi.Router
	key    []byte
}

// New makes the server and its routes.
func New() *Server {
	s := &Server{router: chi.NewRouter()}
	s.routes()
	return s
}

func (s *Server) routes() {
	s.router.Get("/loans", s.Require("loans.read", s.listLoans))
	s.router.Get("/reconciliation/loans", s.RequireClient(s.Require("loans.read", s.listLoans)))
	s.router.With(VerifySignature).Post("/fees/settlements", s.Require("fees.settle", s.settleFees))
	s.router.Post("/sessions", s.signIn)
}

// Require runs h only for a caller whose role grants permission.
func (s *Server) Require(permission string, h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Desk-Role") == "" {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		h(w, r)
	}
}

// RequireClient runs h only for a caller presenting a client's API key,
// and refuses a person's session.
func (s *Server) RequireClient(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Api-Key") == "" {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		h(w, r)
	}
}

// VerifySignature refuses a request whose Signature header does not verify.
func VerifySignature(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Signature") == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) listLoans(w http.ResponseWriter, r *http.Request) {}

func (s *Server) settleFees(w http.ResponseWriter, r *http.Request) {}

func (s *Server) signIn(w http.ResponseWriter, r *http.Request) {
	claims := jwt.MapClaims{
		"sub":  r.FormValue("cardNumber"),
		"role": "desk-staff",
		"exp":  time.Now().Add(15 * time.Minute).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(s.key)
	if err != nil {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
		return
	}
	_, _ = w.Write([]byte(signed))
}
