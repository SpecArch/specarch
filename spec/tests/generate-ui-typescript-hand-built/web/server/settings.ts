import type { ServerSettings } from "./forward";

/**
 * Where the server routes answer and forward, how they carry the session's
 * token to the service, and the request headers they send on.
 */
export const server: ServerSettings = {
  routes: "/api",
  service: process.env.MEMBERS_SERVICE_URL ?? "",
  cookie: "members_session",
  header: "Authorization",
  scheme: "Bearer",
  headers: ["accept", "content-type", "idempotency-key"],
  wire: { page: "page", pageSize: "pageSize", items: "items", totalItems: "totalItems", totalPages: "totalPages" },
};
