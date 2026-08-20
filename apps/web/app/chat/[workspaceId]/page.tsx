import { unstable_noStore as noStore } from "next/cache";
import { redirect } from "next/navigation";

import { ChatShell } from "../../../components/ChatShell";
import { oidcEnabled } from "../../../lib/oidc/env";
import { getWorkspaceSession } from "../../../lib/session";

export default async function ChatIndexPage({
  params,
  searchParams,
}: {
  params: Promise<{ workspaceId: string }>;
  searchParams: Promise<{ user?: string }>;
}) {
  noStore();
  const { workspaceId } = await params;
  const sp = await searchParams;
  const session = await getWorkspaceSession(sp.user);
  if (oidcEnabled() && !session) {
    redirect("/login");
  }
  const devUser = session!.devMode ? session!.sub : undefined;
  return (
    <ChatShell workspaceId={workspaceId} sessionSub={session!.sub} devUser={devUser}>
      <p className="muted">左からチャンネルを選んでください。既定は general。履歴は REST、配信は別 WS（Yjs とは混ぜていません）。チャンネルやメッセージは監査のため削除しません。</p>
    </ChatShell>
  );
}
