package main

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

const (
	base      = "/api"
	localPath = base + "/local"
	verb      = "DELETE"
	shadowed  = "/const"
)

var twice = []string{http.MethodGet}

var fromCall = methods()

var mixed = []string{http.MethodGet, os.Args[0]}

func init() {
	twice = append(twice, http.MethodPost)
}

func methods() []string { return nil }

// other serves the second listener, through the constants and lists above.
func other() {
	r := gin.New()
	r.Handle(verb, localPath, h)
	r.Match(twice, "/twice", h)
	r.Match(fromCall, "/from-call", h)
	r.Match(mixed, "/mixed", h)
	shadowed := os.Getenv("SHADOW")
	r.GET(shadowed, h)
	r.Run(":8081")
}

func h(c *gin.Context) {}
