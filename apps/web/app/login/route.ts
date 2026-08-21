import { NextRequest, NextResponse } from "next/server";

import { setOn } from "../../lib/oidc/cookies";
import { authorizationEndpoint } from "../../lib/oidc/discovery";
import { clientId, oidcEnabled, publicOrigin, redirectUri } from "../../lib/oidc/env";
import { randomString, s256 } from "../../lib/oidc/pkce";

export async function GET(req: NextRequest) {
  if (!oidcEnabled()) {
    return NextResponse.redirect(new URL("/", publicOrigin(req)));
  }
  const state = randomString(16);
  const nonce = randomString(16);
  const verifier = randomString(32);
  // Scope: omit "org" for Auth0/Entra unless custom API scopes exist; keep offline_access optional via env.
  const scope = process.env.OIDC_SCOPES?.trim() || "openid profile email org offline_access";
  const q = new URLSearchParams({
    response_type: "code",
    client_id: clientId(),
    redirect_uri: redirectUri(),
    scope,
    state,
    nonce,
    code_challenge: s256(verifier),
    code_challenge_method: "S256",
  });
  const authorize = await authorizationEndpoint();
  const res = NextResponse.redirect(`${authorize}?${q.toString()}`);
  setOn(res, "rp_state", state, 600);
  setOn(res, "rp_nonce", nonce, 600);
  setOn(res, "rp_verifier", verifier, 600);
  return res;
}
