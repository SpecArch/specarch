// Command openapi writes the notice board's OpenAPI document, as dxlib
// emits it from the endpoints the service defines, to standard output.
// The service has no DXApp of its own to run under DXLIB_OPENAPI_DUMP, so
// this program defines the API the way the service does and prints it.
package main

import (
	"fmt"
	"os"

	"github.com/donnyhardyanto/dxlib/api"

	"example.com/noticeboard/service"
)

func main() {
	a, err := api.Manager.NewAPI("api")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	service.DefineEndPoints(a)
	doc, err := a.OpenAPIAsJSON()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Stdout.Write(doc)
}
