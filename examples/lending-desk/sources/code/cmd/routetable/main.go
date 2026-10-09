// Command routetable prints the routes the lending desk registers, as the
// JSON array tools/routes/dump-routes.sh takes. It hands Server.Register a
// recorder in place of the ServeMux the service runs on.
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"reflect"
	"runtime"
	"strings"

	"example.com/lendingdesk/lending"
)

type route struct {
	Method     string  `json:"method"`
	Path       string  `json:"path"`
	Permission *string `json:"permission"`
	Handler    string  `json:"handler"`
}

// recorder keeps every pattern registered on it, with the handler.
type recorder struct {
	routes []route
}

// Handle records a pattern "METHOD /path", as http.ServeMux takes it, and
// the permission a lending.Checked handler checks.
func (rec *recorder) Handle(pattern string, h http.Handler) {
	method, path, _ := strings.Cut(pattern, " ")
	out := route{Method: method, Path: path}
	switch h := h.(type) {
	case lending.Checked:
		p := h.Permission
		out.Permission = &p
		out.Handler = handlerName(h.Handler)
	default:
		out.Handler = handlerName(h)
	}
	rec.routes = append(rec.routes, out)
}

func main() {
	rec := &recorder{}
	lending.NewServer(nil).Register(rec)
	var lines []string
	for _, r := range rec.routes {
		b, err := json.Marshal(r)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		lines = append(lines, "    "+string(b))
	}
	fmt.Printf("[\n%s\n]\n", strings.Join(lines, ",\n"))
}

// handlerName is the name of the function that answers a route, without
// its package, its receiver or the suffix of a method value.
func handlerName(h any) string {
	name := runtime.FuncForPC(reflect.ValueOf(h).Pointer()).Name()
	name = strings.TrimSuffix(name, "-fm")
	return name[strings.LastIndex(name, ".")+1:]
}
