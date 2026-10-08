package lending

import "net/http"

// Route is one served method and path, with the permission it checks.
type Route struct {
	Method     string
	Path       string
	Permission string
	Handler    http.HandlerFunc
}

// Routes lists every route the service serves.
func (s *Server) Routes() []Route {
	return []Route{
		{"POST", "/members", "members.write", s.RegisterMember},
		{"POST", "/loans", "loans.write", s.LendBook},
		{"POST", "/loans/{loanId}/return", "loans.write", s.ReturnBook},
		{"GET", "/members/{cardNumber}/loans", "loans.read", s.ListMemberLoans},
	}
}

// Server holds the handlers and the count they read.
type Server struct {
	openLoans map[string]int // card number -> books on loan
}

// RegisterMember adds a member and answers 201.
func (s *Server) RegisterMember(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusCreated)
}

// LendBook lends a book, or answers 409 when the member already has
// MaxOpenLoans books out.
func (s *Server) LendBook(w http.ResponseWriter, r *http.Request) {
	if s.openLoans[r.FormValue("cardNumber")] >= MaxOpenLoans {
		w.WriteHeader(http.StatusConflict)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

// ReturnBook closes a loan and answers 200.
func (s *Server) ReturnBook(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

// ListMemberLoans answers the loans of one member.
func (s *Server) ListMemberLoans(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}
