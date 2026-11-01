package genvuehtm

import (
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"fmt"
	"hash"
	"html"
	"regexp"
	"strings"

	"github.com/SpecArch/specarch/internal/genscreens"
	"github.com/SpecArch/specarch/internal/genui"
)

// vendored is one file a library ships, committed beside the generated
// code: the library's name, version and licence, the file's path under the
// output folder and its integrity, and whether index.html loads it.
type vendored struct {
	library, version, licence, file, integrity string
	loaded                                     bool
}

// integrityForm is a subresource integrity value: the hash function and the
// digest in base64 (W3C Subresource Integrity, 3.3).
var integrityForm = regexp.MustCompile(`^sha(256|384|512)-([A-Za-z0-9+/]+={0,2})$`)

// readVendor reads the libraries' files from the implementation file:
// every file under libraries.<name>.files, with its integrity checked in
// form and, when the file is in the output folder, against its bytes, and
// the libraries settings.vendor names as the ones index.html loads, in
// that order, vue and htm among them. An error at the entry refuses what
// does not fit; a file not in the output folder yet is a warning naming it.
func (g *gen) readVendor(existing []genui.File) []vendored {
	content := map[string]string{}
	for _, f := range existing {
		content[f.Path] = f.Content
	}
	libraries := obj0(g.Impl.Content["libraries"])
	named, given := g.Settings["vendor"]
	order := []string{"vue", "htm"}
	if given {
		order = nil
		for _, l := range list(named) {
			order = append(order, text(l))
		}
		if list(named) == nil {
			g.Problem("error", "/", "the ui target's settings.vendor is not a list; name the libraries index.html loads, in order, such as [vue, htm]")
		}
	}
	loads := map[string]int{}
	for i, l := range order {
		loads[l] = i
	}
	for _, must := range []string{"vue", "htm"} {
		if _, ok := loads[must]; !ok {
			g.Problem("error", "/", "the ui target's settings.vendor does not name %s, and the screens run on Vue's runtime-only build and htm; name both, in order, under settings.vendor, each a library under libraries with its files", must)
		}
	}
	for _, l := range order {
		lib, ok := genscreens.Obj(libraries[l])
		if !ok {
			g.Problem("error", "/", "the ui target's settings.vendor names %s, which is not a library of %s; add it under libraries with its version, licence, purpose and files", l, g.Impl.File)
			continue
		}
		if len(obj0(lib["files"])) == 0 {
			g.Problem("error", "/", "the ui target's settings.vendor names %s, and the library has no files; list each file it ships under libraries.%s.files, by its path under the output folder, with its integrity", l, l)
		}
	}
	var out []vendored
	for _, l := range sortedKeys(libraries) {
		lib := obj0(libraries[l])
		files := obj0(lib["files"])
		_, loaded := loads[l]
		for _, file := range sortedKeys(files) {
			integrity := text(files[file])
			at := "/libraries/" + l + "/files/" + strings.ReplaceAll(strings.ReplaceAll(file, "~", "~0"), "/", "~1")
			m := integrityForm.FindStringSubmatch(integrity)
			if m == nil {
				g.Problem("error", at, "the integrity of %s, %q, is not a subresource integrity value: sha256-, sha384- or sha512- and the file's digest in base64, such as the output of openssl dgst -sha384 -binary %s | openssl base64 -A", file, integrity, file)
				continue
			}
			digest, err := base64.StdEncoding.DecodeString(m[2])
			if err != nil || len(digest) != digestLength(m[1]) {
				g.Problem("error", at, "the integrity of %s names sha%s, whose digest is %d bytes in base64, and carries another length; compute it again with openssl dgst -sha%s -binary %s | openssl base64 -A", file, m[1], digestLength(m[1]), m[1], file)
				continue
			}
			if data, there := content[file]; there {
				if computed := digestOf(m[1], data); computed != m[2] {
					g.Problem("error", at, "%s in the output folder has the integrity sha%s-%s, and the implementation file says %s; the browser would refuse the file, so take the version the implementation file names, or compute the hash of the file there and write it", file, m[1], computed, integrity)
					continue
				}
			} else if loaded {
				g.Problem("warning", at, "%s is not in the output folder, and index.html loads it with its integrity; copy %s %s's file there, as it ships it", file, l, text(lib["version"]))
			}
			out = append(out, vendored{library: l, version: text(lib["version"]), licence: text(lib["licence"]), file: file, integrity: integrity, loaded: loaded})
		}
	}
	// The files index.html loads come first, in the order settings.vendor
	// gives their libraries; the rest follow by library.
	sortVendored(out, loads)
	return out
}

func sortVendored(v []vendored, loads map[string]int) {
	rank := func(x vendored) int {
		if i, ok := loads[x.library]; ok {
			return i
		}
		return len(loads)
	}
	for i := 1; i < len(v); i++ {
		for j := i; j > 0 && (rank(v[j]) < rank(v[j-1]) || (rank(v[j]) == rank(v[j-1]) && v[j].library == v[j-1].library && v[j].file < v[j-1].file)); j-- {
			v[j], v[j-1] = v[j-1], v[j]
		}
	}
}

// digestLength is the length of a hash function's digest in bytes.
func digestLength(bits string) int {
	switch bits {
	case "256":
		return 32
	case "384":
		return 48
	}
	return 64
}

// digestOf is a file's digest in base64, by the hash function named.
func digestOf(bits, data string) string {
	var h hash.Hash
	switch bits {
	case "256":
		h = sha256.New()
	case "384":
		h = sha512.New384()
	default:
		h = sha512.New()
	}
	h.Write([]byte(data))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// manifestFile is vendor/manifest.js: a row per vendored file with the
// library's name, version and licence, the file and its integrity, and
// whether index.html loads it.
func (g *gen) manifestFile(v []vendored) string {
	var rows []value
	for _, x := range v {
		rows = append(rows, object([]member{
			{Key: "library", Val: str(x.library)},
			{Key: "version", Val: str(x.version)},
			{Key: "licence", Val: str(x.licence)},
			{Key: "file", Val: str(x.file)},
			{Key: "integrity", Val: str(x.integrity)},
			{Key: "loaded", Val: raw(fmt.Sprint(x.loaded))},
		}))
	}
	var b strings.Builder
	b.WriteString("/**\n * The vendor manifest: every file a third-party library ships, committed\n * beside the generated code, with the library's version and licence from\n * the implementation file, the file's path and the integrity index.html\n * loads it with; loaded says whether index.html loads it.\n */\n")
	b.WriteString(statement("export const manifest = ", array(rows), ";"))
	return b.String()
}

// indexFile is index.html, the one document: the title, the stylesheets,
// the vendored files settings.vendor names in order, each with its
// integrity, and app.js as a module; no inline script, no inline style and
// no event attribute, so it loads under a policy with neither unsafe-inline
// nor unsafe-eval. Under a declared application route the document names
// its files from that route through base, since the server answers it at
// every page route.
func (g *gen) indexFile(v []vendored) string {
	var b strings.Builder
	title := text(obj0(g.Spec["info"])["title"])
	fmt.Fprintf(&b, "<!doctype html>\n<html lang=%q>\n<head>\n<meta charset=\"utf-8\">\n<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">\n<title>%s</title>\n", text(g.Settings["language"]), html.EscapeString(title))
	if base, ok := g.applicationBase(); ok {
		// The server answers this document at every page route under the
		// application route, so the files are named from that route, not
		// from the page's.
		fmt.Fprintf(&b, "<base href=%q>\n", base+"/")
	}
	b.WriteString("<link rel=\"stylesheet\" href=\"theme.css\">\n<link rel=\"stylesheet\" href=\"screens/screens.css\">\n")
	for _, x := range v {
		if x.loaded && strings.HasSuffix(x.file, ".css") {
			fmt.Fprintf(&b, "<link rel=\"stylesheet\" href=%q integrity=%q>\n", x.file, x.integrity)
		}
	}
	for _, x := range v {
		if x.loaded && strings.HasSuffix(x.file, ".js") {
			fmt.Fprintf(&b, "<script src=%q integrity=%q></script>\n", x.file, x.integrity)
		}
	}
	b.WriteString("<script type=\"module\" src=\"app.js\"></script>\n</head>\n<body>\n<div id=\"app\"></div>\n")
	fmt.Fprintf(&b, "<noscript><p>%s</p></noscript>\n</body>\n</html>\n", html.EscapeString(vueStrings["screens.noScript"]))
	return b.String()
}
