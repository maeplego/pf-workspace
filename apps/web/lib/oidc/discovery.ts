import { internalBase } from "./env";

type Discovery = {
  authorization_endpoint?: string;
  token_endpoint?: string;
  userinfo_endpoint?: string;
  end_session_endpoint?: string;
  jwks_uri?: string;
};

let cached: { at: number; doc: Discovery } | null = null;

/** OpenID Provider Metadata (Auth0 / Entra / P01). Falls back to issuer-relative paths. */
export async function oidcDiscovery(): Promise<Discovery> {
  if (cached && Date.now() - cached.at < 10 * 60 * 1000) {
    return cached.doc;
  }
  const base = internalBase();
  try {
    const res = await fetch(`${base}/.well-known/openid-configuration`, { cache: "no-store" });
    if (res.ok) {
      const doc = (await res.json()) as Discovery;
      cached = { at: Date.now(), doc };
      return doc;
    }
  } catch {
    /* use fallbacks */
  }
  const doc: Discovery = {
    authorization_endpoint: `${base}/authorize`,
    token_endpoint: `${base}/token`,
    userinfo_endpoint: `${base}/userinfo`,
    end_session_endpoint: `${base}/end-session`,
    jwks_uri: `${base}/.well-known/jwks.json`,
  };
  cached = { at: Date.now(), doc };
  return doc;
}

export async function authorizationEndpoint(): Promise<string> {
  const d = await oidcDiscovery();
  return d.authorization_endpoint || `${internalBase()}/authorize`;
}

export async function tokenEndpoint(): Promise<string> {
  const d = await oidcDiscovery();
  return d.token_endpoint || `${internalBase()}/token`;
}

export async function userinfoEndpoint(): Promise<string> {
  const d = await oidcDiscovery();
  return d.userinfo_endpoint || `${internalBase()}/userinfo`;
}

export async function endSessionEndpoint(): Promise<string | undefined> {
  const d = await oidcDiscovery();
  return d.end_session_endpoint;
}
