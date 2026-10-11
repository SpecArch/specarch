import { forward } from "@/server/forward";
import { server } from "@/server/settings";

/** suspendMember: POST /members/{memberId}/suspend, forwarded to the service with the session's token. */
export function POST(request: Request): Promise<Response> {
  return forward(request, server);
}
