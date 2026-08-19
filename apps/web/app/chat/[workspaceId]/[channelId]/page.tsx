import { unstable_noStore as noStore } from "next/cache";
import { redirect } from "next/navigation";

import { apiFetchForPage, issueChatTicket } from "../../../actions";
import { ChatRoom, type ChatMsg } from "../../../../components/ChatRoom";
import { ChatShell } from "../../../../components/ChatShell";
import { oidcEnabled } from "../../../../lib/oidc/env";
import { getWorkspaceSession } from "../../../../lib/session";

export default async function ChatChannelPage({
  params,
  searchParams,
}: {
  params: Promise<{ workspaceId: string; channelId: string }>;
  searchParams: Promise<{ user?: string }>;
}) {
  noStore();
  const { workspaceId, channelId } = await params;
  const sp = await searchParams;
  const session = await getWorkspaceSession(sp.user);
  if (oidcEnabled() && !session) {
    redirect("/login");
  }
  const devUser = session!.devMode ? session!.sub : undefined;
  const { messages } = (await apiFetchForPage(`/v1/channels/${channelId}/messages`, devUser)) as { messages: ChatMsg[] };
  const { members } = (await apiFetchForPage(`/v1/workspaces/${workspaceId}/members`, devUser)) as {
    members: { sub: string; role: string }[];
  };
  const me = members.find((m) => m.sub === session!.sub);
  const readOnly = me?.role === "guest";
  const ticket = await issueChatTicket(channelId, devUser);
  const wsBase = process.env.NEXT_PUBLIC_CHAT_WS_URL || "ws://localhost:8096/chat/ws";

  return (
    <ChatShell workspaceId={workspaceId} currentId={channelId} sessionSub={session!.sub} devUser={devUser}>
      <ChatRoom
        workspaceId={workspaceId}
        channelId={channelId}
        initial={messages || []}
        ticket={ticket.ticket}
        wsBase={wsBase}
        userSub={session!.sub}
        members={members}
        readOnly={!!readOnly || ticket.readOnly}
        devUser={devUser}
      />
    </ChatShell>
  );
}
