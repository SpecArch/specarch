import SwiftData
import SwiftUI

struct BookListView: View {
    @Query(sort: \Book.title) private var books: [Book]
    @State private var adding = false

    var body: some View {
        NavigationStack {
            List(books) { book in
                NavigationLink(book.title, value: book)
            }
            .navigationTitle("Books")
            .navigationDestination(for: Book.self) { book in
                BookDetailView(book: book)
            }
            .toolbar {
                Button("Add") { adding = true }
            }
            .sheet(isPresented: $adding) {
                AddBookView()
            }
        }
    }
}
