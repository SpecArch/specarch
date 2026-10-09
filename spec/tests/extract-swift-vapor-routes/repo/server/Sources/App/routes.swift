import Vapor

func routes(_ app: Application) throws {
    app.get("health") { _ in "ok" }
    app.on(.GET, "status", use: status)
    app.on(.OPTIONS, "status", use: status)

    let api = app.grouped("api", "v1")
    api.get("shelves", ":shelfId", use: showShelf)
    api.get("shelves", ":shelfId", use: showShelf)

    let admin = api.grouped(Guarded("shelves.admin"))
    admin.delete("shelves", ":shelfId", use: deleteShelf)
    let both = admin.grouped(Guarded("shelves.write"))
    both.put("shelves", ":shelfId", use: updateShelf)
    let named = api.grouped(Guarded(permissionName))
    named.patch("shelves", ":shelfId", use: patchShelf)

    api.get("files", "**", use: files)
    api.get("any", "*", use: files)
    api.get(PathComponent(stringLiteral: "x"), use: files)
    if app.environment == .development {
        app.get("debug", use: files)
    }

    try app.register(collection: ItemController())
    try api.register(collection: ItemController())
}

let permissionName = "shelves.patch"

func helper(_ routes: RoutesBuilder) {
    routes.get("helped", use: files)
}

func status(req: Request) async throws -> String { "up" }

func showShelf(req: Request) async throws -> ShelfBody {
    let id = req.parameters.get("shelfId")
    return ShelfBody(name: id ?? "", capacity: 0)
}

func deleteShelf(req: Request) async throws -> HTTPStatus {
    _ = req.parameters.get("id")
    return .noContent
}

func updateShelf(req: Request) async throws -> ShelfBody {
    try req.content.decode(ShelfBody.self)
}

func patchShelf(req: Request) async throws -> HTTPStatus { .ok }

func files(req: Request) async throws -> String { "" }
