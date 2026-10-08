// Command routetable prints the routes the lending desk's router registers,
// as the JSON array tools/routes/dump-routes.sh takes.
package main

import (
	"encoding/json"
	"fmt"
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

func main() {
	var lines []string
	for _, r := range (&lending.Server{}).Routes() {
		out := route{Method: r.Method, Path: r.Path, Handler: handlerName(r.Handler)}
		if r.Permission != "" {
			p := r.Permission
			out.Permission = &p
		}
		b, err := json.Marshal(out)
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
