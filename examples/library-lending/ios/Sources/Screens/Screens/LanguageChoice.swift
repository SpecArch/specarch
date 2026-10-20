import SwiftUI

/// The choice of the language the screens are shown in, kept in the app's
/// defaults across launches; the app's settings place it.
public struct LanguageChoice: View {
    @AppStorage(Strings.languageKey) private var language = Strings.languages[0]

    public init() {}

    public var body: some View {
        Picker(say(Strings.texts(language), "screens.language"), selection: $language) {
            ForEach(Strings.languages, id: \.self) { code in
                Text(Locale.current.localizedString(forIdentifier: code) ?? code).tag(code)
            }
        }
    }
}
