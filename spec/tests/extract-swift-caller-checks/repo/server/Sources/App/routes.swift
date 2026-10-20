import Vapor

let audience = Environment.get("TOKEN_AUDIENCE") ?? ""
let callers = (Environment.get("VOID_CALLERS") ?? "").split(separator: ",").map(String.init)

func routes(_ app: Application) throws {
    let api = app.grouped(RequireAudience(audience))
    let reading = api.grouped(RequireClients(["reporting", "archive"]), Guarded("invoices.read"))
    reading.get("invoices", use: listInvoices)
    let admin = api.grouped(RequireCaller(["middleware"]), RequireApplication("admin-web"), Audit(), Guarded("invoices.delete"))
    admin.delete("invoices", ":invoiceId", use: deleteInvoice)
    let voiding = api.grouped(RequireCaller(callers), Guarded("invoices.delete"))
    voiding.post("invoices", ":invoiceId", "void", use: voidInvoice)
    let archiving = api.grouped(Guarded("invoices.delete"))
    archiving.post("invoices", ":invoiceId", "archive", use: archiveInvoice)
    let sending = api.grouped(RequireClients(["reporting", "mailer"]), RequireClient("mailer", "only the mailer sends invoices"), Guarded("invoices.read"))
    sending.post("invoices", ":invoiceId", "send", use: sendInvoice)
}

func listInvoices(req: Request) async throws -> HTTPStatus { .ok }

func deleteInvoice(req: Request) async throws -> HTTPStatus {
    _ = req.parameters.get("invoiceId")
    return .noContent
}

func voidInvoice(req: Request) async throws -> HTTPStatus { .ok }

func archiveInvoice(req: Request) async throws -> HTTPStatus { .ok }

func sendInvoice(req: Request) async throws -> HTTPStatus { .ok }
