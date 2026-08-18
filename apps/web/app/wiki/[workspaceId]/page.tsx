import { unstable_noStore as noStore } from "next/cache";
import { redirect } from "next/navigation";

import { WikiShell } from "../../../components/WikiShell";
import { oidcEnabled } from "../../../lib/oidc/env";
import { getWorkspaceSession } from "../../../lib/session";

export default async function WikiIndexPage({
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
    <WikiShell workspaceId={workspaceId} sessionSub={session!.sub} devUser={devUser}>
      <p className="muted">左のツリーからページを選ぶか、新規作成してください。</p>
    </WikiShell>
  );
}
