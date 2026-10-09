import Foundation
import SwiftData

@Model
final class Book {
    @Attribute(.unique) var isbn: String
    var title: String
    var author: String
    var finished: Bool
    var startedOn: Date?
    var pages: Int
    var rating: Float?
    @Relationship(deleteRule: .cascade, inverse: \Note.book) var notes: [Note] = []
    @Transient var opened = false

    var summary: String { "\(title) by \(author)" }

    init(isbn: String, title: String, author: String) {
        self.isbn = isbn
        self.title = title
        self.author = author
        self.finished = false
        self.pages = 0
    }
}

@Model
final class Note {
    var text: String
    var page: Int32
    var book: Book?

    init(text: String, page: Int32) {
        self.text = text
        self.page = page
    }
}
