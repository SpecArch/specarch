import SwiftUI

/// The sign-in page.
public struct SignInView: View {
    public init() {}

    public var body: some View {
        TaskPage(schema: signInSchema, routes: [
            "members-list": { _ in .membersList },
            "second-factor": { _ in .secondFactor }
        ])
    }
}
