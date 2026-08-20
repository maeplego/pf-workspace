import { unstable_noStore as noStore } from "next/cache";
import Link from "next/link";
import { redirect } from "next/navigation";

import { apiFetchForPage, createBoard, createInvitation, createWorkspace, resendInvitation, revokeInvitation, searchOrgMembers, syncMemberDisplayName, unarchiveBoard, updateInvitationPolicy } from "./actions";
import { InviteEmailField } from "./InviteEmailField";
import { ambiguousDisplayNames, memberLabel } from "../lib/display";
import { oidcEnabled } from "../lib/oidc/env";
import { getWorkspaceSession } from "../lib/session";

type Workspace = { id: string; name: string };
type Board = { id: string; name: string; workspaceId: string };
type Member = { sub: string; role: string; displayName?: string };
type Invitation = {
  id: string;
  role: string;
  maxUses: number;
  useCount: number;
  expiresAt: string;
  invitedEmail?: string;
  revokedAt?: string | null;
};
type OrgMember = { sub: string; role: string; email?: string; displayName?: string };

function invitationStatus(inv: Invitation): string {
  if (inv.revokedAt) return "revoked";
  if (inv.useCount >= inv.maxUses) return "used";
  if (new Date(inv.expiresAt) <= new Date()) return "expired";
  return "active";
}

function homeHref(devUser?: string) {
  return devUser ? `/?user=${encodeURIComponent(devUser)}` : "/";
}

export default async function HomePage({
  searchParams,
}: {
  searchParams: Promise<{ user?: string; error?: string; createdInvite?: string; resentInvite?: string }>;
}) {
  noStore();
  const sp = await searchParams;
  const session = await getWorkspaceSession(sp.user);
  if (oidcEnabled() && !session) {
    redirect("/login");
  }

  const devUser = session!.devMode ? session!.sub : undefined;
  const displayName = session!.displayName || session!.sub;
  const wsPayload = (await apiFetchForPage("/v1/workspaces", devUser)) as { workspaces?: Workspace[] | null };
  const workspaces = wsPayload.workspaces ?? [];

  const boardsByWs: Record<string, Board[]> = {};
  const archivedByWs: Record<string, Board[]> = {};
  const membersByWs: Record<string, Member[]> = {};
  const invitationsByWs: Record<string, Invitation[]> = {};
  const orgMembersByWs: Record<string, OrgMember[]> = {};
  for (const ws of workspaces) {
    try {
      await syncMemberDisplayName(ws.id, displayName, devUser);
    } catch {
      // ignore if not a member yet
    }
    const data = (await apiFetchForPage(`/v1/workspaces/${ws.id}/boards`, devUser)) as {
      boards?: Board[] | null;
      archivedBoards?: Board[] | null;
    };
    boardsByWs[ws.id] = data.boards ?? [];
    archivedByWs[ws.id] = data.archivedBoards ?? [];
    const mem = (await apiFetchForPage(`/v1/workspaces/${ws.id}/members`, devUser)) as {
      members?: Member[] | null;
    };
    membersByWs[ws.id] = mem.members ?? [];
    const isOwner = (mem.members ?? []).some((m) => m.sub === session!.sub && m.role === "owner");
    if (isOwner) {
      try {
        const invPayload = (await apiFetchForPage(`/v1/workspaces/${ws.id}/invitations`, devUser)) as {
          invitations?: Invitation[] | null;
        };
        invitationsByWs[ws.id] = invPayload.invitations ?? [];
      } catch {
        invitationsByWs[ws.id] = [];
      }
      try {
        const membersPayload = await searchOrgMembers("", devUser);
        orgMembersByWs[ws.id] = membersPayload.members ?? [];
      } catch {
        orgMembersByWs[ws.id] = [];
      }
    }
  }

  async function createWorkspaceAction(formData: FormData) {
    "use server";
    await createWorkspace(formData, devUser);
  }

  async function createBoardAction(formData: FormData) {
    "use server";
    const wsId = String(formData.get("workspaceId") || "");
    if (!wsId) return;
    await createBoard(wsId, formData, devUser);
  }

  async function createInvitationAction(formData: FormData) {
    "use server";
    const wsId = String(formData.get("workspaceId") || "");
    if (!wsId) return;
    const token = await createInvitation(wsId, formData, devUser);
    const q = new URLSearchParams();
    if (devUser) q.set("user", devUser);
    q.set("createdInvite", token);
    redirect(`/?${q.toString()}`);
  }

  async function revokeInvitationAction(formData: FormData) {
    "use server";
    const wsId = String(formData.get("workspaceId") || "");
    const invitationId = String(formData.get("invitationId") || "");
    if (!wsId || !invitationId) return;
    await revokeInvitation(wsId, invitationId, devUser);
  }

  async function resendInvitationAction(formData: FormData) {
    "use server";
    const wsId = String(formData.get("workspaceId") || "");
    const invitationId = String(formData.get("invitationId") || "");
    if (!wsId || !invitationId) return;
    const token = await resendInvitation(wsId, invitationId, devUser);
    const q = new URLSearchParams();
    if (devUser) q.set("user", devUser);
    q.set("resentInvite", token);
    redirect(`/?${q.toString()}`);
  }

  async function updateInvitationPolicyAction(formData: FormData) {
    "use server";
    const wsId = String(formData.get("workspaceId") || "");
    const invitationId = String(formData.get("invitationId") || "");
    if (!wsId || !invitationId) return;
    await updateInvitationPolicy(wsId, invitationId, formData, devUser);
    redirect(devUser ? `/?user=${encodeURIComponent(devUser)}` : "/");
  }

  return (
    <>
      <section className="hero">
        <h1 className="page-title">ワークスペース</h1>
        <p className="page-lead row">
          <span>
            ユーザー: <strong>{session!.displayName || session!.sub}</strong>
          </span>
          {session!.devMode ? (
            <>
              <Link href={homeHref("demo-user-a")}>A</Link>
              <Link href={homeHref("demo-user-b")}>B</Link>
              <span className="pill">開発モード</span>
            </>
          ) : (
            <form action="/logout" method="post">
              <button type="submit" className="btn btn-secondary">
                ログアウト
              </button>
            </form>
          )}
        </p>
        {sp.error ? <p className="error">ログインエラー: {sp.error}</p> : null}
        {sp.createdInvite ? (
          <p className="muted">
            招待リンクを発行しました:{" "}
            <code style={{ userSelect: "all" }}>
              {`${process.env.WORKSPACE_PUBLIC_BASE_URL || "http://localhost:3005"}/join/${sp.createdInvite}`}
            </code>
          </p>
        ) : null}
        {sp.resentInvite ? (
          <p className="muted">
            招待リンクを再発行しました:{" "}
            <code style={{ userSelect: "all" }}>
              {`${process.env.WORKSPACE_PUBLIC_BASE_URL || "http://localhost:3005"}/join/${sp.resentInvite}`}
            </code>
          </p>
        ) : null}
      </section>

      <section className="card" style={{ marginBottom: "1.5rem" }}>
        <h2 style={{ marginTop: 0 }}>ワークスペースを作成</h2>
        <form action={createWorkspaceAction} className="row">
          <input name="name" placeholder="Team name" required style={{ flex: 1 }} />
          <button type="submit" className="btn">
            作成
          </button>
        </form>
      </section>

      {workspaces.length === 0 ? (
        <p className="muted">ワークスペースがありません。上のフォームから作成してください。</p>
      ) : (
        workspaces.map((ws) => (
          <section key={ws.id} className="card" style={{ marginBottom: "1rem" }}>
            <h2 style={{ marginTop: 0 }}>{ws.name}</h2>
            <p>
              <Link href={devUser ? `/wiki/${ws.id}?user=${devUser}` : `/wiki/${ws.id}`}>Wiki</Link>
              {" · "}
              <Link href={devUser ? `/docs/${ws.id}?user=${devUser}` : `/docs/${ws.id}`}>Docs</Link>
              {" · "}
              <Link href={devUser ? `/chat/${ws.id}?user=${devUser}` : `/chat/${ws.id}`}>Chat</Link>
              {" · "}
              <Link href={devUser ? `/search/${ws.id}?user=${devUser}` : `/search/${ws.id}`}>検索</Link>
            </p>
            <form action={devUser ? `/search/${ws.id}` : `/search/${ws.id}`} method="get" className="row" style={{ marginBottom: "0.75rem" }}>
              {devUser ? <input type="hidden" name="user" value={devUser} /> : null}
              <input name="q" placeholder="横断検索" required style={{ flex: 1 }} />
              <button type="submit" className="btn btn-secondary">
                検索
              </button>
            </form>
            <ul style={{ paddingLeft: "1.2rem" }}>
              {(boardsByWs[ws.id] || []).map((b) => (
                <li key={b.id}>
                  <Link href={devUser ? `/boards/${b.id}?user=${devUser}` : `/boards/${b.id}`}>{b.name}</Link>
                  {" · "}
                  <Link href={devUser ? `/boards/${b.id}/sprints?user=${devUser}` : `/boards/${b.id}/sprints`}>スプリント</Link>
                </li>
              ))}
            </ul>
            <form action={createBoardAction} className="row" style={{ marginTop: "0.75rem" }}>
              <input type="hidden" name="workspaceId" value={ws.id} />
              <input name="name" placeholder="Board name" required style={{ flex: 1 }} />
              <button type="submit" className="btn btn-secondary">
                ボード追加
              </button>
            </form>
            {(archivedByWs[ws.id] || []).length > 0 ? (
              <div className="muted" style={{ marginTop: "0.75rem" }}>
                <p style={{ marginBottom: "0.35rem" }}>アーカイブしたボード</p>
                {(archivedByWs[ws.id] || []).map((b) => (
                  <form key={b.id} action={unarchiveBoard.bind(null, b.id, devUser)} className="row" style={{ marginBottom: "0.35rem" }}>
                    <span style={{ flex: 1 }}>{b.name}</span>
                    <button type="submit" className="btn btn-secondary">
                      戻す
                    </button>
                  </form>
                ))}
              </div>
            ) : null}
            <h3 style={{ fontSize: "0.95rem" }}>メンバー</h3>
            <ul style={{ paddingLeft: "1.2rem" }}>
              {(membersByWs[ws.id] || []).map((m) => {
                const dupes = ambiguousDisplayNames(membersByWs[ws.id] || []);
                const label = memberLabel(m, dupes);
                return (
                  <li key={m.sub}>
                    <Link href={devUser ? `/members/${ws.id}/${encodeURIComponent(m.sub)}?user=${devUser}` : `/members/${ws.id}/${encodeURIComponent(m.sub)}`}>
                      {label}
                    </Link>{" "}
                    <span className="muted">({m.role})</span>
                  </li>
                );
              })}
            </ul>
            <p className="muted">
              guest は published の Wiki 閲覧とボード参照だけです。メンバー参加は招待リンク経由になり、sub の手入力は不要です。
            </p>
            {(membersByWs[ws.id] || []).some((m) => m.sub === session!.sub && m.role === "owner") ? (
              <form action={createInvitationAction} className="row" style={{ marginTop: "0.5rem" }}>
                <input type="hidden" name="workspaceId" value={ws.id} />
                <select name="role" defaultValue="member" style={{ width: "auto" }}>
                  <option value="member">member</option>
                  <option value="guest">guest</option>
                </select>
                <input name="maxUses" type="number" min={1} max={100} defaultValue={1} style={{ width: 100 }} />
                <input name="ttlHours" type="number" min={1} max={336} defaultValue={72} style={{ width: 100 }} />
                <InviteEmailField listId={`org-members-${ws.id}`} members={orgMembersByWs[ws.id] || []} />
                <button type="submit" className="btn btn-secondary">
                  招待リンク発行
                </button>
              </form>
            ) : null}
            {(invitationsByWs[ws.id] || []).length > 0 ? (
              <div style={{ marginTop: "0.75rem" }}>
                <h3 style={{ fontSize: "0.95rem" }}>招待リンク</h3>
                <ul style={{ paddingLeft: "1.2rem" }}>
                  {(invitationsByWs[ws.id] || []).map((inv) => {
                    const status = invitationStatus(inv);
                    return (
                      <li key={inv.id} style={{ marginBottom: "0.35rem" }}>
                        <span>
                          {inv.role} · {inv.useCount}/{inv.maxUses} · {status}
                          {inv.invitedEmail ? ` · ${inv.invitedEmail}` : ""}
                        </span>
                        {status === "active" ? (
                          <>
                            <form action={updateInvitationPolicyAction} className="row" style={{ display: "flex", flexWrap: "wrap", gap: "0.35rem", marginTop: "0.35rem" }}>
                              <input type="hidden" name="workspaceId" value={ws.id} />
                              <input type="hidden" name="invitationId" value={inv.id} />
                              <select name="role" defaultValue={inv.role} style={{ width: "auto" }}>
                                <option value="member">member</option>
                                <option value="guest">guest</option>
                              </select>
                              <input name="maxUses" type="number" min={Math.max(1, inv.useCount)} max={100} defaultValue={inv.maxUses} style={{ width: 80 }} />
                              <input name="ttlHours" type="number" min={1} max={336} defaultValue={72} style={{ width: 80 }} title="残り有効時間（時間）" />
                              <input name="invitedEmail" type="email" defaultValue={inv.invitedEmail || ""} placeholder="招待先メール" style={{ width: 180 }} />
                              <button type="submit" className="btn btn-secondary">
                                条件更新
                              </button>
                            </form>
                            <form action={resendInvitationAction} className="row" style={{ display: "inline-flex", marginLeft: "0.5rem" }}>
                              <input type="hidden" name="workspaceId" value={ws.id} />
                              <input type="hidden" name="invitationId" value={inv.id} />
                              <button type="submit" className="btn btn-secondary">
                                再送
                              </button>
                            </form>
                            <form action={revokeInvitationAction} className="row" style={{ display: "inline-flex", marginLeft: "0.35rem" }}>
                              <input type="hidden" name="workspaceId" value={ws.id} />
                              <input type="hidden" name="invitationId" value={inv.id} />
                              <button type="submit" className="btn btn-secondary">
                                取り消し
                              </button>
                            </form>
                          </>
                        ) : null}
                      </li>
                    );
                  })}
                </ul>
              </div>
            ) : null}
          </section>
        ))
      )}
    </>
  );
}
