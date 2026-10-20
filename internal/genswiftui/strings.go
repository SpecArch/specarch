package genswiftui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/SpecArch/specarch/internal/genscreens"
	"github.com/SpecArch/specarch/internal/genui"
)

// stringsFile is Strings.swift: every text the written pages and the
// screens show, by key, in the target's language and in each translation,
// and the key the language a person chose is kept under.
func (g *gen) stringsFile(existing []genui.File) (string, bool) {
	all := map[string]string{}
	for k, v := range genscreens.ScreenStrings {
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
	b.WriteString("/// Every text the screens show, by string key: in the specification's\n")
	fmt.Fprintf(&b, "/// language, %s, and in each translation, read from its file beside the\n/// package when the screens were generated.\npublic enum Strings {\n", language)
	every := append([]string{language}, codes...)
	quoted := make([]string, len(every))
	for i, c := range every {
		quoted[i] = quote(c)
	}
	fmt.Fprintf(&b, "    public static let languages = [%s]\n\n", strings.Join(quoted, ", "))
	b.WriteString("    /// The key the language a person chose is kept under in the app's defaults.\n")
	fmt.Fprintf(&b, "    public static let languageKey = %s\n\n", quote(g.Name("language-choice", "key")))
	b.WriteString("    static let strings: [String: String] = [\n")
	for _, k := range keys {
		fmt.Fprintf(&b, "        %s: %s,\n", quote(k), quote(all[k]))
	}
	b.WriteString("    ]\n\n    /// The texts in each other language, an entry for every key.\n")
	if len(codes) == 0 {
		b.WriteString("    static let translations: [String: [String: String]] = [:]\n")
	} else {
		b.WriteString("    static let translations: [String: [String: String]] = [\n")
		sort.Strings(codes)
		for _, c := range codes {
			fmt.Fprintf(&b, "        %s: [\n", quote(c))
			for _, k := range keys {
				fmt.Fprintf(&b, "            %s: %s,\n", quote(k), quote(translated[c][k]))
			}
			b.WriteString("        ],\n")
		}
		b.WriteString("    ]\n")
	}
	b.WriteString(`
    /// The texts of a language: the specification's own, or a translation.
    public static func texts(_ language: String) -> Texts {
        translations[language] ?? strings
    }
}
`)
	return b.String(), true
}
