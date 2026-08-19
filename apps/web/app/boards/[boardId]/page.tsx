import { unstable_noStore as noStore } from "next/cache";
import { redirect } from "next/navigation";

import { apiFetchForPage } from "../../actions";
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
