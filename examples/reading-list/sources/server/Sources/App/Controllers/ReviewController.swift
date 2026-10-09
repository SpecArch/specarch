import Vapor

struct ReviewController: RouteCollection {
    func boot(routes: RoutesBuilder) throws {
        let reviews = routes.grouped("reviews")
        reviews.post(use: create)
        reviews.group(":reviewId") { review in
            review.delete(use: delete)
        }
    }

    func create(req: Request) async throws -> ReviewRecord {
        try req.content.decode(ReviewRecord.self)
    }

    func delete(req: Request) async throws -> HTTPStatus {
        _ = req.parameters.get("reviewID")
        return .noContent
    }
}
