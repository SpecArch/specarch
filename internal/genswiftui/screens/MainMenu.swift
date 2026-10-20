import SwiftUI

/// The application's menu: one level of groups, each entry a link to a
/// page, shown only to someone who holds the permission of the page it
/// opens, read from the same permission as the page's guard.
public struct MainMenu: View {
    @AppStorage(Strings.languageKey) private var language = Strings.languages[0]
    private var store: SessionStore { SessionStore.shared }

    public init() {}

    public var body: some View {
        let texts = Strings.texts(language)
        List {
            if let session = store.current {
                ForEach(Access.visibleMenu(Application.menu, session: session), id: \.title) { group in
                    Section(say(texts, group.title)) {
                        ForEach(group.items, id: \.self) { item in
                            NavigationLink(say(texts, item.title), value: item.route)
                        }
                    }
                }
                if !session.signedIn, let signIn = Application.signIn {
                    NavigationLink(say(texts, Application.signInTitle), value: signIn)
                }
            } else {
                ProgressView(say(texts, "screens.loading")).task { await store.read() }
            }
        }
        .navigationTitle(say(texts, "screens.menu"))
        .overlay(alignment: .top) { NoticeView() }
    }
}
