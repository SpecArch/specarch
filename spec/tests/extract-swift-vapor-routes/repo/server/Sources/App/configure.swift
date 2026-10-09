import Vapor

public func configure(_ app: Application) async throws {
    let password = Environment.get("SHELF_DB_PASSWORD") ?? ""
    app.logger.info("database password set: \(!password.isEmpty)")
    try routes(app)
}
