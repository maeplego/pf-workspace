import Link from "next/link";
import { redirect } from "next/navigation";

import { apiFetchForPage, createDocument } from "../app/actions";

type Doc = { id: string; title: string };

export async function DocsShell({
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
  const { documents } = (await apiFetchForPage(`/v1/workspaces/${workspaceId}/documents`, devUser)) as {
    documents: Doc[];
  };
  const { members } = (await apiFetchForPage(`/v1/workspaces/${workspaceId}/members`, devUser)) as {
    members: { sub: string; role: string }[];
  };
  const me = members.find((m) => m.sub === sessionSub);
  const canEdit = me?.role === "owner" || me?.role === "member";
  const q = devUser ? `?user=${encodeURIComponent(devUser)}` : "";

  async function createDocAction(formData: FormData) {
    "use server";
    const id = await createDocument(workspaceId, formData, devUser);
    if (id) {
      redirect(`/docs/${workspaceId}/${id}${q}`);
    }
  }

  return (
    <div className="shell" style={{ display: "grid", gridTemplateColumns: "240px 1fr", gap: "1.5rem" }}>
      <aside className="card-surface">
        <p className="muted" style={{ marginTop: 0 }}>
          <Link href={`/${q}`}>{workspace.name}</Link>
        </p>
        <h2 style={{ margin: "0 0 0.75rem", fontSize: "1rem" }}>Docs</h2>
        <p className="muted" style={{ marginTop: 0 }}>
          <Link href={`/wiki/${workspaceId}${q}`}>Wiki</Link>
          {" · "}
          <Link href={`/chat/${workspaceId}${q}`}>Chat</Link>
          {" · "}
          <Link href={`/search/${workspaceId}${q}`}>検索</Link>
        </p>
        <ul style={{ listStyle: "none", padding: 0, margin: 0 }}>
          {(documents || []).map((d) => (
            <li key={d.id} style={{ margin: "0.2rem 0" }}>
              <Link href={`/docs/${workspaceId}/${d.id}${q}`} style={{ fontWeight: d.id === currentId ? 700 : 400 }}>
                {d.title}
              </Link>
            </li>
          ))}
        </ul>
        {canEdit ? (
          <form action={createDocAction} style={{ marginTop: "1rem", display: "grid", gap: "0.35rem" }}>
            <input name="title" placeholder="New document" required style={{ padding: "0.35rem" }} />
            <button type="submit">ドキュメント追加</button>
          </form>
        ) : (
          <p className="muted">閲覧のみ</p>
        )}
      </aside>
      <main>{children}</main>
    </div>
  );
}
