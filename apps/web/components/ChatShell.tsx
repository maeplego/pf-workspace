import Link from "next/link";
import { redirect } from "next/navigation";

import { apiFetchForPage, createChannel } from "../app/actions";

type Channel = { id: string; name: string; unreadCount?: number };

export async function ChatShell({
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
  const { channels } = (await apiFetchForPage(`/v1/workspaces/${workspaceId}/channels`, devUser)) as {
    channels: Channel[];
  };
  const { members } = (await apiFetchForPage(`/v1/workspaces/${workspaceId}/members`, devUser)) as {
    members: { sub: string; role: string }[];
  };
  const me = members.find((m) => m.sub === sessionSub);
  const canEdit = me?.role === "owner" || me?.role === "member";
  const q = devUser ? `?user=${encodeURIComponent(devUser)}` : "";

  async function createChannelAction(formData: FormData) {
    "use server";
    const id = await createChannel(workspaceId, formData, devUser);
    if (id) {
      redirect(`/chat/${workspaceId}/${id}${q}`);
    }
  }

  return (
    <div className="shell" style={{ display: "grid", gridTemplateColumns: "240px 1fr", gap: "1.5rem" }}>
      <aside className="card-surface">
        <p className="muted" style={{ marginTop: 0 }}>
          <Link href={`/${q}`}>{workspace.name}</Link>
        </p>
        <h2 style={{ margin: "0 0 0.75rem", fontSize: "1rem" }}>Chat</h2>
        <p className="muted" style={{ marginTop: 0 }}>
          <Link href={`/wiki/${workspaceId}${q}`}>Wiki</Link>
          {" · "}
          <Link href={`/docs/${workspaceId}${q}`}>Docs</Link>
          {" · "}
          <Link href={`/search/${workspaceId}${q}`}>検索</Link>
        </p>
        <ul style={{ listStyle: "none", padding: 0, margin: 0 }}>
          {(channels || []).map((c) => {
            const unread = c.id === currentId ? 0 : c.unreadCount || 0;
            return (
              <li key={c.id} style={{ margin: "0.2rem 0" }}>
                <Link href={`/chat/${workspaceId}/${c.id}${q}`} style={{ fontWeight: c.id === currentId ? 700 : 400 }}>
                  #{c.name}
                  {unread > 0 ? (
                    <span className="muted" style={{ marginLeft: "0.35rem" }}>
                      ({unread})
                    </span>
                  ) : null}
                </Link>
              </li>
            );
          })}
        </ul>
        {canEdit ? (
          <form action={createChannelAction} style={{ marginTop: "1rem", display: "grid", gap: "0.35rem" }}>
            <input name="name" placeholder="New channel" required style={{ padding: "0.35rem" }} />
            <button type="submit">チャンネル追加</button>
          </form>
        ) : (
          <p className="muted">閲覧のみ</p>
        )}
      </aside>
      <main>{children}</main>
    </div>
  );
}
