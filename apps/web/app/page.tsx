import { unstable_noStore as noStore } from "next/cache";
import Link from "next/link";
import { redirect } from "next/navigation";

import { apiFetchForPage, addMember, createBoard, createWorkspace } from "./actions";
import { oidcEnabled } from "../lib/oidc/env";
import { getWorkspaceSession } from "../lib/session";

type Workspace = { id: string; name: string };
type Board = { id: string; name: string; workspaceId: string };

function homeHref(devUser?: string) {
  return devUser ? `/?user=${encodeURIComponent(devUser)}` : "/";
}

export default async function HomePage({
  searchParams,
}: {
  searchParams: Promise<{ user?: string; error?: string }>;
}) {
  noStore();
  const sp = await searchParams;
  const session = await getWorkspaceSession(sp.user);
  if (oidcEnabled() && !session) {
    redirect("/login");
  }

  const devUser = session!.devMode ? session!.sub : undefined;
  const { workspaces } = (await apiFetchForPage("/v1/workspaces", devUser)) as { workspaces: Workspace[] };

  const boardsByWs: Record<string, Board[]> = {};
  for (const ws of workspaces) {
    const { boards } = (await apiFetchForPage(`/v1/workspaces/${ws.id}/boards`, devUser)) as {
      boards: Board[];
    };
    boardsByWs[ws.id] = boards;
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

  async function addMemberAction(formData: FormData) {
    "use server";
    const wsId = String(formData.get("workspaceId") || "");
    if (!wsId) return;
    await addMember(wsId, formData, devUser);
  }

  return (
    <div className="shell">
      <header style={{ marginBottom: "1.5rem" }}>
        <h1 style={{ margin: 0 }}>Workspace</h1>
        <p className="muted">
          ユーザー: <strong>{session!.displayName || session!.sub}</strong>
          {session!.devMode ? (
            <>
              {" "}
              · <Link href={homeHref("demo-user-a")}>A</Link> · <Link href={homeHref("demo-user-b")}>B</Link>
              <span>（開発モード）</span>
            </>
          ) : (
            <>
              {" "}
              · <form action="/logout" method="post" style={{ display: "inline" }}>
                  <button type="submit">ログアウト</button>
                </form>
            </>
          )}
        </p>
        {sp.error ? <p style={{ color: "#bf2600" }}>ログインエラー: {sp.error}</p> : null}
      </header>

      <section className="card-surface" style={{ marginBottom: "1.5rem" }}>
        <h2 style={{ marginTop: 0 }}>ワークスペースを作成</h2>
        <form action={createWorkspaceAction} style={{ display: "flex", gap: "0.5rem" }}>
          <input name="name" placeholder="Team name" required style={{ flex: 1, padding: "0.5rem" }} />
          <button type="submit">作成</button>
        </form>
      </section>

      {workspaces.length === 0 ? (
        <p className="muted">ワークスペースがありません。上のフォームから作成してください。</p>
      ) : (
        workspaces.map((ws) => (
          <section key={ws.id} className="card-surface" style={{ marginBottom: "1rem" }}>
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
            <form action={devUser ? `/search/${ws.id}` : `/search/${ws.id}`} method="get" style={{ display: "flex", gap: "0.5rem", marginBottom: "0.75rem" }}>
              {devUser ? <input type="hidden" name="user" value={devUser} /> : null}
              <input name="q" placeholder="横断検索" required style={{ flex: 1, padding: "0.5rem" }} />
              <button type="submit">検索</button>
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
            <form action={createBoardAction} style={{ display: "flex", gap: "0.5rem", marginTop: "0.75rem" }}>
              <input type="hidden" name="workspaceId" value={ws.id} />
              <input name="name" placeholder="Board name" style={{ flex: 1, padding: "0.5rem" }} />
              <button type="submit">ボード追加</button>
            </form>
            <form action={addMemberAction} style={{ display: "flex", gap: "0.5rem", marginTop: "0.5rem" }}>
              <input type="hidden" name="workspaceId" value={ws.id} />
              <input name="sub" placeholder="demo-user-b" style={{ flex: 1, padding: "0.5rem" }} />
              <select name="role" defaultValue="member">
                <option value="member">member</option>
                <option value="guest">guest</option>
              </select>
              <button type="submit">メンバー追加</button>
            </form>
          </section>
        ))
      )}
    </div>
  );
}
