import { forward } from "@/server/forward";
import { server } from "@/server/settings";

/** getSession: GET /sessions/current, forwarded to the service with the session's token. */
export function GET(request: Request): Promise<Response> {
  return forward(request, server);
}
