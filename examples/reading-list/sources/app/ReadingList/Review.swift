import Foundation

struct BookDraft: Codable {
    var isbn: String
    var title: String
}

struct Review: Codable, Identifiable {
    let id: UUID
    let isbn: String
    let stars: UInt8
    let text: String
    let writtenAt: Date

    enum CodingKeys: String, CodingKey {
        case id, isbn, stars, text
        case writtenAt = "written_at"
    }
}
