package stdmux

import "net/http"

// Handler serves the desk's own routes on a ServeMux.
func Handler(admin http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /loans/{loanId}", showLoan)
	mux.HandleFunc("POST /loans", http.HandlerFunc(lendBook))
	mux.HandleFunc("/health", health)
	mux.HandleFunc("GET desk.example.com/status", health)
	mux.HandleFunc("GET /files/{path...}", health)
	mux.HandleFunc("GET /{$}", health)
	mux.HandleFunc("HEAD /loans", health)
	reports := http.NewServeMux()
	reports.HandleFunc("GET /daily", daily)
	mux.Handle("/reports/", http.StripPrefix("/reports", reports))
	mux.Handle("/admin/", admin)
	for _, p := range []string{"/a", "/b"} {
		mux.HandleFunc("GET "+p, health)
	}
	return mux
}

func showLoan(w http.ResponseWriter, r *http.Request) { _ = r.PathValue("loanId") }
func lendBook(w http.ResponseWriter, r *http.Request) {}
func health(w http.ResponseWriter, r *http.Request)   {}
func daily(w http.ResponseWriter, r *http.Request)    {}
