import SwiftUI

/// The pages, one case each with the parameters of its route; the app's
/// NavigationStack opens each through destination.
public enum Route: Hashable, Sendable {
    case membersList
    case secondFactor
    case signIn
}

extension Route {
    /// The view the route opens.
    @MainActor @ViewBuilder public var destination: some View {
        switch self {
        case .membersList: MissingPage(name: "members-list")
        case .secondFactor: SecondFactorView()
        case .signIn: SignInView()
        }
    }
}
