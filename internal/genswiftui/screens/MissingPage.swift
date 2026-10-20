import SwiftUI

/// A page an event or a menu leads to that this package does not hold:
/// one another stakeholder writes, or one this version does not write.
public struct MissingPage: View {
    let name: String
    @AppStorage(Strings.languageKey) private var language = Strings.languages[0]

    public init(name: String) {
        self.name = name
    }

    public var body: some View {
        Text(say(Strings.texts(language), "screens.missingPage").replacingOccurrences(of: "{name}", with: name)).foregroundStyle(.secondary).padding()
    }
}
