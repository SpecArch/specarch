import Vapor

struct BookRecord: Content {
    var isbn: String
    var title: String
    var pageCount: Int32?
}

struct ReviewRecord: Content {
    var id: UUID
    var isbn: String
    var stars: UInt8
    var text: String
}
