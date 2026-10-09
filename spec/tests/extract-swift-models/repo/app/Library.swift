import CoreLocation
import Foundation
import SwiftData

@Model
final class Author {
    @Attribute(.unique) var handle: String
    var name: String
    var born: Int
    var rating: Float
    var balance: Decimal
    var visits: Int64
    var age: UInt8
    var photo: Data?
    var tags: [String]
    var extras: [String: String]
    @Relationship(deleteRule: .cascade) var books: [Book] = []
    @Transient var selected = false
    var label: String { name.uppercased() }
    static let sample = "sample"
    var shelf: ShelfKind
    var origin: Origin?
    var home: CLLocationCoordinate2D?

    init(handle: String, name: String) {
        self.handle = handle
        self.name = name
        self.born = 0
        self.rating = 0
        self.balance = 0
        self.visits = 0
        self.age = 0
        self.tags = []
        self.extras = [:]
        self.shelf = .fiction
    }
}

@Model
final class Book {
    var title: String
    var author: Author?
    var pages = 0
    var payload: AuthorPayload?

    init(title: String) {
        self.title = title
    }
}
