import SwiftUI

struct BookDetailView: View {
    @Bindable var book: Book

    var body: some View {
        Form {
            TextField("Title", text: $book.title)
            TextField("Author", text: $book.author)
            Toggle("Finished", isOn: $book.finished)
            Stepper("Pages", value: $book.pages)
            NavigationLink("Reviews") {
                ReviewListView(isbn: book.isbn)
            }
        }
        .navigationTitle("Book")
    }
}
