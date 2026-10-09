import Vapor

struct ShelfBody: Content {
    var name: String
    var capacity: Int16
}

struct ItemBody: Content {
    var title: String
}

struct Guarded: AsyncMiddleware {
    let permission: String

    init(_ permission: String) {
        self.permission = permission
    }

    func respond(to request: Request, chainingTo next: AsyncResponder) async throws -> Response {
        try await next.respond(to: request)
    }
}

struct ItemController: RouteCollection {
    func boot(routes: RoutesBuilder) throws {
        let items = routes.grouped("items")
        items.post(use: create)
        items.group(":itemId") { item in
            item.get(use: show)
        }
    }

    func create(req: Request) async throws -> HTTPStatus {
        _ = try req.content.decode([ItemBody].self)
        return .created
    }

    func show(req: Request) async throws -> String {
        req.parameters.get("itemID") ?? ""
    }
}

struct Orphan: RouteCollection {
    func boot(routes: RoutesBuilder) throws {
        routes.get("orphan", use: { _ in "" })
    }
}
