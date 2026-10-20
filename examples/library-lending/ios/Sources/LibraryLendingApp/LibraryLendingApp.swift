import Screens
import SwiftUI

/// The library lending app: the menu at the root of one navigation stack,
/// every page opened through its route, and the service at the URL set
/// here. The language chosen is kept across launches by the screens.
@main
struct LibraryLendingApp: App {
    @State private var router = Router()

    init() {
        Service.baseURL = URL(string: ProcessInfo.processInfo.environment["LIBRARY_LENDING_URL"] ?? "http://localhost:8080")!
    }

    var body: some Scene {
        WindowGroup {
            NavigationStack(path: $router.path) {
                MainMenu()
                    .navigationDestination(for: Route.self) { route in route.destination }
            }
            .environment(router)
            .tint(Theme.accent)
        }
    }
}
