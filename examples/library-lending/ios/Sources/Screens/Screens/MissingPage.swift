import SwiftUI

/// A page an event or a menu leads to that this package does not hold:
/// one another stakeholder writes, or one this version does not write.
public struct MissingPage: View {
    let name: String

    public init(name: String) {
        self.name = name
    }

    public var body: some View {
        Text("The page \(name) is not part of this package.").foregroundStyle(.secondary).padding()
    }
}
