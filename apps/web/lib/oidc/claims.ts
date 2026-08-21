/** Claim key mapping for BYO IdPs (Auth0 / Entra / P01). Comma-separated alternates allowed. */

export function orgClaimKeys(): string[] {
  const raw = process.env.OIDC_ORG_CLAIM?.trim() || "org_id,orgId,organization_id,tid";
  return raw.split(",").map((s) => s.trim()).filter(Boolean);
}

export function orgsClaimKeys(): string[] {
  const raw = process.env.OIDC_ORGS_CLAIM?.trim() || "organizations,orgs,org_memberships";
  return raw.split(",").map((s) => s.trim()).filter(Boolean);
}

export type OrgMembership = {
  orgId: string;
  orgName: string;
  role: string;
};

function asRecord(v: unknown): Record<string, unknown> | null {
  return v && typeof v === "object" && !Array.isArray(v) ? (v as Record<string, unknown>) : null;
}

export function pickOrgId(claims: Record<string, unknown>): string | undefined {
  for (const key of orgClaimKeys()) {
    const v = claims[key];
    if (typeof v === "string" && v.trim()) return v.trim();
  }
  const orgs = pickOrganizations(claims);
  return orgs[0]?.orgId;
}

export function pickOrganizations(claims: Record<string, unknown>): OrgMembership[] {
  let raw: unknown;
  for (const key of orgsClaimKeys()) {
    if (claims[key] !== undefined) {
      raw = claims[key];
      break;
    }
  }
  if (!Array.isArray(raw)) return [];
  const out: OrgMembership[] = [];
  for (const item of raw) {
    if (typeof item === "string" && item.trim()) {
      out.push({ orgId: item.trim(), orgName: item.trim(), role: "member" });
      continue;
    }
    const o = asRecord(item);
    if (!o) continue;
    const orgId = [o.org_id, o.orgId, o.id].find((x) => typeof x === "string" && String(x).trim()) as
      | string
      | undefined;
    if (!orgId?.trim()) continue;
    const orgName = ([o.org_name, o.orgName, o.name, orgId].find(
      (x) => typeof x === "string" && String(x).trim(),
    ) || orgId) as string;
    const role = (typeof o.role === "string" && o.role) || "member";
    out.push({ orgId: orgId.trim(), orgName: String(orgName).trim(), role });
  }
  return out;
}
