package genuits

import (
	"fmt"
	"sort"
	"strings"

	"github.com/SpecArch/specarch/internal/genui"
)

// designTokens reads the names of the design-tokens idiom for
// nextjs-carbon, by part, the project's override first, key by key.
func (g *gen) designTokens() map[string]map[string]string {
	out := map[string]map[string]string{}
	for _, i := range g.impl.Idioms {
		if i.Name != "design-tokens" || i.As == "excluded" {
			continue
		}
		for _, src := range []map[string]any{i.Override, i.Content} {
			parts := obj0(src["parts"])
			for _, part := range sortedKeys(parts) {
				if out[part] == nil {
					out[part] = map[string]string{}
				}
				for k, v := range obj0(obj0(obj0(obj0(parts[part])["stack"])[framework])["names"]) {
					if _, ok := out[part][k]; !ok {
						out[part][k] = text(v)
					}
				}
			}
		}
	}
	return out
}

// themeTokens are the theme's tokens by path, each with its type and
// value, the type given by it or a group above it.
func themeTokens(group map[string]any, prefix, typ string, out map[string][2]any) {
	if t := text(group["$type"]); t != "" {
		typ = t
	}
	for _, k := range sortedKeys(group) {
		if strings.HasPrefix(k, "$") {
			continue
		}
		path := strings.TrimPrefix(prefix+"."+k, ".")
		v := obj0(group[k])
		if val, ok := v["$value"]; ok {
			out[path] = [2]any{orText(text(v["$type"]), typ), val}
			continue
		}
		themeTokens(v, path, typ, out)
	}
}

// themeFile is theme.scss: the library's light and dark themes with the
// colours of the parts the settings map laid over them, through the
// design-tokens idiom. Without the idiom it writes nothing.
func (g *gen) themeFile() (string, bool) {
	names := g.designTokens()
	themes := names["themes"]
	if themes == nil {
		return "", false
	}
	for _, role := range []string{"light", "dark", "themes", "theme"} {
		if themes[role] == "" {
			g.problem("error", "/", "the design-tokens idiom's part themes names no %s for %s; add it under names", role, framework)
			return "", false
		}
	}
	theme := obj0(g.spec["theme"])
	tokens := map[string][2]any{}
	themeTokens(obj0(theme["tokens"]), "", "", tokens)
	modes := obj0(theme["modes"])
	for _, m := range sortedKeys(modes) {
		if m != "dark" {
			g.problem("warning", "/theme/modes/"+m, "the mode %s has no media query of its own on the web, and %s writes the dark mode only; it is left out", m, name)
		}
	}
	// valueIn is a token's value in a mode, an alias read through.
	var valueIn func(path, mode string, depth int) (any, string)
	valueIn = func(path, mode string, depth int) (any, string) {
		t, ok := tokens[path]
		if !ok || depth > 8 {
			return nil, ""
		}
		v := t[1]
		if over, ok := obj0(modes[mode])[path]; ok && mode != "" {
			v = over
		}
		if s, ok := v.(string); ok && strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}") {
			return valueIn(strings.Trim(s, "{}"), mode, depth+1)
		}
		return v, text(t[0])
	}
	parts := obj0(g.settings["tokens"])
	takes := map[string][]string{} // the library's tokens each part goes to
	for carbon, part := range names["tokens"] {
		takes[part] = append(takes[part], carbon)
	}
	light := map[string]string{}
	dark := map[string]string{}
	for _, part := range sortedKeys(parts) {
		path := text(parts[part])
		at := "/"
		if _, ok := tokens[path]; !ok {
			g.problem("error", at, "the ui target's settings.tokens gives %s the token %s, which is not a token of the theme", part, path)
			continue
		}
		if len(takes[part]) == 0 {
			g.problem("warning", at, "the ui target's settings.tokens gives %s a token, and no token of the design-tokens idiom takes %s on %s, whose own spacing and type scale apply; it is left out", part, part, framework)
			continue
		}
		v, typ := valueIn(path, "", 0)
		if typ != "color" {
			g.problem("error", at, "the ui target's settings.tokens gives %s the token %s, of type %s, and %s lays only colours over the library's themes", part, path, orText(typ, "none"), name)
			continue
		}
		d, _ := valueIn(path, "dark", 0)
		for _, carbon := range takes[part] {
			light[carbon] = genui.CSSValue("color", v)
			dark[carbon] = genui.CSSValue("color", d)
		}
	}
	if len(tokens) > 0 && len(parts) == 0 {
		g.problem("warning", "/theme", "the theme's colours reach no token of %s, and its own look applies; map the parts to the theme's tokens under the ui target's settings.tokens", framework)
	}
	merged := func(base string, over map[string]string, indent string) string {
		if len(over) == 0 {
			return "themes.$" + base
		}
		keys := make([]string, 0, len(over))
		for k := range over {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		var b strings.Builder
		fmt.Fprintf(&b, "map.merge(\n%s    themes.$%s,\n%s    (\n", indent, base, indent)
		for _, k := range keys {
			fmt.Fprintf(&b, "%s      %q: %s,\n", indent, k, over[k])
		}
		fmt.Fprintf(&b, "%s    )\n%s  )", indent, indent)
		return b.String()
	}
	var b strings.Builder
	fmt.Fprintf(&b, "// The specification's theme as Carbon's theme tokens: the %s theme with\n// the parts' colours in the light mode, and the %s theme with the dark\n// mode's colours where the system asks for dark.\n", themes["light"], themes["dark"])
	fmt.Fprintf(&b, "@use \"sass:map\";\n@use %q;\n@use %q;\n\n", themes["themes"], themes["theme"])
	fmt.Fprintf(&b, ":root {\n  @include theme.theme(\n    %s\n  );\n}\n\n", merged(themes["light"], light, "  "))
	fmt.Fprintf(&b, "@media (prefers-color-scheme: dark) {\n  :root {\n    @include theme.theme(\n      %s\n    );\n  }\n}\n", merged(themes["dark"], dark, "    "))
	return b.String(), true
}
