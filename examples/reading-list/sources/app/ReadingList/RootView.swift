import SwiftUI

struct RootView: View {
    var body: some View {
        TabView {
            BookListView()
                .tabItem { Label("Books", systemImage: "books.vertical") }
            SettingsView()
                .tabItem { Label("Settings", systemImage: "gear") }
        }
    }
}
