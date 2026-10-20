package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

const (
	healthLivePath  = "/live"
	healthReadyPath = "/ready"
	metricsPath     = "/metrics"
)

var healthMethods = []string{http.MethodGet, http.MethodHead}

func main() {
	r := gin.New()
	r.Match(healthMethods, healthLivePath, live)
	r.Match([]string{"GET", "HEAD"}, healthReadyPath, ready)
	r.Match([]string{"GET", "HEAD"}, "/started", started)
	r.GET(metricsPath, metrics)
	r.GET("/version", version)
	r.NoRoute(proxy)
	r.Run(":8080")
}

func live(c *gin.Context)    { c.Status(204) }
func ready(c *gin.Context)   { c.Status(204) }
func started(c *gin.Context) { c.Status(204) }
func metrics(c *gin.Context) { c.String(200, "") }
func version(c *gin.Context) { c.String(200, "") }

// proxy forwards every request no route answers to the service the
// token's audience names.
func proxy(c *gin.Context) {
	c.Status(502)
}
