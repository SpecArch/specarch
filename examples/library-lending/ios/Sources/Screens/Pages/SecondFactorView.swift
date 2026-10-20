import SwiftUI

/// The second-factor page.
public struct SecondFactorView: View {
    public init() {}

    public var body: some View {
        TaskPage(schema: secondFactorSchema, routes: [
            "members-list": { _ in .membersList }
        ])
    }
}
