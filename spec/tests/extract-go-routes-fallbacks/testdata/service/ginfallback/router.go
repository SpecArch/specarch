package ginfallback

import "github.com/gin-gonic/gin"

// Engine serves the health routes itself and forwards the rest.
func Engine() *gin.Engine {
	e := gin.New()
	e.GET("/health", health)
	e.Any("/every", health)
	files := e.Group("/files")
	files.Any("/*path", serve)
	e.Match([]string{"GET", "HEAD"}, "/static/*filepath", serve)
	e.NoRoute(proxy)
	e.NoMethod(refuse)
	return e
}

func health(c *gin.Context) { c.Status(204) }
func serve(c *gin.Context)  { c.Status(200) }
func proxy(c *gin.Context)  { c.Status(502) }
func refuse(c *gin.Context) { c.Status(405) }
