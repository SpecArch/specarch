import { paged } from "@/server/forward";
import { server } from "@/server/settings";

/** listAllBranches: GET /branches/all, read whole from the service and answered a page at a time. */
export function GET(request: Request): Promise<Response> {
  return paged(request, server, { pageSize: 25, maximum: 100 });
}
