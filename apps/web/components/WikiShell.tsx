import Link from "next/link";
import { redirect } from "next/navigation";

import { apiFetchForPage, createPage, unarchivePage } from "../app/actions";
import { WikiTree, type PageNode } from "./WikiTree";

export async function WikiShell({
  workspaceId,
  currentId,
  sessionSub,
  devUser,
  children,
}: {
  workspaceId: string;
  currentId?: string;
  sessionSub: string;
  devUser?: string;
  children: React.ReactNode;
}) {
  const workspace = (await apiFetchForPage(`/v1/workspaces/${workspaceId}`, devUser)) as { id: string; name: string };
  const { tree, archived } = (await apiFetchForPage(`/v1/workspaces/${workspaceId}/pages/tree`, devUser)) as {
    tree: PageNode[];
    archived?: { id: string; title: string }[];
  };
  const { members } = (await apiFetchForPage(`/v1/workspaces/${workspaceId}/members`, devUser)) as {
    members: { sub: string; role: string }[];
  };
  const me = members.find((m) => m.sub === sessionSub);
  const canEdit = me?.role === "owner" || me?.role === "member";
  const q = devUser ? `?user=${encodeURIComponent(devUser)}` : "";

  async function createPageAction(formData: FormData) {
    "use server";
    const id = await createPage(workspaceId, formData, devUser);
    if (id) {
      redirect(`/wiki/${workspaceId}/pages/${id}${q}`);
    }
  }

  return (
    <div className="shell" style={{ display: "grid", gridTemplateColumns: "240px 1fr", gap: "1.5rem" }}>
      <aside className="card-surface">
        <p className="muted" style={{ marginTop: 0 }}>
          <Link href={`/${q}`}>{workspace.name}</Link>
        </p>
        <h2 style={{ margin: "0 0 0.75rem", fontSize: "1rem" }}>Wiki</h2>
        <p className="muted" style={{ marginTop: 0 }}>
          <Link href={devUser ? `/docs/${workspaceId}?user=${encodeURIComponent(devUser)}` : `/docs/${workspaceId}`}>
            共同編集ドキュメント
          </Link>
          {" · "}
          <Link href={devUser ? `/chat/${workspaceId}?user=${encodeURIComponent(devUser)}` : `/chat/${workspaceId}`}>
            Chat
          </Link>
          {" · "}
          <Link href={devUser ? `/search/${workspaceId}?user=${encodeURIComponent(devUser)}` : `/search/${workspaceId}`}>
            検索
          </Link>
        </p>
        <WikiTree nodes={tree || []} workspaceId={workspaceId} currentId={currentId} devUser={devUser} />
        {canEdit && (archived || []).length > 0 ? (
          <div style={{ marginTop: "1rem" }}>
            <p className="muted" style={{ marginBottom: "0.35rem" }}>
              アーカイブ
            </p>
            {(archived || []).map((p) => (
              <form key={p.id} action={unarchivePage.bind(null, workspaceId, p.id, devUser)} style={{ marginBottom: "0.35rem" }}>
                <button type="submit" className="btn btn-secondary">
                  {p.title} を戻す
                </button>
              </form>
            ))}
          </div>
        ) : null}
        {canEdit ? (
          <form action={createPageAction} style={{ marginTop: "1rem", display: "grid", gap: "0.35rem" }}>
            <input name="title" placeholder="New page" required style={{ padding: "0.35rem" }} />
            <select name="status" defaultValue="published" style={{ padding: "0.35rem" }}>
              <option value="published">published</option>
              <option value="draft">draft</option>
            </select>
            <button type="submit">ページ追加</button>
          </form>
        ) : (
          <p className="muted">閲覧のみ</p>
        )}
      </aside>
      <main>{children}</main>
    </div>
  );
}
