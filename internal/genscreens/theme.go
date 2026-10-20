package genscreens

import (
	"strings"
)

// DesignTokens reads the names of the design-tokens idiom for the
// framework, by part, the project's override first, key by key.
func (g *Gen) DesignTokens() map[string]map[string]string {
	out := map[string]map[string]string{}
	for _, i := range g.Impl.Idioms {
		if i.Name != "design-tokens" || i.As == "excluded" {
			continue
		}
		for _, src := range []map[string]any{i.Override, i.Content} {
			parts := obj0(src["parts"])
			for _, part := range sortedKeys(parts) {
				if out[part] == nil {
					out[part] = map[string]string{}
				}
				for k, v := range obj0(obj0(obj0(obj0(parts[part])["stack"])[g.Profile.Framework])["names"]) {
					if _, ok := out[part][k]; !ok {
						out[part][k] = text(v)
					}
				}
			}
		}
	}
	return out
}

// ThemeTokens are the theme's tokens by path, each with its type and
// value, the type given by it or a group above it.
func ThemeTokens(group map[string]any, prefix, typ string, out map[string][2]any) {
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
		ThemeTokens(v, path, typ, out)
	}
}

// Theme is the theme's colours laid over the library's: the value of each
// token of the library the design-tokens idiom names, in the light mode
// and in the dark mode, as the theme gives them.
type Theme struct {
	// Light and Dark are the colour of each library token, as the theme
	// writes a colour value.
	Light, Dark map[string]any
	// Types are the type of each token the theme gives, by its path.
	Types map[string]string
}

// ThemeParts reads settings.tokens, the parts (text, background, accent,
// link, danger, border) mapped to tokens of the theme, and gives each
// library token that takes a part its colour in the light and the dark
// mode, through the design-tokens idiom's tokens names, which map each
// library token to a part. Modes other than dark are reported and left
// out; a part no library token takes, a token that is not a colour, and
// a theme that reaches no token are reported. false when the theme is not
// written, the idiom's themes part naming no rendering.
func (g *Gen) ThemeParts(names map[string]map[string]string) Theme {
	theme := obj0(g.Spec["theme"])
	tokens := map[string][2]any{}
	ThemeTokens(obj0(theme["tokens"]), "", "", tokens)
	modes := obj0(theme["modes"])
	for _, m := range sortedKeys(modes) {
		if m != "dark" {
			g.Problem("warning", "/theme/modes/"+m, "the mode %s %s, and %s writes the dark mode only; it is left out", m, g.Profile.OtherMode, g.Profile.Name)
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
	parts := obj0(g.Settings["tokens"])
	takes := map[string][]string{} // the library's tokens each part goes to
	for library, part := range names["tokens"] {
		takes[part] = append(takes[part], library)
	}
	out := Theme{Light: map[string]any{}, Dark: map[string]any{}, Types: map[string]string{}}
	for _, part := range sortedKeys(parts) {
		path := text(parts[part])
		at := "/"
		if _, ok := tokens[path]; !ok {
			g.Problem("error", at, "the ui target's settings.tokens gives %s the token %s, which is not a token of the theme", part, path)
			continue
		}
		if len(takes[part]) == 0 {
			g.Problem("warning", at, "the ui target's settings.tokens gives %s a token, and no token of the design-tokens idiom takes %s on %s, whose own spacing and type scale apply; it is left out", part, part, g.Profile.Framework)
			continue
		}
		v, typ := valueIn(path, "", 0)
		if typ != "color" {
			g.Problem("error", at, "the ui target's settings.tokens gives %s the token %s, of type %s, and %s lays only colours over the library's themes", part, path, orText(typ, "none"), g.Profile.Name)
			continue
		}
		d, _ := valueIn(path, "dark", 0)
		for _, library := range takes[part] {
			out.Light[library] = v
			out.Dark[library] = d
			out.Types[library] = typ
		}
	}
	if len(tokens) > 0 && len(parts) == 0 {
		g.Problem("warning", "/theme", "the theme's colours reach no token of %s, and its own look applies; map the parts to the theme's tokens under the ui target's settings.tokens", g.Profile.Framework)
	}
	return out
}
