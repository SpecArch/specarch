import SwiftUI

/// The specification's theme as colours: each part's colour in the light
/// mode, and the dark mode's where the system asks for dark. The app sets
/// accent as its tint and draws its own views with the rest.
public enum Theme {
    public static let accent = Color(light: Color(hex: "#0b5cad"), dark: Color(hex: "#6cb0f5"))
    public static let background = Color(light: Color(hex: "#ffffff"), dark: Color(hex: "#101214"))
    public static let border = Color(light: Color(hex: "#6e7781"), dark: Color(hex: "#8c959f"))
    public static let danger = Color(light: Color(hex: "#b3261e"), dark: Color(hex: "#f2b8b5"))
    public static let link = Color(light: Color(hex: "#0b5cad"), dark: Color(hex: "#6cb0f5"))
    public static let text = Color(light: Color(hex: "#1b1f24"), dark: Color(hex: "#e6e8eb"))
}

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
