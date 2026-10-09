import { NextResponse, type NextRequest } from "next/server";

// Sends anyone without a desk session to the sign-in page before any
// desk page opens.
export function middleware(request: NextRequest) {
  if (!request.cookies.has("desk-session")) {
    return NextResponse.redirect(new URL("/sign-in", request.url));
  }
  return NextResponse.next();
}

export const config = { matcher: ["/loans/:path*", "/members/:path*"] };
