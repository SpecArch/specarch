import SwiftUI

struct AddBookView: View {
    @Environment(\.dismiss) private var dismiss
    @State private var draft = BookDraft(isbn: "", title: "")
    private let client = ReviewClient()

    var body: some View {
        NavigationStack {
            Form {
                TextField("ISBN", text: $draft.isbn)
                TextField("Title", text: $draft.title)
            }
            .navigationTitle("Add a book")
            .toolbar {
                Button("Save") {
                    Task {
                        try await client.add(draft)
                        dismiss()
                    }
                }
            }
        }
    }
}
