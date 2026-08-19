import { NextRequest, NextResponse } from "next/server";

import { cookieKey, oidcEnabled } from "./lib/oidc/env";

const publicPaths = new Set(["/login", "/callback", "/logged-out", "/health", "/ready"]);

export function middleware(req: NextRequest) {
  if (!oidcEnabled()) {
    return NextResponse.next();
  }
  const path = req.nextUrl.pathname;
  if (publicPaths.has(path) || path.startsWith("/_next")) {
    return NextResponse.next();
  }
  if (!req.cookies.get(cookieKey("rp_access"))?.value) {
    return NextResponse.redirect(new URL("/login", req.url));
  }
  return NextResponse.next();
}

export const config = {
  matcher: ["/((?!_next/static|_next/image|favicon.ico).*)"],
};
