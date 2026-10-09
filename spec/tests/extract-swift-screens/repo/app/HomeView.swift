import SwiftUI

struct HomeView: View {
    @State private var moreTitle = "More"

    var body: some View {
        TabView {
            ShelfListView()
                .tabItem { Label("Shelf", systemImage: "books.vertical") }
            Tab("Search", systemImage: "magnifyingglass") {
                SearchScreen()
            }
            AboutView()
                .tabItem { Text(moreTitle) }
        }
    }
}
