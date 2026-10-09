import Foundation

struct ReviewClient {
    private let base = URL(string: "https://api.reading-list.example")!

    private var apiKey: String {
        Bundle.main.object(forInfoDictionaryKey: "READING_LIST_API_KEY") as? String ?? ""
    }

    func reviews(isbn: String) async throws -> [Review] {
        let url = URL(string: "https://api.reading-list.example/books/\(isbn)/reviews")!
        let (data, _) = try await URLSession.shared.data(from: url)
        let decoder = JSONDecoder()
        decoder.dateDecodingStrategy = .iso8601
        return try decoder.decode([Review].self, from: data)
    }

    func add(_ draft: BookDraft) async throws {
        var request = URLRequest(url: URL(string: "https://api.reading-list.example/books")!)
        request.httpMethod = "POST"
        request.setValue(apiKey, forHTTPHeaderField: "X-Api-Key")
        request.httpBody = try JSONEncoder().encode(draft)
        _ = try await URLSession.shared.data(for: request)
    }

    func remove(isbn: String) async throws {
        var request = URLRequest(url: base.appendingPathComponent("books").appendingPathComponent(isbn))
        request.httpMethod = "DELETE"
        _ = try await URLSession.shared.data(for: request)
    }
}
