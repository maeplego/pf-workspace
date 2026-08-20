import { unstable_noStore as noStore } from "next/cache";
import { redirect } from "next/navigation";

import { apiFetchForPage, issueCollabTicket } from "../../../actions";
import { DocEditor } from "../../../../components/DocEditor";
import { DocsShell } from "../../../../components/DocsShell";
import { oidcEnabled } from "../../../../lib/oidc/env";
import { getWorkspaceSession } from "../../../../lib/session";

export default async function DocViewPage({
  params,
  searchParams,
}: {
  params: Promise<{ workspaceId: string; documentId: string }>;
  searchParams: Promise<{ user?: string }>;
}) {
  noStore();
  const { workspaceId, documentId } = await params;
  const sp = await searchParams;
  const session = await getWorkspaceSession(sp.user);
  if (oidcEnabled() && !session) {
    redirect("/login");
  }
  const devUser = session!.devMode ? session!.sub : undefined;
  const doc = (await apiFetchForPage(`/v1/documents/${documentId}`, devUser)) as {
    id: string;
    title: string;
    body: string;
    collabDocumentId: string;
    lastEditorSub?: string;
    updatedAt?: string;
    deletedAt?: string;
  };
  const { members } = (await apiFetchForPage(`/v1/workspaces/${workspaceId}/members`, devUser)) as {
    members: { sub: string; role: string }[];
  };
  const me = members.find((m) => m.sub === session!.sub);
  const readOnly = me?.role === "guest";
  const collabWsUrl = process.env.NEXT_PUBLIC_COLLAB_WS_URL || "ws://localhost:8097";
  let collab: { ticket: string; collabDocumentId: string; readOnly: boolean } | null = null;
  try {
    collab = await issueCollabTicket(doc.collabDocumentId, devUser);
  } catch {
    collab = null;
  }

  return (
    <DocsShell workspaceId={workspaceId} currentId={doc.id} sessionSub={session!.sub} devUser={devUser}>
      <DocEditor
        workspaceId={workspaceId}
        documentId={doc.id}
        title={doc.title}
        body={doc.body}
        collabDocumentId={doc.collabDocumentId}
        collab={collab}
        collabWsUrl={collabWsUrl}
        userName={session!.displayName || session!.sub}
        readOnly={!!readOnly || !!doc.deletedAt}
        devUser={devUser}
        lastEditorSub={doc.lastEditorSub}
        updatedAt={doc.updatedAt}
      />
    </DocsShell>
  );
}
