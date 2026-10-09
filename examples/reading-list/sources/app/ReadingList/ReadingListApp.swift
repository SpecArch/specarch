import SwiftData
import SwiftUI

@main
struct ReadingListApp: App {
    var body: some Scene {
        WindowGroup {
            RootView()
        }
        .modelContainer(for: [Book.self, Note.self])
    }
}
