import SwiftData
import SwiftUI

struct ShelfListView: View {
    @Query private var items: [Item]
    @State private var adding = false
    @State private var showHelp = false
    @State private var compact = true

    var body: some View {
        NavigationStack {
            List(items) { item in
                NavigationLink(item.name, value: item)
            }
            .navigationTitle("Shelf")
            .navigationDestination(for: Item.self) { item in
                ItemDetailView(item: item)
            }
            .sheet(isPresented: $adding) {
                ItemFormView()
            }
            .fullScreenCover(isPresented: $showHelp) {
                if compact {
                    HelpView()
                } else {
                    AboutView()
                }
            }
        }
    }
}
