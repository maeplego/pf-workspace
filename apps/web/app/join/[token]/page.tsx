import Link from "next/link";
import { redirect } from "next/navigation";

import { acceptInvitation, previewInvitation } from "../../actions";
import { oidcEnabled } from "../../../lib/oidc/env";
import { getWorkspaceSession } from "../../../lib/session";

export default async function JoinPage({
  params,
  searchParams,
}: {
  params: Promise<{ token: string }>;
  searchParams: Promise<{ user?: string }>;
}) {
  const { token } = await params;
  const sp = await searchParams;
  const session = await getWorkspaceSession(sp.user);
  if (oidcEnabled() && !session) redirect("/login");
  const devUser = session!.devMode ? session!.sub : undefined;

  let preview: Awaited<ReturnType<typeof previewInvitation>> | null = null;
  let error = "";
  try {
    preview = await previewInvitation(token, devUser);
  } catch {
    error = "招待リンクが無効、期限切れ、または上限に達しています。";
  }

  async function acceptAction() {
    "use server";
    const joined = await acceptInvitation(token, devUser);
    const q = new URLSearchParams();
    if (devUser) q.set("user", devUser);
    q.set("joined", joined.workspace.id);
    redirect(`/?${q.toString()}`);
  }

  return (
    <section className="card">
      <h1 style={{ marginTop: 0 }}>ワークスペース参加</h1>
      {preview ? (
        <>
          <p>
            <strong>{preview.workspace.name}</strong> に <strong>{preview.invitation.role}</strong> として参加します。
          </p>
          <p className="muted">
            利用状況: {preview.invitation.useCount}/{preview.invitation.maxUses} ・期限:{" "}
            {new Date(preview.invitation.expiresAt).toLocaleString("ja-JP")}
          </p>
          {preview.invitation.invitedEmail ? (
            <p className="muted">
              この招待は <strong>{preview.invitation.invitedEmail}</strong> 宛です。ログイン中アカウントのメールと一致する必要があります。
            </p>
          ) : null}
          <form action={acceptAction} className="row">
            <button type="submit" className="btn">
              このアカウントで参加
            </button>
            <Link href={devUser ? `/?user=${encodeURIComponent(devUser)}` : "/"}>キャンセル</Link>
          </form>
        </>
      ) : (
        <p className="error">{error}</p>
      )}
    </section>
  );
}
