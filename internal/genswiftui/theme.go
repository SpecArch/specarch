package genswiftui

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/SpecArch/specarch/internal/genscreens"
	"github.com/SpecArch/specarch/internal/genui"
)

// themeFile is Theme.swift: a Color per part of the theme the settings map
// to a token, the light mode's colour or, where the system asks for dark,
// the dark mode's, through the design-tokens idiom's tokens for swiftui.
// Without the idiom it writes nothing.
func (g *gen) themeFile() (string, bool) {
	names := g.DesignTokens()
	themes := names["themes"]
	if themes == nil {
		return "", false
	}
	for _, role := range []string{"light", "dark"} {
		if themes[role] == "" {
			g.Problem("error", "/", "the design-tokens idiom's part themes names no %s for %s; add it under names", role, framework)
			return "", false
		}
	}
	colours := g.ThemeParts(names)
	tokens := make([]string, 0, len(colours.Light))
	for t := range colours.Light {
		tokens = append(tokens, t)
	}
	sort.Strings(tokens)
	var b strings.Builder
	b.WriteString("import SwiftUI\n\n/// The specification's theme as colours: each part's colour in the light\n/// mode, and the dark mode's where the system asks for dark. The app sets\n/// accent as its tint and draws its own views with the rest.\npublic enum Theme {\n")
	for _, t := range tokens {
		fmt.Fprintf(&b, "    public static let %s = Color(light: %s, dark: %s)\n", camelCase(t), swiftColor(colours.Light[t]), swiftColor(colours.Dark[t]))
	}
	b.WriteString(`}

extension Color {
    /// A colour of the light mode, or of the dark mode where the system asks for dark.
    init(light: Color, dark: Color) {
        #if os(iOS)
        self.init(UIColor { traits in traits.userInterfaceStyle == .dark ? UIColor(dark) : UIColor(light) })
        #else
        self.init(NSColor(name: nil) { appearance in appearance.bestMatch(from: [.darkAqua, .aqua]) == .darkAqua ? NSColor(dark) : NSColor(light) })
        #endif
    }

    /// A colour from its hex value, #rrggbb or #rrggbbaa, as a design token writes one.
    init(hex: String) {
        var digits = hex.dropFirst(hex.hasPrefix("#") ? 1 : 0)
        if digits.count == 6 {
            digits += "ff"
        }
        let value = UInt64(digits, radix: 16) ?? 0
        self.init(
            red: Double((value >> 24) & 0xff) / 255,
            green: Double((value >> 16) & 0xff) / 255,
            blue: Double((value >> 8) & 0xff) / 255,
            opacity: Double(value & 0xff) / 255
        )
    }
}
`)
	return b.String(), true
}

// swiftColor is a theme colour as a Color: its hex value, with its alpha
// when it has one, through the initialiser above.
func swiftColor(v any) string {
	if m, ok := v.(map[string]any); ok {
		if comps := genscreens.List(m["components"]); len(comps) == 3 {
			var c [3]int
			for i, x := range comps {
				f, _ := strconv.ParseFloat(text(x), 64)
				c[i] = int(math.Round(f * 255))
			}
			hex := fmt.Sprintf("#%02x%02x%02x", c[0], c[1], c[2])
			if a := text(m["alpha"]); a != "" && a != "1" {
				f, _ := strconv.ParseFloat(a, 64)
				hex += fmt.Sprintf("%02x", int(math.Round(f*255)))
			}
			return "Color(hex: " + quote(hex) + ")"
		}
	}
	return "Color(hex: " + quote(genui.CSSValue("color", v)) + ")"
}
