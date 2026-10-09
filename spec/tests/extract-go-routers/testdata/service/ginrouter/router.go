package ginrouter

import "github.com/gin-gonic/gin"

// Engine serves the catalogue on gin.
func Engine() *gin.Engine {
	e := gin.Default()
	v1 := e.Group("/v1")
	books := v1.Group("/books")
	books.GET("/:isbn", showBook)
	books.POST("", addBook)
	books.Any("/search", showBook)
	e.Handle("PATCH", "/v1/books/:isbn", addBook)
	e.GET("/static/*filepath", showBook)
	return e
}

func showBook(c *gin.Context) { _ = c.Param("isbn") }
func addBook(c *gin.Context)  {}
