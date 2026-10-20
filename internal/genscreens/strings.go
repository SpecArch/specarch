package genscreens

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/SpecArch/specarch/internal/genui"
)

// languageCode is a language as BCP 47 writes one, such as id or pt-BR.
var languageCode = regexp.MustCompile(`^[a-z]{2,3}(-[A-Za-z0-9]{2,8})*$`)

// placeholder is a value a text has filled in, such as {count}.
var placeholder = regexp.MustCompile(`\{[a-z]+\}`)

// TranslationFile is the file in the output folder that holds a
// language's texts, which the project writes and the generator only reads.
func TranslationFile(language string) string {
	return "strings." + language + ".json"
}

// Translations reads settings.translations, the languages besides the
// specification's, each from its file in the output folder. Every key the
// screens use must have an entry in each, and a key no screen uses is
// reported and left out.
func (g *Gen) Translations(keys map[string]string, existing []genui.File) ([]string, map[string]map[string]string, bool) {
	raw, named := g.Settings["translations"]
	if !named {
		return nil, nil, true
	}
	languages := list(raw)
	if languages == nil {
		g.Problem("error", "/", "the ui target's settings.translations is not a list; list the languages besides %s, such as [id]", text(g.Settings["language"]))
		return nil, nil, false
	}
	files := map[string]string{}
	for _, f := range existing {
		files[f.Path] = f.Content
	}
	ok := true
	var codes []string
	out := map[string]map[string]string{}
	for _, l := range languages {
		code := text(l)
		file := TranslationFile(code)
		switch {
		case !languageCode.MatchString(code):
			g.Problem("error", "/", "the ui target's settings.translations names %q, which is not a language as BCP 47 writes one, such as id or pt-BR", code)
			ok = false
			continue
		case code == text(g.Settings["language"]) || out[code] != nil:
			g.Problem("error", "/", "the ui target's settings.translations names %s twice, or as the specification's own language; name each other language once", code)
			ok = false
			continue
		}
		content, found := files[file]
		if !found {
			g.Problem("error", "/", "the ui target's settings.translations names %s, and %s is not in the output folder; write it, an object with the %s text of each of the %d keys the screens use", code, file, code, len(keys))
			ok = false
			continue
		}
		var entries map[string]any
		if err := json.Unmarshal([]byte(content), &entries); err != nil {
			g.Problem("error", "/", "%s is not a JSON object of string keys and texts: %v", file, err)
			ok = false
			continue
		}
		texts := map[string]string{}
		for _, k := range sortedKeys(entries) {
			t, isText := entries[k].(string)
			_, used := keys[k]
			switch {
			case !used:
				g.Problem("warning", "/", "%s has an entry for %s, which no screen uses; it is left out", file, k)
			case !isText || strings.TrimSpace(t) == "":
				g.Problem("error", "/", "%s gives %s no text; give it the %s text of %q", file, k, code, keys[k])
				ok = false
			default:
				if missing := missingPlaceholders(keys[k], t); missing != "" {
					g.Problem("error", "/", "%s gives %s without %s, which the %s text fills in; keep it in the translation", file, k, missing, text(g.Settings["language"]))
					ok = false
				}
				texts[k] = t
			}
		}
		for _, k := range sortedKeys(anyMap(keys)) {
			if _, has := entries[k]; !has {
				g.Problem("error", "/", "%s has no entry for %s, which the screens use, so a page would show the key; add its %s text of %q", file, k, code, keys[k])
				ok = false
			}
		}
		codes = append(codes, code)
		out[code] = texts
	}
	return codes, out, ok
}

// missingPlaceholders are the placeholders of a text its translation leaves out.
func missingPlaceholders(text, translated string) string {
	var missing []string
	for _, p := range placeholder.FindAllString(text, -1) {
		if !strings.Contains(translated, p) {
			missing = append(missing, p)
		}
	}
	return strings.Join(missing, ", ")
}
