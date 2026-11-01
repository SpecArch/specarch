package genvuehtm

import (
	"fmt"
	"sort"
	"strings"

	"github.com/SpecArch/specarch/internal/genscreens"
	"github.com/SpecArch/specarch/internal/genui"
)

// stringsFile is strings.js: every text the written pages and the screens
// show, by key, in the target's language and in each translation, and the
// key the language a person chose is kept under.
func (g *gen) stringsFile(existing []genui.File) (string, bool) {
	all := map[string]string{}
	for k, v := range genscreens.ScreenStrings {
		all[k] = v
	}
	for k, v := range vueStrings {
		all[k] = v
	}
	for k, v := range g.Strings {
		all[k] = v
	}
	codes, translated, ok := g.Translations(all, existing)
	if !ok {
		return "", false
	}
	language := text(g.Settings["language"])
	keys := sortedKeys(anyMap(all))
	var b strings.Builder
	b.WriteString("/**\n * Every text the screens show, by string key: in the specification's\n")
	fmt.Fprintf(&b, " * language, %s, and in each translation, read from its file beside this one.\n */\n", language)
	fmt.Fprintf(&b, "export const languages = %s;\n\n", flat(array(stringValues(append([]string{language}, codes...)))))
	b.WriteString("/** The key the language a person chose is kept under, in the browser's local storage. */\n")
	fmt.Fprintf(&b, "export const languageKey = %s;\n\n", quote(g.Name("language-choice", "key")))
	b.WriteString("export const strings = {\n")
	fmt.Fprintf(&b, "  %s: {\n", quote(language))
	for _, k := range keys {
		fmt.Fprintf(&b, "    %s: %s,\n", quote(k), quote(all[k]))
	}
	b.WriteString("  },\n")
	sort.Strings(codes)
	for _, c := range codes {
		fmt.Fprintf(&b, "  %s: {\n", quote(c))
		for _, k := range keys {
			fmt.Fprintf(&b, "    %s: %s,\n", quote(k), quote(translated[c][k]))
		}
		b.WriteString("  },\n")
	}
	b.WriteString("};\n")
	return b.String(), true
}

func stringValues(ss []string) []value {
	out := make([]value, len(ss))
	for i, s := range ss {
		out[i] = str(s)
	}
	return out
}
