import { createRemoteJWKSet, jwtVerify } from "jose";

import { oidcDiscovery } from "./discovery";
import { clientId, internalBase, issuer } from "./env";

export async function verifyIdToken(idToken: string, nonce: string) {
  const iss = issuer();
  const discovery = await oidcDiscovery();
  const jwksURL = discovery.jwks_uri || `${internalBase()}/.well-known/jwks.json`;
  const JWKS = createRemoteJWKSet(new URL(jwksURL));
  const { payload } = await jwtVerify(idToken, JWKS, {
    issuer: iss,
    audience: clientId(),
  });
  if (payload.nonce !== nonce) {
    throw new Error("nonce mismatch");
  }
  return payload;
}
