import { forward } from "@/server/forward";
import { server } from "@/server/settings";

/** acceptInvitation: POST /invitations, forwarded to the service with the session's token. */
export function POST(request: Request): Promise<Response> {
  return forward(request, server);
}
