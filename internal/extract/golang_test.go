package extract

import (
	"os"
	"os/exec"
	"path/filepath"
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
	res, err := Go([]string{dir}, filepath.Join(t.TempDir(), "out"), "code", "")
	if err != nil {
		t.Fatal(err)
	}
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
	res, err := Go([]string{dir}, filepath.Join(t.TempDir(), "out"), "code", "")
	if err != nil {
		t.Fatal(err)
	}
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
