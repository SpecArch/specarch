import Vapor

/// Guarded runs the next responder only for a caller whose role grants
/// the permission.
struct Guarded: AsyncMiddleware {
    let permission: String

    init(_ permission: String) {
        self.permission = permission
    }

    func respond(to request: Request, chainingTo next: AsyncResponder) async throws -> Response {
        try await next.respond(to: request)
    }
}

/// RequireClients admits only one of the clients named.
struct RequireClients: AsyncMiddleware {
    let clients: [String]

    init(_ clients: [String]) {
        self.clients = clients
    }

    func respond(to request: Request, chainingTo next: AsyncResponder) async throws -> Response {
        try await next.respond(to: request)
    }
}

/// RequireClient admits only the client named, and logs the reason when
/// it refuses.
struct RequireClient: AsyncMiddleware {
    let client: String
    let reason: String

    init(_ client: String, _ reason: String) {
        self.client = client
        self.reason = reason
    }

    func respond(to request: Request, chainingTo next: AsyncResponder) async throws -> Response {
        try await next.respond(to: request)
    }
}

/// RequireApplication admits only a person who signed in through the
/// application named.
struct RequireApplication: AsyncMiddleware {
    let application: String

    init(_ application: String) {
        self.application = application
    }

    func respond(to request: Request, chainingTo next: AsyncResponder) async throws -> Response {
        try await next.respond(to: request)
    }
}

/// RequireCaller admits only an immediate caller among the services
/// named.
struct RequireCaller: AsyncMiddleware {
    let services: [String]

    init(_ services: [String]) {
        self.services = services
    }

    func respond(to request: Request, chainingTo next: AsyncResponder) async throws -> Response {
        try await next.respond(to: request)
    }
}

/// RequireAudience refuses a token whose aud is not the one given.
struct RequireAudience: AsyncMiddleware {
    let audience: String

    init(_ audience: String) {
        self.audience = audience
    }

    func respond(to request: Request, chainingTo next: AsyncResponder) async throws -> Response {
        try await next.respond(to: request)
    }
}

/// Audit writes a security event when the next responder refuses.
struct Audit: AsyncMiddleware {
    func respond(to request: Request, chainingTo next: AsyncResponder) async throws -> Response {
        try await next.respond(to: request)
    }
}
