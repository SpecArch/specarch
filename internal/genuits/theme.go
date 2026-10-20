package genuits

import (
	"fmt"
	"sort"
	"strings"

	"github.com/SpecArch/specarch/internal/genui"
)

// themeFile is theme.scss: the library's light and dark themes with the
// colours of the parts the settings map laid over them, through the
// design-tokens idiom. Without the idiom it writes nothing.
func (g *gen) themeFile() (string, bool) {
	names := g.DesignTokens()
	themes := names["themes"]
	if themes == nil {
		return "", false
	}
	for _, role := range []string{"light", "dark", "themes", "theme"} {
		if themes[role] == "" {
			g.Problem("error", "/", "the design-tokens idiom's part themes names no %s for %s; add it under names", role, framework)
			return "", false
		}
	}
	colours := g.ThemeParts(names)
	light := map[string]string{}
	dark := map[string]string{}
	for token, v := range colours.Light {
		light[token] = genui.CSSValue("color", v)
		dark[token] = genui.CSSValue("color", colours.Dark[token])
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
