import { unstable_noStore as noStore } from "next/cache";
import Link from "next/link";
import { redirect } from "next/navigation";

import { apiFetchForPage } from "../../../actions";
import { SprintPanel, type BurndownView, type SprintView } from "../../../../components/SprintPanel";
import { oidcEnabled } from "../../../../lib/oidc/env";
import { getWorkspaceSession } from "../../../../lib/session";

export default async function SprintsPage({
  params,
  searchParams,
}: {
  params: Promise<{ boardId: string }>;
  searchParams: Promise<{ user?: string }>;
}) {
  noStore();
  const { boardId } = await params;
  const sp = await searchParams;
  const session = await getWorkspaceSession(sp.user);
  if (oidcEnabled() && !session) {
    redirect("/login");
  }
  const devUser = session!.devMode ? session!.sub : undefined;
  const q = devUser ? `?user=${encodeURIComponent(devUser)}` : "";

  const board = (await apiFetchForPage(`/v1/boards/${boardId}`, devUser)) as {
    id: string;
    name: string;
    workspaceId: string;
  };
  const workspace = (await apiFetchForPage(`/v1/workspaces/${board.workspaceId}`, devUser)) as { name: string };
  const { sprints } = (await apiFetchForPage(`/v1/boards/${boardId}/sprints`, devUser)) as { sprints: SprintView[] };
  const burndowns: Record<string, BurndownView> = {};
  for (const sprint of sprints || []) {
    burndowns[sprint.id] = (await apiFetchForPage(`/v1/sprints/${sprint.id}/burndown`, devUser)) as BurndownView;
  }

  let readOnly = false;
  try {
    const { members } = (await apiFetchForPage(`/v1/workspaces/${board.workspaceId}/members`, devUser)) as {
      members: { sub: string; role: string }[];
    };
    const me = members.find((m) => m.sub === session!.sub);
    readOnly = me?.role === "guest";
  } catch {
    readOnly = false;
  }

  return (
    <div className="shell">
      <p className="muted">
        <Link href={devUser ? `/?user=${devUser}` : "/"}>{workspace.name}</Link>
        {" / "}
        <Link href={`/boards/${board.id}${q}`}>{board.name}</Link>
        {" / スプリント"}
      </p>
      <h1 style={{ marginTop: 0 }}>{board.name} のバーンダウン</h1>
      <p className="muted">未完了は Done 列以外のカード数。ストーリーポイントは持たない。単位は cards。</p>
      <SprintPanel
        boardId={board.id}
        sprints={sprints || []}
        burndowns={burndowns}
        readOnly={readOnly}
        devUser={devUser}
      />
    </div>
  );
}
