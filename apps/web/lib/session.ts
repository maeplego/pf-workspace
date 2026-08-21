import { pickOrgId, pickOrganizations } from "./oidc/claims";
import { readCookie } from "./oidc/cookies";
import { userinfoEndpoint } from "./oidc/discovery";
import { oidcEnabled } from "./oidc/env";

export type OrgMembership = {
  orgId: string;
  orgName: string;
  role: string;
};

export type WorkspaceSession = {
  sub: string;
  email?: string;
  accessToken?: string;
  displayName?: string;
  orgId?: string;
  organizations?: OrgMembership[];
  devMode: boolean;
};

export async function getWorkspaceSession(devUser?: string): Promise<WorkspaceSession | null> {
  if (!oidcEnabled()) {
    const saved = (await readCookie("dev_org")) || "";
    const organizations = [
      { orgId: "org-demo-a", orgName: "Demo Org A", role: "owner" },
      { orgId: "org-demo-b", orgName: "Demo Org B", role: "member" },
    ];
    const orgId = saved || organizations[0].orgId;
    return {
      sub: devUser?.trim() || "demo-user-a",
      email: undefined,
      orgId,
      organizations,
      devMode: true,
    };
  }
  const access = await readCookie("rp_access");
  if (!access) return null;
  const userinfo = await userinfoEndpoint();
  const res = await fetch(userinfo, {
    headers: { Authorization: `Bearer ${access}` },
    cache: "no-store",
  });
  if (!res.ok) return null;
  const ui = (await res.json()) as Record<string, unknown>;
  if (typeof ui.sub !== "string" || !ui.sub) return null;
  const organizations = pickOrganizations(ui);
  const cookieOrg = (await readCookie("rp_active_org")) || "";
  const fromCookie =
    cookieOrg && organizations.some((o) => o.orgId === cookieOrg) ? cookieOrg : "";
  const fromClaim = pickOrgId(ui) || organizations[0]?.orgId;
  return {
    sub: ui.sub,
    email: typeof ui.email === "string" ? ui.email.toLowerCase().trim() : undefined,
    accessToken: access,
    displayName:
      (typeof ui.name === "string" && ui.name) ||
      (typeof ui.email === "string" && ui.email) ||
      ui.sub,
    orgId: fromCookie || fromClaim,
    organizations,
    devMode: false,
  };
}

export async function requireWorkspaceSession(devUser?: string): Promise<WorkspaceSession> {
  const session = await getWorkspaceSession(devUser);
  if (!session) throw new Error("unauthorized");
  return session;
}
