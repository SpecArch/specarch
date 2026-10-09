import Vapor

/// Lets a request through when its caller holds the permission.
struct RequirePermission: AsyncMiddleware {
    let permission: String

    init(_ permission: String) {
        self.permission = permission
    }

    func respond(to request: Request, chainingTo next: AsyncResponder) async throws -> Response {
        guard request.headers.first(name: "X-Permissions")?.contains(permission) == true else {
            throw Abort(.forbidden)
        }
        return try await next.respond(to: request)
    }
}
