package main

import (
	"flag"
	"net/http"

	"example.com/desk/desk"
)

func main() {
	addr := flag.String("listen-addr", ":8080", "the address the desk listens on")
	var verbose bool
	flag.BoolVar(&verbose, "verbose", false, "log every request")
	limit := flag.Int64("max-body-bytes", 1048576, "the largest request body read")
	token := flag.String("api-token", "", "the token the desk sends to the catalogue")
	flag.Parse()
	_, _, _ = verbose, limit, token
	_ = http.ListenAndServe(*addr, desk.New().Handler())
}
