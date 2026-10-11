import { forward } from "@/server/forward";
import { server } from "@/server/settings";

/** getMember: GET /members/{memberId}, forwarded to the service with the session's token. */
export function GET(request: Request): Promise<Response> {
  return forward(request, server);
}

/** updateMember: PUT /members/{memberId}, forwarded to the service with the session's token. */
export function PUT(request: Request): Promise<Response> {
  return forward(request, server);
}
