import { readCookie } from "./oidc/cookies";
import { internalBase, oidcEnabled } from "./oidc/env";

export type WorkspaceSession = {
  sub: string;
  email?: string;
  accessToken?: string;
  displayName?: string;
  devMode: boolean;
};

export async function getWorkspaceSession(devUser?: string): Promise<WorkspaceSession | null> {
  if (!oidcEnabled()) {
    return { sub: devUser?.trim() || "demo-user-a", devMode: true };
  }
  const access = await readCookie("rp_access");
  if (!access) return null;
  const res = await fetch(`${internalBase()}/userinfo`, {
    headers: { Authorization: `Bearer ${access}` },
    cache: "no-store",
  });
  if (!res.ok) return null;
  const ui = (await res.json()) as { sub?: string; name?: string; email?: string };
  if (!ui.sub) return null;
  return {
    sub: ui.sub,
    email: ui.email ? ui.email.toLowerCase().trim() : undefined,
    accessToken: access,
    displayName: ui.name || ui.email || ui.sub,
    devMode: false,
  };
}

export async function requireWorkspaceSession(devUser?: string): Promise<WorkspaceSession> {
  const session = await getWorkspaceSession(devUser);
  if (!session) throw new Error("unauthorized");
  return session;
}
