import SwiftUI

/// The message an event left for the page shown, announced until it is
/// closed or another page opens.
public struct NoticeView: View {
    @Environment(Router.self) private var router
    @AppStorage(Strings.languageKey) private var language = Strings.languages[0]

    public init() {}

    public var body: some View {
        if let text = router.noticeShown() {
            HStack {
                Text(text).accessibilityAddTraits(.isStaticText)
                Spacer()
                Button(say(Strings.texts(language), "screens.cancel"), systemImage: "xmark") { router.forgetNotice() }
                    .labelStyle(.iconOnly)
            }
            .padding()
            .background(.thinMaterial, in: RoundedRectangle(cornerRadius: 8))
            .padding()
        }
    }
}
