import { unstable_noStore as noStore } from "next/cache";
import Link from "next/link";
import { redirect } from "next/navigation";

import { apiFetchForPage } from "../../actions";
import { oidcEnabled } from "../../../lib/oidc/env";
import { getWorkspaceSession } from "../../../lib/session";

type Hit = {
  type: "page" | "document" | "card" | "message";
  id: string;
  title: string;
  snippet: string;
  hrefHints: Record<string, string>;
};

function withUser(path: string, devUser?: string) {
  if (!devUser) return path;
  const join = path.includes("?") ? "&" : "?";
  return `${path}${join}user=${encodeURIComponent(devUser)}`;
}

function hitHref(workspaceId: string, hit: Hit, devUser?: string) {
  switch (hit.type) {
    case "page":
      return withUser(`/wiki/${workspaceId}/pages/${hit.hrefHints.pageId || hit.id}`, devUser);
    case "document":
      return withUser(`/docs/${workspaceId}/${hit.hrefHints.documentId || hit.id}`, devUser);
    case "card":
      return withUser(`/boards/${hit.hrefHints.boardId}`, devUser);
    case "message":
      return withUser(`/chat/${workspaceId}/${hit.hrefHints.channelId}`, devUser);
    default:
      return withUser(`/search/${workspaceId}`, devUser);
  }
}

export default async function SearchPage({
  params,
  searchParams,
}: {
  params: Promise<{ workspaceId: string }>;
  searchParams: Promise<{ user?: string; q?: string; types?: string }>;
}) {
  noStore();
  const { workspaceId } = await params;
  const sp = await searchParams;
  const session = await getWorkspaceSession(sp.user);
  if (oidcEnabled() && !session) {
    redirect("/login");
  }
  const devUser = session!.devMode ? session!.sub : undefined;
  const q = (sp.q || "").trim();
  const types = (sp.types || "").trim();
  const workspace = (await apiFetchForPage(`/v1/workspaces/${workspaceId}`, devUser)) as { name: string };
  let hits: Hit[] = [];
  let error = "";
  if (q) {
    const qs = new URLSearchParams({ q });
    if (types) qs.set("types", types);
    try {
      const data = (await apiFetchForPage(`/v1/workspaces/${workspaceId}/search?${qs.toString()}`, devUser)) as {
        hits: Hit[];
      };
      hits = data.hits || [];
    } catch {
      error = "検索に失敗しました（権限または入力）";
    }
  }

  return (
    <div className="shell">
      <p className="muted">
        <Link href={withUser("/", devUser)}>{workspace.name}</Link>
        {" · "}
        <Link href={withUser(`/wiki/${workspaceId}`, devUser)}>Wiki</Link>
        {" · "}
        <Link href={withUser(`/docs/${workspaceId}`, devUser)}>Docs</Link>
        {" · "}
        <Link href={withUser(`/chat/${workspaceId}`, devUser)}>Chat</Link>
      </p>
      <h1 style={{ marginTop: 0 }}>検索</h1>
      <form method="get" className="card-surface" style={{ display: "flex", gap: "0.5rem", marginBottom: "1rem" }}>
        {devUser ? <input type="hidden" name="user" value={devUser} /> : null}
        <input name="q" defaultValue={q} placeholder="ページ・カード・チャット" required style={{ flex: 1, padding: "0.5rem" }} />
        <button type="submit">検索</button>
      </form>
      {!q ? <p className="muted">キーワードを入力してください。</p> : null}
      {error ? <p style={{ color: "#bf2600" }}>{error}</p> : null}
      {q && !error ? (
        <ul style={{ listStyle: "none", padding: 0 }}>
          {hits.length === 0 ? <li className="muted">ヒットなし</li> : null}
          {hits.map((h) => (
            <li key={`${h.type}-${h.id}`} className="card-surface" style={{ marginBottom: "0.75rem" }}>
              <span className={`search-badge search-badge-${h.type}`}>{h.type}</span>{" "}
              <Link href={hitHref(workspaceId, h, devUser)}>{h.title || h.id}</Link>
              <p className="muted" style={{ margin: "0.35rem 0 0" }}>
                {h.snippet}
              </p>
            </li>
          ))}
        </ul>
      ) : null}
    </div>
  );
}
