import Vapor

public func configure(_ app: Application) async throws {
    guard let password = Environment.get("READING_LIST_DATABASE_PASSWORD") else {
        throw Abort(.internalServerError, reason: "No database password")
    }
    app.storage[DatabasePasswordKey.self] = password
    app.middleware.use(ErrorMiddleware.default(environment: app.environment))
    try routes(app)
}

struct DatabasePasswordKey: StorageKey {
    typealias Value = String
}
