package genvuehtm

import (
	"encoding/json"
	"io/fs"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/SpecArch/specarch/internal/genscreens"
	"github.com/SpecArch/specarch/internal/genui"
	"github.com/SpecArch/specarch/internal/source"
	"github.com/SpecArch/specarch/internal/spec"
	"github.com/SpecArch/specarch/internal/validate"
)

// The conformance case whose project these tests generate: the TypeScript
// case's specification with a JavaScript implementation file on vue-htm.
const caseProject = "../../spec/tests/generate-ui-vue-htm/project"

// request builds the request specarch would send for the case's ui target,
// after a change to its specification and its implementation file's
// content, with the files existing in the output folder.
func request(t *testing.T, existing []genui.File, change func(specification, content map[string]any)) *Request {
	t.Helper()
	s := spec.Load(caseProject)
	if len(s.Problems) > 0 {
		t.Fatalf("the case does not load: %v", s.Problems)
	}
	impl := s.Implementations[0]
	root := source.Parse(impl.Data).Root
	var idioms []map[string]any
	for _, u := range validate.IdiomUses(impl.Path, root, s) {
		i := map[string]any{"name": u.Name, "version": u.Version, "as": u.As}
		if u.As == "shipped" {
			i["content"] = source.ValueOf(validate.ShippedIdioms()[u.Name].Root)
		}
		idioms = append(idioms, i)
	}
	content := source.ValueOf(root).(map[string]any)
	specification := s.Value.(map[string]any)
	if change != nil {
		change(specification, content)
	}
	if existing == nil {
		existing = []genui.File{}
	}
	data, err := json.Marshal(map[string]any{"specarch": "1.0.0", "target": "ui", "root": filepath.ToSlash(s.RootFile),
		"specification": specification, "output": caseProject + "/../web", "existing": existing,
		"implementations": []any{map[string]any{"file": impl.Path, "content": content, "idioms": idioms}}})
	if err != nil {
		t.Fatal(err)
	}
	r, err := Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// The translation the case's output folder holds, which the settings name.
func translation(t *testing.T) genui.File {
	t.Helper()
	data, err := fs.ReadFile(fsOf(caseProject+"/../web"), "strings.en-GB.json")
	if err != nil {
		t.Fatal(err)
	}
	return genui.File{Path: "strings.en-GB.json", Content: string(data)}
}

// The files the case's output folder holds before a run: the translation
// and the vendored files the case commits, stand-ins for the real
// libraries, whose integrity the implementation file names.
func vendoredFiles(t *testing.T) []genui.File {
	t.Helper()
	out := []genui.File{translation(t)}
	for _, name := range []string{"vue.runtime.global.prod.js", "htm.umd.js"} {
		data, err := fs.ReadFile(fsOf(caseProject+"/../web/vendor"), name)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, genui.File{Path: "vendor/" + name, Content: string(data)})
	}
	return out
}

func fsOf(dir string) fs.FS { return dirFS(dir) }

// settings is the ui target's settings of a request's implementation file.
func settings(content map[string]any) map[string]any {
	return content["targets"].(map[string]any)["ui"].(map[string]any)["settings"].(map[string]any)
}

func libraries(content map[string]any) map[string]any {
	return content["libraries"].(map[string]any)
}

func fileOf(t *testing.T, files []genui.File, path string) string {
	t.Helper()
	for _, f := range files {
		if f.Path == path {
			return f.Content
		}
	}
	t.Fatalf("no %s among the files written: %d files", path, len(files))
	return ""
}

func messages(diags []genui.Diagnostic, severity string) []string {
	var out []string
	for _, d := range diags {
		if d.Severity == severity {
			out = append(out, d.Message)
		}
	}
	return out
}

func containing(ss []string, text string) bool {
	for _, s := range ss {
		if strings.Contains(s, text) {
			return true
		}
	}
	return false
}

// Every key of the screens' own texts a component names has a text, so no
// screen shows a key in place of words.
func TestScreenStringsCoverComponents(t *testing.T) {
	used := regexp.MustCompile(`"(screens\.[A-Za-z.]+)"`)
	err := fs.WalkDir(screens, "screens", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, _ := screens.ReadFile(path)
		for _, m := range used.FindAllStringSubmatch(string(data), -1) {
			if m[1] == "screens.notice" {
				continue // the key Notice keeps a message under in the session's storage, not a text
			}
			if _, ok := genscreens.ScreenStrings[m[1]]; ok {
				continue
			}
			if _, ok := vueStrings[m[1]]; !ok {
				t.Errorf("%s names %s, which has no text in ScreenStrings or vueStrings", path, m[1])
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for k := range vueStrings {
		if _, ok := genscreens.ScreenStrings[k]; ok {
			t.Errorf("%s is in both ScreenStrings and vueStrings", k)
		}
	}
}

// The generated files load under a content security policy with neither
// unsafe-inline nor unsafe-eval: the document has no inline script, no
// inline style and no event attribute, and every script it loads from the
// vendored files carries its integrity; no JavaScript file evaluates text
// as code or writes markup as text.
func TestGeneratedFilesAreCSPSafe(t *testing.T) {
	out := Generate(request(t, vendoredFiles(t), nil))
	if errs := messages(out.Diagnostics, "error"); len(errs) > 0 {
		t.Fatalf("the case does not generate: %v", errs)
	}
	if len(out.Files) < 40 {
		t.Fatalf("%d files written, which is too few", len(out.Files))
	}
	inlineScript := regexp.MustCompile(`<script(?:\s[^>]*)?>`)
	eventAttribute := regexp.MustCompile(`(?i)\s+on[a-z]+\s*=`)
	for _, f := range out.Files {
		switch {
		case strings.HasSuffix(f.Path, ".html"):
			for _, tag := range inlineScript.FindAllString(f.Content, -1) {
				if !strings.Contains(tag, " src=") {
					t.Errorf("%s has an inline script: %s", f.Path, tag)
				}
				if strings.Contains(tag, "vendor/") && !strings.Contains(tag, " integrity=\"sha384-") {
					t.Errorf("%s loads a vendored file without its integrity: %s", f.Path, tag)
				}
			}
			if strings.Contains(f.Content, "<style") || strings.Contains(f.Content, " style=") {
				t.Errorf("%s has an inline style", f.Path)
			}
			if m := eventAttribute.FindString(f.Content); m != "" {
				t.Errorf("%s has an event attribute: %q", f.Path, m)
			}
		case strings.HasSuffix(f.Path, ".js"):
			for _, forbidden := range []string{"eval(", "new Function", "innerHTML", "outerHTML", "document.write", "javascript:", "insertAdjacentHTML"} {
				if strings.Contains(f.Content, forbidden) {
					t.Errorf("%s uses %s", f.Path, forbidden)
				}
			}
			if strings.Contains(f.Content, " style=") || strings.Contains(f.Content, "style:") {
				t.Errorf("%s sets a style attribute", f.Path)
			}
		}
		if !strings.Contains(strings.SplitN(f.Content, "\n", 2)[0], "Generated by specarch-gen-ui-vue-htm from") {
			t.Errorf("%s does not say on line 1 which command wrote it", f.Path)
		}
	}
	index := fileOf(t, out.Files, "index.html")
	vue := strings.Index(index, "vendor/vue.runtime.global.prod.js")
	htm := strings.Index(index, "vendor/htm.umd.js")
	app := strings.Index(index, `<script type="module" src="app.js">`)
	if vue < 0 || htm < 0 || app < 0 || !(vue < htm && htm < app) {
		t.Errorf("index.html loads vue, htm and app.js out of order:\n%s", index)
	}
	if strings.Contains(index, "<base ") {
		t.Errorf("index.html names a base with no application route:\n%s", index)
	}
}

// A page's data names each call by its operation alone; how the call
// travels is in config/actions.js, once per operation.
func TestCallsAreReducedToTheirOperation(t *testing.T) {
	out := Generate(request(t, vendoredFiles(t), nil))
	list := fileOf(t, out.Files, "pages/members-list.js")
	if !strings.Contains(list, `source: { operation: "listMembers" }`) {
		t.Errorf("the list's source is not reduced to its operation:\n%s", list)
	}
	for _, key := range []string{"method:", "path:", "handshake:"} {
		if strings.Contains(list, key) {
			t.Errorf("the list's data still carries %s", key)
		}
	}
	actions := fileOf(t, out.Files, "config/actions.js")
	for _, want := range []string{
		`listMembers: { operation: "listMembers", method: "GET", path: "/members", permission: "members.read" }`,
		`suspendMember: { operation: "suspendMember", method: "POST", path: "/members/{memberId}/suspend", permission: "members.write" }`,
		`getSession: { operation: "getSession", method: "GET", path: "/sessions/current", permission: "public" }`,
	} {
		if !strings.Contains(actions, want) {
			t.Errorf("config/actions.js lacks %s:\n%s", want, actions)
		}
	}
	form := fileOf(t, out.Files, "pages/member-form.js")
	if !strings.Contains(form, "submit: {\n      operation: \"createMember\",\n      fields:") {
		t.Errorf("the form's submit is not reduced to its operation:\n%s", form)
	}
}

// A page's name as a JavaScript identifier: its words run together, a
// reserved word or a digit first prefixed.
func TestIdentifier(t *testing.T) {
	for in, want := range map[string]string{"member-form": "memberForm", "sign_in.page": "signInPage", "default": "pageDefault", "2fa": "page2fa", "members": "members"} {
		if got := identifier(in); got != want {
			t.Errorf("identifier(%q) = %q, want %q", in, got, want)
		}
	}
}

// The vendored files: an integrity not in the form, one of another
// length, one that differs from the file in the output folder, a library
// settings.vendor names without files, vue or htm missing from it and
// settings.server are errors; a file not in the output folder yet is a
// warning naming it.
func TestVendorIsChecked(t *testing.T) {
	run := func(existing []genui.File, change func(specification, content map[string]any)) genui.Response {
		return Generate(request(t, existing, change))
	}
	vendored := vendoredFiles(t)
	out := run(vendored, nil)
	if errs := messages(out.Diagnostics, "error"); len(errs) > 0 {
		t.Fatalf("the case does not generate: %v", errs)
	}
	manifest := fileOf(t, out.Files, "vendor/manifest.js")
	for _, want := range []string{`library: "vue"`, `version: "3.5.43"`, `licence: "MIT"`, `file: "vendor/htm.umd.js"`, "loaded: true"} {
		if !strings.Contains(manifest, want) {
			t.Errorf("vendor/manifest.js lacks %s:\n%s", want, manifest)
		}
	}

	out = run([]genui.File{translation(t)}, nil)
	if warns := messages(out.Diagnostics, "warning"); !containing(warns, "vendor/vue.runtime.global.prod.js is not in the output folder, and index.html loads it") {
		t.Errorf("a vendored file missing from the output folder: %v", warns)
	}

	out = run(vendored, func(_, content map[string]any) {
		libraries(content)["vue"].(map[string]any)["files"] = map[string]any{"vendor/vue.runtime.global.prod.js": "md5-abc"}
	})
	if errs := messages(out.Diagnostics, "error"); !containing(errs, `"md5-abc", is not a subresource integrity value`) {
		t.Errorf("an integrity not in the form: %v", errs)
	}

	out = run(vendored, func(_, content map[string]any) {
		libraries(content)["vue"].(map[string]any)["files"] = map[string]any{"vendor/vue.runtime.global.prod.js": "sha384-YWJj"}
	})
	if errs := messages(out.Diagnostics, "error"); !containing(errs, "names sha384, whose digest is 48 bytes in base64, and carries another length") {
		t.Errorf("an integrity of another length: %v", errs)
	}

	changed := []genui.File{vendored[0], {Path: vendored[1].Path, Content: vendored[1].Content + "\n// changed\n"}, vendored[2]}
	out = run(changed, nil)
	if errs := messages(out.Diagnostics, "error"); !containing(errs, "the browser would refuse the file") {
		t.Errorf("a file that differs from its integrity: %v", errs)
	}

	out = run(vendored, func(_, content map[string]any) {
		delete(libraries(content)["htm"].(map[string]any), "files")
	})
	if errs := messages(out.Diagnostics, "error"); !containing(errs, "names htm, and the library has no files") {
		t.Errorf("a library without files: %v", errs)
	}

	out = run(vendored, func(_, content map[string]any) {
		settings(content)["vendor"] = []any{"vue"}
	})
	if errs := messages(out.Diagnostics, "error"); !containing(errs, "does not name htm") {
		t.Errorf("htm missing from settings.vendor: %v", errs)
	}

	out = run(vendored, func(_, content map[string]any) {
		settings(content)["vendor"] = []any{"vue", "htm", "lodash"}
	})
	if errs := messages(out.Diagnostics, "error"); !containing(errs, "names lodash, which is not a library of") {
		t.Errorf("a library settings.vendor names that is not there: %v", errs)
	}

	out = run(vendored, func(_, content map[string]any) {
		settings(content)["server"] = map[string]any{"routes": "/api"}
	})
	if errs := messages(out.Diagnostics, "error"); !containing(errs, "settings.server names server routes, which specarch-gen-ui-vue-htm does not write") {
		t.Errorf("settings.server: %v", errs)
	}
}

// Under a declared application route the routes are paths under it and
// the document names its files from it; without one they are fragments.
func TestRoutingFollowsTheApplicationRoute(t *testing.T) {
	out := Generate(request(t, vendoredFiles(t), func(specification, _ map[string]any) {
		specification["routes"] = map[string]any{"app": map[string]any{"path": "/app/", "serves": "application"}}
	}))
	if errs := messages(out.Diagnostics, "error"); len(errs) > 0 {
		t.Fatalf("the case does not generate: %v", errs)
	}
	if app := fileOf(t, out.Files, "config/application.js"); !strings.Contains(app, `routing: { mode: "path", base: "/app" }`) {
		t.Errorf("config/application.js does not route by path under /app:\n%s", app)
	}
	if index := fileOf(t, out.Files, "index.html"); !strings.Contains(index, `<base href="/app/">`) {
		t.Errorf("index.html does not name its base:\n%s", index)
	}
	out = Generate(request(t, vendoredFiles(t), nil))
	if app := fileOf(t, out.Files, "config/application.js"); !strings.Contains(app, `routing: { mode: "hash" }`) {
		t.Errorf("config/application.js does not route by fragment:\n%s", app)
	}
}

// A form's hooks are imported from hooks/<page>.js when the form opens,
// keyed by the field's name on the wire.
func TestHooksAreKeyedByWireName(t *testing.T) {
	out := Generate(request(t, vendoredFiles(t), nil))
	if app := fileOf(t, out.Files, "app.js"); !strings.Contains(app, `"member-form": () => import("./hooks/member-form.js").then((module) => ({ joinedOn: module.joinedOn })),`) {
		t.Errorf("app.js does not import the form's hooks:\n%s", app)
	}
	out = Generate(request(t, vendoredFiles(t), func(specification, _ map[string]any) {
		specification["info"].(map[string]any)["wireNames"] = "snake_case"
	}))
	if app := fileOf(t, out.Files, "app.js"); !strings.Contains(app, `({ joined_on: module.joinedOn })`) {
		t.Errorf("app.js does not key the hook by its wire name:\n%s", app)
	}
}
