import SwiftUI

/// Opens a page only to someone who holds its permission. Anyone else sees
/// the page's refusal, or is sent to sign in first, and the page is never
/// made, so it calls no operation.
public struct Guard<Content: View>: View {
    let permission: String
    let route: Route
    let content: () -> Content
    @Environment(Router.self) private var router
    @AppStorage(Strings.languageKey) private var language = Strings.languages[0]
    private var store: SessionStore { SessionStore.shared }

    public init(permission: String, route: Route, @ViewBuilder content: @escaping () -> Content) {
        self.permission = permission
        self.route = route
        self.content = content
    }

    public var body: some View {
        let texts = Strings.texts(language)
        if permission == "public" {
            content()
        } else if let session = store.current {
            switch Access.decide(permission: permission, session: session) {
            case .allowed:
                content()
            case .signIn where Application.signIn != nil:
                ProgressView(say(texts, "screens.loading")).task { router.askToSignIn(from: route) }
            case .signIn, .refused:
                Text(say(texts, store.failed ? "screens.failed" : "screens.refused")).foregroundStyle(.red)
            }
        } else {
            ProgressView(say(texts, "screens.loading")).task { await store.read() }
        }
    }
}
