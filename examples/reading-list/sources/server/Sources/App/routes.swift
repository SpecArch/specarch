import Vapor

func routes(_ app: Application) throws {
    app.get("health") { _ in "ok" }

    let books = app.grouped("books")
    books.get(use: listBooks)
    books.get(":isbn", "reviews", use: listReviews)

    let writers = books.grouped(RequirePermission("books.write"))
    writers.post(use: addBook)
    writers.delete(":isbn", use: removeBook)

    for format in ["csv", "json"] {
        app.get("export", "\(format)", use: exportBooks)
    }

    try app.register(collection: ReviewController())
}

func listBooks(req: Request) async throws -> [BookRecord] {
    []
}

func listReviews(req: Request) async throws -> [ReviewRecord] {
    let isbn = req.parameters.get("isbn")
    _ = isbn
    return []
}

func addBook(req: Request) async throws -> BookRecord {
    try req.content.decode(BookRecord.self)
}

func removeBook(req: Request) async throws -> HTTPStatus {
    _ = req.parameters.get("isbn")
    return .noContent
}

func exportBooks(req: Request) async throws -> String {
    ""
}
