import { forward } from "@/server/forward";
import { server } from "@/server/settings";

/** recordVisits: POST /members/{memberId}/visits, forwarded to the service with the session's token. */
export function POST(request: Request): Promise<Response> {
  return forward(request, server);
}
