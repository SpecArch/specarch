package extract

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// goModuleRepository writes a Go module into a new repository and commits
// it, as the conformance runner makes its repositories. The module file
// is written here rather than kept in the repository, where it would be a
// dependency file of a module that is only read.
func goModuleRepository(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, text := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{{"init", "-q", "-b", "main"}, {"add", "."}, {"commit", "-q", "--no-gpg-sign", "-m", "The module"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1",
			"GIT_AUTHOR_NAME=SpecArch test", "GIT_AUTHOR_EMAIL=test@specarch.invalid", "GIT_AUTHOR_DATE=2026-01-01T00:00:00+0000",
			"GIT_COMMITTER_NAME=SpecArch test", "GIT_COMMITTER_EMAIL=test@specarch.invalid", "GIT_COMMITTER_DATE=2026-01-01T00:00:00+0000")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	return dir
}

// TestGoHandlerInAnotherPackage finds a handler named through an import
// of another package of the module, by go.mod's module path, and reads
// the parameters it reads and the refusals it answers.
func TestGoHandlerInAnotherPackage(t *testing.T) {
	dir := goModuleRepository(t, map[string]string{
		"go.mod": "module example.com/desk\n\ngo 1.26\n",
		"api/endpoints.go": `package api

import (
	"github.com/donnyhardyanto/dxlib/api"

	loans "example.com/desk/handlers/loans"
)

func Define(a *api.DXAPI) {
	a.NewEndPoint("Show a loan", "", "/loans/show", "POST", api.EndPointTypeHTTPJSON, 0,
		[]api.DXAPIEndPointParameter{{NameId: "id"}},
		loans.Show, nil, nil, nil, []string{"loans.read"}, 0, "")
}
`,
		"handlers/loans/show.go": `package loans

import (
	"net/http"

	"github.com/donnyhardyanto/dxlib/api"
)

func Show(aepr *api.DXAPIEndPointRequest) (err error) {
	_, _, err = aepr.GetParameterValueAsInt64("id")
	return aepr.WriteResponseAndNewErrorf(http.StatusNotFound, "LOAN_NOT_FOUND", "none")
}
`,
	})
	res, err := Go([]string{dir}, filepath.Join(t.TempDir(), "out"), "code", "", false)
	if err != nil {
		t.Fatal(err)
	}
	res.Tree.anchorBlocks()
	paths, err := encode(res.Tree.files["design/paths.yaml"])
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"clause: 'handlers/loans/show.go:9', says: The handler of POST /loans/show reads the parameter id and answers 404 LOAN_NOT_FOUND.",
		"problem: loan-not-found",
		"loan-not-found:\n    status: 404",
	} {
		if !strings.Contains(string(paths), want) {
			t.Errorf("the tree does not hold %q:\n%s", want, paths)
		}
	}
	questions, err := encode(res.Tree.files["design/questions.yaml"])
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(questions), "is not a function the reader finds") {
		t.Errorf("the handler in another package was not found:\n%s", questions)
	}
}

// TestPrivilegePermission maps dxlib_module's names by the rule of
// ADR-076 and leaves every other form unmapped.
func TestPrivilegePermission(t *testing.T) {
	for name, want := range map[string]string{
		"ORDER.CREATE":                "order.create",
		"GLOBAL.SET_MAINTENANCE_MODE": "global.set.maintenance.mode",
		"EVERYTHING":                  "everything",
		"V2_REPORT":                   "v2.report",
		"REPORT_2X":                   "",
		"PUBLIC":                      "",
		"Order.Create":                "",
		"order.create":                "",
		"ORDER..CREATE":               "",
	} {
		if got := privilegePermission(name); got != want {
			t.Errorf("%s: got %q, want %q", name, got, want)
		}
	}
}

// goRouterOperations runs extract go on a module and gives the paths
// fragment and the questions fragment as text.
func goRouterOperations(t *testing.T, files map[string]string) (string, string) {
	t.Helper()
	dir := goModuleRepository(t, files)
	res, err := Go([]string{dir}, filepath.Join(t.TempDir(), "out"), "code", "", false)
	if err != nil {
		t.Fatal(err)
	}
	res.Tree.anchorBlocks()
	paths, err := encode(res.Tree.files["design/paths.yaml"])
	if err != nil {
		t.Fatal(err)
	}
	questions, err := encode(res.Tree.files["design/questions.yaml"])
	if err != nil {
		t.Fatal(err)
	}
	return string(paths), string(questions)
}

// The server package of the two tests below: a struct another package's
// constructor returns, with a method that is given the router.
const goRestServer = `package rest

import "github.com/gin-gonic/gin"

type Server struct{ items []string }

func NewServer() *Server { return &Server{} }

func Open() (*Server, error) { return &Server{}, nil }

func (s *Server) RegisterRoutes(r *gin.Engine) {
	r.GET("/items", s.List)
	r.POST("/items", s.Add)
}

func (s *Server) List(c *gin.Context) {}
func (s *Server) Add(c *gin.Context)  {}
`

// TestGoRouterPassedToMethodOfConstructedValue follows a gin router into
// a method of a struct from another package, when the value the method is
// called on comes from a constructor, returning the value alone or with an
// error.
func TestGoRouterPassedToMethodOfConstructedValue(t *testing.T) {
	paths, questions := goRouterOperations(t, map[string]string{
		"go.mod":                      "module example.com/svc\n\ngo 1.26\n",
		"internal/api/rest/server.go": goRestServer,
		"cmd/main.go": `package main

import (
	"github.com/gin-gonic/gin"

	"example.com/svc/internal/api/rest"
)

func main() {
	r := gin.New()
	server := rest.NewServer()
	server.RegisterRoutes(r)
	_ = r.Run(":8080")
}
`,
		"cmd/other/main.go": `package main

import (
	"github.com/gin-gonic/gin"

	"example.com/svc/internal/api/rest"
)

func main() {
	r := gin.New()
	server, err := rest.Open()
	if err != nil {
		panic(err)
	}
	server.RegisterRoutes(r)
	_ = r.Run(":8081")
}
`,
	})
	for _, want := range []string{"/items:", "get:", "post:", "clause: 'internal/api/rest/server.go:12'"} {
		if !strings.Contains(paths, want) {
			t.Errorf("the paths fragment does not hold %q:\n%s", want, paths)
		}
	}
	if strings.Contains(questions, "found no call that passes it one") {
		t.Errorf("the router passed to the method was not followed:\n%s", questions)
	}
}

// TestGoRouterPassedFromClosure follows the router when the call that
// passes it sits in a closure, as a command's run function does.
func TestGoRouterPassedFromClosure(t *testing.T) {
	paths, questions := goRouterOperations(t, map[string]string{
		"go.mod":                      "module example.com/svc\n\ngo 1.26\n",
		"internal/api/rest/server.go": goRestServer,
		"cmd/start.go": `package main

import (
	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"

	"example.com/svc/internal/api/rest"
)

func startCommand() *cobra.Command {
	return &cobra.Command{
		Use: "start",
		RunE: func(cmd *cobra.Command, args []string) error {
			r := gin.New()
			server := rest.NewServer()
			server.RegisterRoutes(r)
			return r.Run(":8080")
		},
	}
}

func main() { _ = startCommand().Execute() }
`,
	})
	for _, want := range []string{"/items:", "get:", "post:"} {
		if !strings.Contains(paths, want) {
			t.Errorf("the paths fragment does not hold %q:\n%s", want, paths)
		}
	}
	if strings.Contains(questions, "found no call that passes it one") {
		t.Errorf("the router passed from the closure was not followed:\n%s", questions)
	}
}

// TestGoBodyIntoStructWrittenInPlace reads a body a handler decodes into a
// variable declared with a struct type in place, by the rules a named
// struct is read by: json tags, a nested struct in place, a pointer and a
// slice; and anchors every question's pointer in the tree it writes, so
// that a body of a type outside the module blocks the operation's missing
// requestBody, a map of any values is an open map whose values the
// question asks, blocking the written values, and an interface as a struct's only field blocks
// the object's missing properties, never a key below one the tree does
// not have.
func TestGoBodyIntoStructWrittenInPlace(t *testing.T) {
	paths, questions := goRouterOperations(t, map[string]string{
		"go.mod": "module example.com/svc\n\ngo 1.26\n",
		"cmd/main.go": `package main

import (
	"github.com/gin-gonic/gin"

	"example.com/svc"
)

func main() {
	r := gin.New()
	server := svc.NewServer()
	server.Routes(r)
	_ = r.Run(":8080")
}
`,
		"server.go": `package svc

import (
	"github.com/gin-gonic/gin"
	"example.com/other/pb"
)

type Server struct{}

func NewServer() *Server { return &Server{} }

func (s *Server) Routes(r *gin.Engine) {
	r.POST("/items", s.Create)
	r.PUT("/items", s.Replace)
	r.POST("/imports", s.Import)
	r.POST("/policies", s.Upsert)
	r.POST("/tags", s.Tag)
}

func (s *Server) Create(c *gin.Context) {
	var req struct {
		Name string ` + "`json:\"name\"`" + `
		Size *struct {
			Width int32 ` + "`json:\"width\"`" + `
		} ` + "`json:\"size\"`" + `
		Tags []struct {
			Label string ` + "`json:\"label\"`" + `
		} ` + "`json:\"tags\"`" + `
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		return
	}
}

func (s *Server) Replace(c *gin.Context) {
	var body struct {
		Name string ` + "`json:\"name\"`" + `
	}
	if err := c.BindJSON(&body); err != nil {
		return
	}
}

func (s *Server) Import(c *gin.Context) {
	var req pb.Item
	_ = c.ShouldBindJSON(&req)
}

type upsertRequest struct {
	Policy map[string]any ` + "`json:\"policy\"`" + `
}

func (s *Server) Upsert(c *gin.Context) {
	var req upsertRequest
	_ = c.ShouldBindJSON(&req)
}

func (s *Server) Tag(c *gin.Context) {
	var req struct {
		Data any ` + "`json:\"data\"`" + `
	}
	_ = c.ShouldBindJSON(&req)
}
`,
	})
	if strings.Count(paths, "name:\n                  type: string") != 2 {
		t.Errorf("want the property name on POST /items and PUT /items:\n%s", paths)
	}
	for _, want := range []string{
		"size:\n                  type: [object, \"null\"]\n                  properties:\n                    width:\n                      type: integer\n                      format: int32",
		"tags:\n                  type: array\n                  items:\n                    type: object\n                    properties:\n                      label:\n                        type: string",
	} {
		if strings.Count(paths, want) != 1 {
			t.Errorf("the paths fragment does not hold %q once:\n%s", want, paths)
		}
	}
	if strings.Count(paths, "requestBody:") != 4 {
		t.Errorf("want a requestBody on POST /items, PUT /items, POST /policies and POST /tags:\n%s", paths)
	}
	if want := "policy:\n                  type: object\n                  additionalProperties: true\n"; !strings.Contains(paths, want) {
		t.Errorf("the map property is not written as an open map:\n%s", paths)
	}
	for _, want := range []string{
		"blocks: ['#/paths/~1imports/post/requestBody']",
		"blocks: ['#/paths/~1policies/post/requestBody/content/application~1json/schema/properties/policy/additionalProperties']",
		"blocks: ['#/paths/~1tags/post/requestBody/content/application~1json/schema/properties']",
		"it is a map, map[string]any, whose values may be any JSON value",
	} {
		if !strings.Contains(questions, want) {
			t.Errorf("the questions fragment does not hold %q:\n%s", want, questions)
		}
	}
	if strings.Contains(questions, "/requestBody/content/application~1json/schema'") {
		t.Errorf("a question blocks a schema the tree does not have:\n%s", questions)
	}
}

// TestGoRouterThroughLocalHelper follows a route registered in a callback
// handed to a helper that takes a prefix and a function: the helper a
// local closure, and the helper a function of the module. The call binds
// the string and the function as it binds a router, so the group's prefix
// is known and the callback is walked with the group.
func TestGoRouterThroughLocalHelper(t *testing.T) {
	for name, routes := range map[string]string{
		"closure": `package svc

import "github.com/gin-gonic/gin"

func Routes(r *gin.Engine) {
	v1 := r.Group("/v1")
	v1.GET("/things", listThings)

	group := func(prefix string, reg func(g *gin.RouterGroup)) {
		g := v1.Group(prefix)
		reg(g)
	}
	group("/callbacks", func(g *gin.RouterGroup) {
		g.POST("/thing-create", createThing)
	})
	group("/events", registerEvents)
}

func registerEvents(g *gin.RouterGroup) { g.POST("/thing-closed", closeThing) }

func listThings(c *gin.Context)  {}
func createThing(c *gin.Context) {}
func closeThing(c *gin.Context)  {}
`,
		"function": `package svc

import "github.com/gin-gonic/gin"

func Routes(r *gin.Engine) {
	v1 := r.Group("/v1")
	v1.GET("/things", listThings)
	group(v1, "/callbacks", func(g *gin.RouterGroup) {
		g.POST("/thing-create", createThing)
	})
	group(v1, "/events", registerEvents)
}

func group(parent *gin.RouterGroup, prefix string, reg func(g *gin.RouterGroup)) {
	reg(parent.Group(prefix))
}

func registerEvents(g *gin.RouterGroup) { g.POST("/thing-closed", closeThing) }

func listThings(c *gin.Context)  {}
func createThing(c *gin.Context) {}
func closeThing(c *gin.Context)  {}
`,
	} {
		t.Run(name, func(t *testing.T) {
			paths, questions := goRouterOperations(t, map[string]string{
				"go.mod":    "module example.com/svc\n\ngo 1.26\n",
				"routes.go": routes,
				"cmd/main.go": `package main

import (
	"github.com/gin-gonic/gin"

	"example.com/svc"
)

func main() {
	r := gin.New()
	svc.Routes(r)
	_ = r.Run(":8080")
}
`,
			})
			for _, want := range []string{"/v1/things:", "/v1/callbacks/thing-create:", "/v1/events/thing-closed:"} {
				if strings.Count(paths, want) != 1 {
					t.Errorf("the paths fragment does not hold %q once:\n%s", want, paths)
				}
			}
			if strings.Contains(questions, "whose place the reader does not know") {
				t.Errorf("a route registered through the helper was not placed:\n%s", questions)
			}
		})
	}
}

// TestGoLiteralSeesWhereItIsWritten holds the reader to walking a
// function literal in the scope it is written in: one passed to a
// function of the module sees the routers around it, not the callee's,
// and a helper declared again in an inner block holds only to the end of
// that block. A helper walked once per string its callers pass, where the
// string does not change the route, counts the registration once.
func TestGoLiteralSeesWhereItIsWritten(t *testing.T) {
	goRoutesCases(t, map[string]goRoutesCase{
		"callee has a router of the same name": {code: `
func Routes(r *gin.Engine) {
	api := r.Group("/api")
	apply(r, func() { api.GET("/x", h) })
}

func apply(r *gin.Engine, f func()) {
	api := r.Group("/other")
	_ = api
	f()
}
`, want: []string{"/api/x:"}, notWant: []string{"/other/x:"}, counted: 1, wrote: 1},
		"callee has no router of that name": {code: `
func Routes(r *gin.Engine) {
	api := r.Group("/api")
	apply(r, func() { api.GET("/x", h) })
}

func apply(r *gin.Engine, f func()) { f() }
`, want: []string{"/api/x:"}, counted: 1, wrote: 1},
		"helper declared again in an inner block": {code: `
func Routes(r *gin.Engine) {
	v1, v2 := r.Group("/v1"), r.Group("/v2")
	group := func(p string, reg func(*gin.RouterGroup)) { reg(v1.Group(p)) }
	{
		group := func(p string, reg func(*gin.RouterGroup)) { reg(v2.Group(p)) }
		group("/in", func(g *gin.RouterGroup) { g.GET("/x", h) })
	}
	group("/out", func(g *gin.RouterGroup) { g.GET("/y", h) })
}
`, want: []string{"/v2/in/x:", "/v1/out/y:"}, notWant: []string{"/v2/out/y:"}, counted: 2, wrote: 2},
		"string the route does not use": {code: `
func Routes(r *gin.Engine) {
	reg(r, "a")
	reg(r, "b")
}

func reg(r *gin.Engine, name string) { r.GET("/health", h) }
`, want: []string{"/health:"}, counted: 1, wrote: 1},
		"callee calls it behind a condition": {code: `
func Routes(r *gin.Engine) {
	api := r.Group("/api")
	apply(api, func(g *gin.RouterGroup) { g.GET("/x", h) })
}

func apply(g *gin.RouterGroup, f func(*gin.RouterGroup)) {
	if os.Getenv("X") != "" {
		f(g)
	}
}
`, notWant: []string{"/api/x:"}, counted: 1, asked: 1},
		"called twice, once behind a condition": {code: `
func Routes(r *gin.Engine) {
	api := r.Group("/api")
	add := func(g *gin.RouterGroup) { g.GET("/x", h) }
	add(api)
	if os.Getenv("X") != "" {
		add(api)
	}
}
`, want: []string{"/api/x:"}, counted: 2, wrote: 1, asked: 1},
		"helper walked per string, a route behind a condition": {code: `
func Routes(r *gin.Engine) {
	reg(r, "a")
	reg(r, "b")
}

func reg(r *gin.Engine, name string) {
	if name == "a" {
		r.GET("/only", h)
	}
	r.GET("/health", h)
}
`, want: []string{"/health:"}, notWant: []string{"/only:"}, counted: 2, wrote: 1, asked: 1},
		"helper called only from a literal nobody calls": {code: `
var hooks []func()

func Routes(r *gin.Engine) {
	api := r.Group("/api")
	add := func(g *gin.RouterGroup) { g.GET("/x", h) }
	hook := func() { add(api) }
	hooks = append(hooks, hook)
}
`, want: []string{"/api/x:"}, counted: 1, wrote: 1},
		"router declared again inside a passed literal": {code: `
func Routes(r *gin.Engine) {
	api := r.Group("/api")
	apply(func() {
		api := r.Group("/other")
		api.GET("/a", h)
	})
	api.GET("/b", h)
}

func apply(f func()) { f() }
`, want: []string{"/other/a:", "/api/b:"}, notWant: []string{"/other/b:"}, counted: 2, wrote: 2},
		"helper declared again in an if statement's init": {code: `
func Routes(r *gin.Engine) {
	a := r.Group("/a")
	reg := func() { a.GET("/outer", h) }
	if reg := func() { a.GET("/inner", h) }; os.Getenv("X") != "" {
		_ = reg
	}
	reg()
}
`, want: []string{"/a/outer:"}, notWant: []string{"/a/inner:"}, counted: 2, wrote: 1, asked: 1},
	})
}

// TestGoRouterReturnedByHelper follows a route registered on the group a
// helper returns: the helper a local closure called as a value, assigned
// in a block or chained, called with two prefixes, registering a route
// of its own before it returns, or passed to a function of the module
// that calls it for the group; and a function of the module, called by
// name or passed as a value. A literal's return is its call's value and
// never the return of the function it is written in.
func TestGoRouterReturnedByHelper(t *testing.T) {
	goRoutesCases(t, map[string]goRoutesCase{
		"closure returning the group, assigned in a block": {code: `
func Routes(r *gin.Engine) {
	v1 := r.Group("/v1")
	returned := func(p string) *gin.RouterGroup {
		g := v1.Group(p)
		return g
	}
	{
		g := returned("/d")
		g.POST("/x", h)
		g.PUT("/:id/y", h)
	}
}
`, want: []string{"/v1/d/x:", "/v1/d/{id}/y:"}, counted: 2, wrote: 2},
		"closure called with two prefixes, and in a chain": {code: `
func Routes(r *gin.Engine) {
	v1 := r.Group("/v1")
	under := func(p string) *gin.RouterGroup { return v1.Group(p) }
	a := under("/a")
	a.GET("/x", h)
	b := under("/b")
	b.GET("/y", h)
	under("/c").GET("/z", h)
}
`, want: []string{"/v1/a/x:", "/v1/b/y:", "/v1/c/z:"}, notWant: []string{"/v1/a/y:", "/v1/b/x:"}, counted: 3, wrote: 3},
		"closure registering a route before it returns the group": {code: `
func Routes(r *gin.Engine) {
	v1 := r.Group("/v1")
	mk := func(p string) *gin.RouterGroup {
		g := v1.Group(p)
		g.GET("/ping", h)
		return g
	}
	g := mk("/e")
	g.GET("/x", h)
}
`, want: []string{"/v1/e/ping:", "/v1/e/x:"}, counted: 2, wrote: 2},
		"closure passed to a function of the module that calls it for the group": {code: `
func Routes(r *gin.Engine) {
	v1 := r.Group("/v1")
	mk := func(p string) *gin.RouterGroup { return v1.Group(p) }
	apply(mk)
}

func apply(mk func(string) *gin.RouterGroup) {
	g := mk("/g")
	g.GET("/x", h)
}
`, want: []string{"/v1/g/x:"}, counted: 1, wrote: 1},
		"function of the module returning the group": {code: `
func Routes(r *gin.Engine) {
	v1 := r.Group("/v1")
	g := sub(v1, "/f")
	g.GET("/x", h)
}

func sub(parent *gin.RouterGroup, p string) *gin.RouterGroup { return parent.Group(p) }
`, want: []string{"/v1/f/x:"}, counted: 1, wrote: 1},
		"function of the module passed as a value and called for the group": {code: `
func Routes(r *gin.Engine) {
	v1 := r.Group("/v1")
	mount(v1, sub)
}

func mount(parent *gin.RouterGroup, mk func(*gin.RouterGroup, string) *gin.RouterGroup) {
	g := mk(parent, "/h")
	g.GET("/x", h)
}

func sub(parent *gin.RouterGroup, p string) *gin.RouterGroup { return parent.Group(p) }
`, want: []string{"/v1/h/x:"}, counted: 1, wrote: 1},
		"a literal's return is not the return of the function around it": {code: `
func Routes(r *gin.Engine) {
	api := newAPI(r)
	api.GET("/b", h)
}

func newAPI(r *gin.Engine) *gin.RouterGroup {
	api := r.Group("/api")
	add := func(p string) *gin.RouterGroup { return api.Group(p) }
	add("/x").GET("/a", h)
	return api
}
`, want: []string{"/api/x/a:", "/api/b:"}, notWant: []string{"/api/x/b:"}, counted: 2, wrote: 2},
	})
}

// goRoutesCase is one module whose routes.go holds code around a Routes
// function, and what the reader must write, count and ask of it.
type goRoutesCase struct {
	code                  string
	want, notWant         []string
	counted, wrote, asked int
}

// goRoutesCases reads each case's module, its code in routes.go with a
// handler h and a main that hands Routes a gin engine, and checks that
// the paths fragment holds each wanted path once and no unwanted one,
// and that every registration counted is written or asked once.
func goRoutesCases(t *testing.T, cases map[string]goRoutesCase) {
	t.Helper()
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			dir := goModuleRepository(t, map[string]string{
				"go.mod":    "module example.com/svc\n\ngo 1.26\n",
				"routes.go": "package svc\n\nimport (\n\t\"os\"\n\n\t\"github.com/gin-gonic/gin\"\n)\n\nvar _ = os.Getenv\n" + c.code + "\nfunc h(c *gin.Context) {}\n",
				"cmd/main.go": `package main

import (
	"github.com/gin-gonic/gin"

	"example.com/svc"
)

func main() {
	r := gin.New()
	svc.Routes(r)
	_ = r.Run(":8080")
}
`,
			})
			res, err := Go([]string{dir}, filepath.Join(t.TempDir(), "out"), "code", "", false)
			if err != nil {
				t.Fatal(err)
			}
			res.Tree.anchorBlocks()
			paths, err := encode(res.Tree.files["design/paths.yaml"])
			if err != nil {
				t.Fatal(err)
			}
			for _, want := range c.want {
				if strings.Count(string(paths), want) != 1 {
					t.Errorf("the paths fragment does not hold %q once:\n%s", want, paths)
				}
			}
			for _, not := range c.notWant {
				if strings.Contains(string(paths), not) {
					t.Errorf("the paths fragment holds %q:\n%s", not, paths)
				}
			}
			counted, written := 0, 0
			for _, line := range res.Lines {
				fmt.Sscanf(line, "routers: counted %d", &counted)
				fmt.Sscanf(line, "routes: wrote %d", &written)
			}
			questions, err := encode(res.Tree.files["design/questions.yaml"])
			if err != nil {
				t.Fatal(err)
			}
			asked := len(regexp.MustCompile(`(?m)^    question: .* registers `).FindAll(questions, -1))
			if counted != c.counted || written != c.wrote || asked != c.asked || counted != written+asked {
				t.Errorf("counted %d registrations, wrote %d operations and asked %d questions, want %d, %d and %d, every registration written or asked once\n%s\n%s", counted, written, asked, c.counted, c.wrote, c.asked, strings.Join(res.Lines, "\n"), questions)
			}
		})
	}
}

// TestGoRegistrationsAccountedFor holds the reader to the rule that a
// counted registration is never dropped unsaid: each becomes an
// operation, a question or a not-held line. The module registers routes
// the reader places, routes it cannot place (a prefix that is not a
// literal, a router from nowhere), a catch-all it asks about, and
// routes in a loop and behind a condition.
func TestGoRegistrationsAccountedFor(t *testing.T) {
	dir := goModuleRepository(t, map[string]string{
		"go.mod": "module example.com/svc\n\ngo 1.26\n",
		"cmd/main.go": `package main

import (
	"os"

	"github.com/gin-gonic/gin"
)

func main() {
	r := gin.New()
	r.GET("/things", list)
	r.GET("/files/*path", list)
	under(os.Getenv("PREFIX"), func(g *gin.RouterGroup) { g.POST("/hidden", list) })
	for _, p := range []string{"/a", "/b"} {
		r.GET(p, list)
	}
	if os.Getenv("DEBUG") != "" {
		r.GET("/debug", list)
	}
	r.Any("/every", list)
	_ = r.Run(":8080")
}

func under(prefix string, reg func(g *gin.RouterGroup)) {}

func stray(g *gin.RouterGroup) { g.DELETE("/stray", list) }

func list(c *gin.Context) {}
`,
	})
	res, err := Go([]string{dir}, filepath.Join(t.TempDir(), "out"), "code", "", false)
	if err != nil {
		t.Fatal(err)
	}
	var counted, written, asked, notHeld int
	for _, line := range res.Lines {
		switch {
		case strings.HasPrefix(line, "routers: counted "):
			fmt.Sscanf(line, "routers: counted %d", &counted)
		case strings.HasPrefix(line, "routes: wrote "):
			fmt.Sscanf(line, "routes: wrote %d", &written)
		case strings.HasPrefix(line, "not held: "):
			notHeld++
		}
	}
	res.Tree.anchorBlocks()
	questions, err := encode(res.Tree.files["design/questions.yaml"])
	if err != nil {
		t.Fatal(err)
	}
	for _, said := range []string{"registers POST /hidden on a router whose place the reader does not know", "registers DELETE /stray on a router whose place", "registers a route in a loop", "registers a route behind a condition", "registers /every for every method", "registers GET under /files with the rest of the path matched by *path"} {
		if strings.Contains(string(questions), said) {
			asked++
		} else {
			t.Errorf("no question says %q:\n%s", said, questions)
		}
	}
	if counted != 7 || written != 1 || notHeld != 0 || counted != written+asked+notHeld {
		t.Errorf("counted %d registrations, wrote %d operations, asked %d questions and printed %d not-held lines; every registration is one of the three\n%s", counted, written, asked, notHeld, strings.Join(res.Lines, "\n"))
	}
}

// TestGoTypedMaps checks that a map is written as an object whose
// additionalProperties is the schema of its values, whatever its key type,
// since encoding/json writes every key as text; that a map of maps, of
// lists, of structs and of pointers keeps its value type; and that a value
// type the meta-model has no type for leaves the map open, with the
// question at its values.
func TestGoTypedMaps(t *testing.T) {
	paths, questions := goRouterOperations(t, map[string]string{
		"go.mod": "module example.com/svc\n\ngo 1.26\n",
		"cmd/main.go": `package main

import (
	"github.com/gin-gonic/gin"

	"example.com/svc"
)

func main() {
	r := gin.New()
	server := svc.NewServer()
	server.Routes(r)
	_ = r.Run(":8080")
}
`,
		"server.go": `package svc

import "github.com/gin-gonic/gin"

type Server struct{}

func NewServer() *Server { return &Server{} }

func (s *Server) Routes(r *gin.Engine) {
	r.POST("/settings", s.Save)
}

type Code string

type Label struct {
	Text string ` + "`json:\"text\"`" + `
}

type saveRequest struct {
	Counts  map[string]int32            ` + "`json:\"counts\"`" + `
	ByID    map[int32]Label             ` + "`json:\"byId\"`" + `
	ByCode  map[Code]*Label             ` + "`json:\"byCode\"`" + `
	Groups  map[string][]string         ` + "`json:\"groups\"`" + `
	Nested  map[string]map[string]bool  ` + "`json:\"nested\"`" + `
	Weights map[string]float32          ` + "`json:\"weights\"`" + `
}

func (s *Server) Save(c *gin.Context) {
	var req saveRequest
	_ = c.ShouldBindJSON(&req)
}
`,
	})
	for _, want := range []string{
		"counts:\n                  type: object\n                  additionalProperties:\n                    type: integer\n                    format: int32\n",
		"byId:\n                  type: object\n                  additionalProperties:\n                    type: object\n                    properties:\n                      text:\n                        type: string\n",
		"byCode:\n                  type: object\n                  additionalProperties:\n                    type: [object, \"null\"]\n",
		"groups:\n                  type: object\n                  additionalProperties:\n                    type: array\n                    items:\n                      type: string\n",
		"nested:\n                  type: object\n                  additionalProperties:\n                    type: object\n                    additionalProperties:\n                      type: boolean\n",
		"weights:\n                  type: object\n                  additionalProperties: true\n",
	} {
		if strings.Count(paths, want) != 1 {
			t.Errorf("the paths fragment does not hold %q once:\n%s", want, paths)
		}
	}
	if !strings.Contains(questions, "blocks: ['#/paths/~1settings/post/requestBody/content/application~1json/schema/properties/weights/additionalProperties']") ||
		!strings.Contains(questions, "property weights, a value of the map: it is Go''s float32") {
		t.Errorf("want the question on the float32 values, blocking the values:\n%s", questions)
	}
	if strings.Contains(questions, "it is a map") {
		t.Errorf("a typed map is asked about:\n%s", questions)
	}
}

// TestGoConstantsAcrossPackages reads a path from a constant of another
// package of the module through its import, one joined from it by +, a
// method list of that package assigned once, a list spread into
// gorilla/mux's Methods, and a method from a constant (ADR-143); and keeps
// the question where a local of the function hides a constant.
func TestGoConstantsAcrossPackages(t *testing.T) {
	paths, questions := goRouterOperations(t, map[string]string{
		"go.mod": "module example.com/gateway\n\ngo 1.26\n",
		"paths/paths.go": `package paths

import "net/http"

const (
	Base  = "/api"
	Users = Base + "/users"
	Items = "/items"
	Read  = "GET"
)

var Methods = []string{http.MethodGet, http.MethodHead}
`,
		"cmd/gin/main.go": `package main

import (
	"os"

	"github.com/gin-gonic/gin"

	"example.com/gateway/paths"
)

const localPath = paths.Base + "/local"

func main() {
	r := gin.New()
	r.GET(paths.Users, h)
	r.Handle(paths.Read, paths.Items, h)
	r.Match(paths.Methods, localPath, h)
	localPath := os.Getenv("LOCAL")
	r.POST(localPath, h)
	r.Run(":8080")
}

func h(c *gin.Context) {}
`,
		"cmd/mux/main.go": `package main

import (
	"net/http"

	"github.com/gorilla/mux"

	"example.com/gateway/paths"
)

func main() {
	r := mux.NewRouter()
	r.HandleFunc(paths.Items, h).Methods(paths.Methods...)
	one := r.PathPrefix("/v1").Methods(paths.Read).Subrouter()
	one.HandleFunc("/sub", h)
	two := r.PathPrefix("/v2").Methods(paths.Methods...).Subrouter()
	two.HandleFunc("/two", h)
	http.ListenAndServe(":8081", r)
}

func h(w http.ResponseWriter, r *http.Request) {}
`,
	})
	for _, want := range []string{"/api/users:\n    get:", "/items:\n    get:", "/api/local:\n    get:", "/v1/sub:\n    get:", "/v2/two:\n    get:"} {
		if !strings.Contains(paths, want) {
			t.Errorf("the tree does not hold %q:\n%s", want, paths)
		}
	}
	// paths.Methods is GET and HEAD; HEAD is a could question, as the
	// meta-model does not hold it, so the list read gives gin's
	// /api/local, mux's /items and the subrouter's /v2/two their GET and
	// three HEAD questions.
	if n := len(regexp.MustCompile(`(?m)^    question: .*the route HEAD `).FindAllString(questions, -1)); n != 3 {
		t.Errorf("the questions name %d HEAD routes, want 3 (/api/local, /items and /v2/two):\n%s", n, questions)
	}
	if strings.Contains(questions, "registers /sub for every method") || strings.Contains(questions, "registers /two for every method") || strings.Contains(questions, "not a literal method") {
		t.Errorf("a subrouter's Methods did not read the constant or the list:\n%s", questions)
	}
	if !strings.Contains(questions, "registers a route whose path localPath the reader does not read as a literal") {
		t.Errorf("the local that hides the constant did not keep the question:\n%s", questions)
	}
	for _, said := range []string{"paths.Users", "paths.Items", "paths.Methods", "paths.Read"} {
		if strings.Contains(questions, "whose path "+said) || strings.Contains(questions, "on methods "+said) || strings.Contains(questions, "method "+said) {
			t.Errorf("a question was asked about %s:\n%s", said, questions)
		}
	}
}

// TestGoListChangedByAnotherPackage keeps the question for a package-level
// list another package of the module assigns to and appends to through
// its import, naming that line (ADR-143): a list is read as a literal
// only when no statement of the module changes it.
func TestGoListChangedByAnotherPackage(t *testing.T) {
	paths, questions := goRouterOperations(t, map[string]string{
		"go.mod": "module example.com/gateway\n\ngo 1.26\n",
		"paths/paths.go": `package paths

import "net/http"

var Methods = []string{http.MethodGet}
`,
		"cmd/mux/main.go": `package main

import (
	"net/http"

	"github.com/gorilla/mux"

	"example.com/gateway/paths"
)

func main() {
	paths.Methods = append(paths.Methods, http.MethodPost)
	r := mux.NewRouter()
	r.HandleFunc("/items", h).Methods(paths.Methods...)
	http.ListenAndServe(":8081", r)
}

func h(w http.ResponseWriter, r *http.Request) {}
`,
	})
	if strings.Contains(paths, "/items:") {
		t.Errorf("the route on a list another package appends to was written:\n%s", paths)
	}
	if !strings.Contains(questions, "Methods on paths.Methods, which is a package-level list assigned again at cmd/mux/main.go:12") {
		t.Errorf("the question does not name the append and its line:\n%s", questions)
	}
}
