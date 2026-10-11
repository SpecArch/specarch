import { forward } from "@/server/forward";
import { server } from "@/server/settings";

/** listMembers: GET /members, forwarded to the service with the session's token. */
export function GET(request: Request): Promise<Response> {
  return forward(request, server);
}

/** createMember: POST /members, forwarded to the service with the session's token. */
export function POST(request: Request): Promise<Response> {
  return forward(request, server);
}
