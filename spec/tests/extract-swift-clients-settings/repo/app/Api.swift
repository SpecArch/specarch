import Foundation

struct ShelfItem: Codable {
    let shelfCode: String
    let title: String
}

struct Api {
    private let base = URL(string: "https://api.example.test")!
    private let token = ProcessInfo.processInfo.environment["SHELF_TOKEN"]
    private let keyName = "SHELF_" + "MODE"

    private var timeout: String? {
        Bundle.main.object(forInfoDictionaryKey: "SHELF_TIMEOUT") as? String
    }

    private var region: String? {
        Bundle.main.infoDictionary?["SHELF_REGION"] as? String
    }

    func mode() -> Any? {
        Bundle.main.object(forInfoDictionaryKey: keyName)
    }

    func items(shelf: String) async throws -> [ShelfItem] {
        let url = URL(string: "https://api.example.test/shelves/\(shelf)/items")!
        let (data, _) = try await URLSession.shared.data(from: url)
        let decoder = JSONDecoder()
        decoder.keyDecodingStrategy = .convertFromSnakeCase
        return try decoder.decode([ShelfItem].self, from: data)
    }

    func search(text: String) async throws {
        _ = try await URLSession.shared.data(from: URL(string: "https://api.example.test/search?q=\(text)")!)
    }

    func create() async throws {
        var request = URLRequest(url: URL(string: "https://api.example.test/items")!)
        request.httpMethod = "POST"
        _ = try await URLSession.shared.data(for: request)
    }

    func change(method: String) async throws {
        var request = URLRequest(url: URL(string: "https://api.example.test/items")!)
        request.httpMethod = method
        _ = try await URLSession.shared.data(for: request)
    }

    func remove(id: String) async throws {
        var request = URLRequest(url: base.appendingPathComponent(id))
        request.httpMethod = "DELETE"
        _ = try await URLSession.shared.data(for: request)
    }

    func ping() async throws {
        _ = try await URLSession.shared.data(from: URL(string: "/ping")!)
    }

    func status() {
        URLSession.shared.dataTask(with: URL(string: "https://status.example.test/now")!).resume()
    }
}
