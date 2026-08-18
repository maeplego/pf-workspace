import { NextResponse } from "next/server";

export async function GET() {
  return NextResponse.redirect(new URL("/", process.env.OIDC_REDIRECT_URI || "http://localhost:3006"));
}
