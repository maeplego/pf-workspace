import { unstable_noStore as noStore } from "next/cache";
import Link from "next/link";
import { redirect } from "next/navigation";

import { apiFetchForPage } from "../../../actions";
import { ambiguousDisplayNames, memberLabel } from "../../../../lib/display";
import { oidcEnabled } from "../../../../lib/oidc/env";
import { getWorkspaceSession } from "../../../../lib/session";

function withUser(path: string, devUser?: string) {
  if (!devUser) return path;
  const join = path.includes("?") ? "&" : "?";
  return `${path}${join}user=${encodeURIComponent(devUser)}`;
}

export default async function MemberProfilePage({
  params,
  searchParams,
}: {
  params: Promise<{ workspaceId: string; sub: string }>;
  searchParams: Promise<{ user?: string }>;
}) {
  noStore();
  const { workspaceId, sub } = await params;
  const sp = await searchParams;
  const session = await getWorkspaceSession(sp.user);
  if (oidcEnabled() && !session) {
    redirect("/login");
  }
  const devUser = session!.devMode ? session!.sub : undefined;
  const workspace = (await apiFetchForPage(`/v1/workspaces/${workspaceId}`, devUser)) as { name: string };
  const member = (await apiFetchForPage(`/v1/workspaces/${workspaceId}/members/${encodeURIComponent(sub)}`, devUser)) as {
    sub: string;
    displayName?: string;
    role: string;
    joinedAt: string;
  };
  const { members } = (await apiFetchForPage(`/v1/workspaces/${workspaceId}/members`, devUser)) as {
    members: { sub: string; displayName?: string }[];
  };
  const dupes = ambiguousDisplayNames(members || []);
  const label = memberLabel(member, dupes);

  return (
    <div className="shell">
      <p className="muted">
        <Link href={withUser("/", devUser)}>ホーム</Link>
        {" · "}
        <Link href={withUser("/", devUser)}>{workspace.name}</Link>
      </p>
      <h1 style={{ marginTop: 0 }}>{label}</h1>
      <div className="card-surface">
        <p>
          <span className="muted">ロール</span> {member.role}
        </p>
        <p>
          <span className="muted">参加</span> {new Date(member.joinedAt).toLocaleString("ja-JP")}
        </p>
        <p style={{ wordBreak: "break-all" }}>
          <span className="muted">ユーザー ID</span> {member.sub}
        </p>
        <p className="muted" style={{ marginBottom: 0 }}>
          表示名が重複するときは、ユーザー ID の先頭で区別できます。プロフィールはこのワークスペースのメンバーだけが閲覧できます。
        </p>
      </div>
    </div>
  );
}
