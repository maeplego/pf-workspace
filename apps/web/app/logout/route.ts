import { NextRequest, NextResponse } from "next/server";

import { clearOn, readCookie, setOn } from "../../lib/oidc/cookies";
import { endSessionEndpoint } from "../../lib/oidc/discovery";
import { clientId, oidcEnabled, postLogoutRedirectUri, publicOrigin } from "../../lib/oidc/env";
import { randomString } from "../../lib/oidc/pkce";

export async function POST(req: NextRequest) {
  if (!oidcEnabled()) {
    return NextResponse.redirect(new URL("/", publicOrigin(req)), { status: 303 });
  }
  const idToken = await readCookie("rp_id");
  const state = randomString(16);
  const q = new URLSearchParams({
    client_id: clientId(),
    post_logout_redirect_uri: postLogoutRedirectUri(),
    state,
  });
  if (idToken) {
    q.set("id_token_hint", idToken);
  }
  const endSession = await endSessionEndpoint();
  if (!endSession) {
    const res = NextResponse.redirect(new URL("/logged-out", publicOrigin(req)), { status: 303 });
    clearOn(res, "rp_access");
    clearOn(res, "rp_id");
    clearOn(res, "rp_refresh");
    clearOn(res, "rp_active_org");
    return res;
  }
  const res = NextResponse.redirect(`${endSession}?${q.toString()}`, { status: 303 });
  setOn(res, "rp_logout_state", state, 600);
  return res;
}
