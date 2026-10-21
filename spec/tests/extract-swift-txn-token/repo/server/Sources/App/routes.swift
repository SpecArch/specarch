import Vapor

func routes(_ app: Application) throws {
    app.get("invoices", use: listInvoices)
    app.post("invoices", ":invoiceId", "void", use: voidInvoice)
    app.post("invoices", ":invoiceId", "send", use: sendInvoice)
}

func listInvoices(req: Request) async throws -> HTTPStatus {
    _ = req.headers.first(name: "Accept-Language")
    return .ok
}

func voidInvoice(req: Request) async throws -> HTTPStatus {
    guard req.headers.first(name: "Txn-Token") != nil else { throw Abort(.unauthorized) }
    return .ok
}

func sendInvoice(req: Request) async throws -> HTTPStatus {
    _ = req.headers["txn-token"]
    return .ok
}
