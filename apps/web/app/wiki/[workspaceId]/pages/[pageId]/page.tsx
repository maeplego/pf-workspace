import { unstable_noStore as noStore } from "next/cache";
import { redirect } from "next/navigation";

import { apiFetchForPage, issueCollabTicket } from "../../../../actions";
import { WikiEditor } from "../../../../../components/WikiEditor";
import { WikiShell } from "../../../../../components/WikiShell";
import { oidcEnabled } from "../../../../../lib/oidc/env";
import { getWorkspaceSession } from "../../../../../lib/session";

export default async function WikiPageView({
  params,
  searchParams,
}: {
  params: Promise<{ workspaceId: string; pageId: string }>;
  searchParams: Promise<{ user?: string }>;
}) {
  noStore();
  const { workspaceId, pageId } = await params;
  const sp = await searchParams;
  const session = await getWorkspaceSession(sp.user);
  if (oidcEnabled() && !session) {
    redirect("/login");
  }
  const devUser = session!.devMode ? session!.sub : undefined;
  const page = (await apiFetchForPage(`/v1/pages/${pageId}`, devUser)) as {
    id: string;
    title: string;
    body: string;
    status: string;
    version: number;
    collabDocumentId: string;
  };
  const { members } = (await apiFetchForPage(`/v1/workspaces/${workspaceId}/members`, devUser)) as {
    members: { sub: string; role: string }[];
  };
  const me = members.find((m) => m.sub === session!.sub);
  const readOnly = me?.role === "guest";
  const collabWsUrl = process.env.NEXT_PUBLIC_COLLAB_WS_URL || "ws://localhost:8097";
  let collab: { ticket: string; collabDocumentId: string; readOnly: boolean } | null = null;
  try {
    collab = await issueCollabTicket(page.collabDocumentId, devUser);
  } catch {
    collab = null;
  }

  return (
    <WikiShell workspaceId={workspaceId} currentId={page.id} sessionSub={session!.sub} devUser={devUser}>
      <WikiEditor
        workspaceId={workspaceId}
        pageId={page.id}
        title={page.title}
        body={page.body}
        status={page.status}
        version={page.version}
        readOnly={!!readOnly}
        collabDocumentId={page.collabDocumentId}
        collab={collab}
        collabWsUrl={collabWsUrl}
        userName={session!.displayName || session!.sub}
        devUser={devUser}
      />
    </WikiShell>
  );
}
