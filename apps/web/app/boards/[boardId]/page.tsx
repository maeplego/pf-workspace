import { unstable_noStore as noStore } from "next/cache";
import { redirect } from "next/navigation";

import { apiFetchForPage, archiveBoard } from "../../actions";
import { ConfirmDelete } from "../../../components/ConfirmDelete";
import { KanbanBoard, type ColumnView } from "../../../components/KanbanBoard";
import { oidcEnabled } from "../../../lib/oidc/env";
import { getWorkspaceSession } from "../../../lib/session";

export default async function BoardPage({
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

  const board = (await apiFetchForPage(`/v1/boards/${boardId}`, devUser)) as {
    id: string;
    name: string;
    workspaceId: string;
    archivedAt?: string;
    columns: ColumnView[];
  };
  const workspace = (await apiFetchForPage(`/v1/workspaces/${board.workspaceId}`, devUser)) as { name: string };
  const { sprints } = (await apiFetchForPage(`/v1/boards/${boardId}/sprints`, devUser)) as {
    sprints: { id: string; name: string }[];
  };

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
      {board.archivedAt ? <p className="muted">このボードはアーカイブ済みです。ホームから戻せます。</p> : null}
      {!readOnly && !board.archivedAt ? (
        <div style={{ marginBottom: "0.75rem" }}>
          <ConfirmDelete
            label="ボードをアーカイブ"
            message="ボードをアーカイブします。カードは残ります。ホームから戻せます。"
            onConfirm={archiveBoard.bind(null, board.id, devUser)}
          />
        </div>
      ) : null}
      <KanbanBoard
        boardId={board.id}
        boardName={board.name}
        workspaceName={workspace.name}
        columns={board.columns}
        sprints={sprints || []}
        devUser={devUser}
        readOnly={readOnly}
      />
    </div>
  );
}
