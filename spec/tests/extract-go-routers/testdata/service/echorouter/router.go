package echorouter

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

// Server serves the fines on echo.
func Server() *echo.Echo {
	e := echo.New()
	g := e.Group("/fines")
	g.GET("/:fineId", showFine)
	g.Add(http.MethodPost, "/:fineId/waivers", waive)
	e.Match([]string{"PUT", "PATCH"}, "/fines/:fineId", waive)
	return e
}

func showFine(c echo.Context) error { _ = c.Param("fine"); return nil }
func waive(c echo.Context) error    { return nil }
