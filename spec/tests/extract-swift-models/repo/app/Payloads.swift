import Foundation

struct AuthorPayload: Codable {
    let handle: String
    let displayName: String
    let joinedAt: Date
    let website: URL?
    let score: Double
    let ratio: CGFloat
    let nickname: String
    var internalNote: String = ""

    enum CodingKeys: String, CodingKey {
        case handle
        case displayName = "display_name"
        case joinedAt = "joined_at"
        case website, score, ratio
        case nickname = "nick"
    }
}

struct Summary: Decodable {
    var bookCount: Int32
    var topAuthor: AuthorPayload
}

enum ShelfKind: String, Codable {
    case fiction
    case nonFiction = "non_fiction"
    case kids = "Kids"
}

enum Origin: String, Codable {
    case local, imported
}

struct Outer {
    struct Inner: Codable {
        var value: String
    }
}
