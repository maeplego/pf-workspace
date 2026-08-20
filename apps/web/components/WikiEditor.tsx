"use client";

import dynamic from "next/dynamic";
import { useRouter } from "next/navigation";
import { useCallback, useState, useTransition } from "react";

import { archivePage, updatePage } from "../app/actions";
import { ConfirmDelete } from "./ConfirmDelete";
import { MarkdownPreview } from "./MarkdownPreview";
import { WikiAttach } from "./WikiAttach";
import { WikiHistory } from "./WikiHistory";

const CollabMarkdownEditor = dynamic(() => import("./CollabMarkdownEditor").then((m) => m.CollabMarkdownEditor), {
  ssr: false,
});

type Ticket = {
  ticket: string;
  collabDocumentId: string;
  readOnly: boolean;
};

type VersionInfo = {
  pageId: string;
  number: number;
  title: string;
  status?: string;
  sub?: string;
  createdAt: string;
};

type Props = {
  workspaceId: string;
  pageId: string;
  title: string;
  body: string;
  status: string;
  version: number;
  readOnly: boolean;
  collabDocumentId: string;
  collab?: Ticket | null;
  collabWsUrl?: string;
  userName: string;
  devUser?: string;
  versions?: VersionInfo[];
};

export function WikiEditor({
  workspaceId,
  pageId,
  title,
  body,
  status,
  version,
  readOnly,
  collabDocumentId,
  collab,
  collabWsUrl,
  userName,
  devUser,
  versions = [],
}: Props) {
  const router = useRouter();
  const [pending, startTransition] = useTransition();
  const [t, setT] = useState(title);
  const [md, setMd] = useState(body);
  const [st, setSt] = useState(status);
  const [error, setError] = useState("");
  const [useCollab, setUseCollab] = useState(Boolean(collab && collabWsUrl));
  const onFallback = useCallback(() => setUseCollab(false), []);

  function save(includeBody: boolean) {
    startTransition(async () => {
      try {
        await updatePage(workspaceId, pageId, t, includeBody ? md : undefined, st, version, devUser);
        setError("");
        router.refresh();
      } catch {
        setError("保存に失敗しました（version 競合の可能性）");
        router.refresh();
      }
    });
  }

  if (readOnly && !useCollab) {
    return (
      <article>
        <h1>{title}</h1>
        <MarkdownPreview source={body} />
        <WikiHistory
          workspaceId={workspaceId}
          pageId={pageId}
          lockVersion={version}
          readOnly
          devUser={devUser}
          versions={versions}
        />
      </article>
    );
  }

  return (
    <div>
      {error ? <p style={{ color: "#bf2600" }}>{error}</p> : null}
      <WikiAttach workspaceId={workspaceId} pageId={pageId} readOnly={readOnly} devUser={devUser} />
      {!readOnly ? (
        <>
          <label style={{ display: "block", marginBottom: "0.5rem" }}>
            タイトル
            <input value={t} onChange={(e) => setT(e.target.value)} style={{ display: "block", width: "100%", padding: "0.4rem" }} />
          </label>
          <label style={{ display: "inline-flex", gap: "0.5rem", alignItems: "center", marginBottom: "0.5rem" }}>
            状態
            <select value={st} onChange={(e) => setSt(e.target.value)}>
              <option value="draft">draft</option>
              <option value="published">published</option>
            </select>
          </label>
          <div style={{ marginBottom: "0.75rem", display: "flex", gap: "0.5rem", flexWrap: "wrap" }}>
            <button type="button" onClick={() => save(false)} disabled={pending}>
              タイトルと状態を保存
            </button>
            <ConfirmDelete
              label="ページをアーカイブ"
              message="このページと子ページをアーカイブします。左のアーカイブ一覧から戻せます。"
              onConfirm={archivePage.bind(null, workspaceId, pageId, devUser)}
            />
          </div>
        </>
      ) : null}
      {useCollab && collab && collabWsUrl ? (
        <CollabMarkdownEditor
          wsUrl={collabWsUrl}
          documentName={collabDocumentId}
          ticket={collab.ticket}
          userName={userName}
          readOnly={readOnly || collab.readOnly}
          onFallback={onFallback}
        />
      ) : readOnly ? (
        <MarkdownPreview source={body} />
      ) : (
        <>
          <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: "1rem" }}>
            <textarea
              value={md}
              onChange={(e) => setMd(e.target.value)}
              rows={18}
              style={{ width: "100%", padding: "0.5rem", fontFamily: "ui-monospace, monospace" }}
            />
            <MarkdownPreview source={md} />
          </div>
          <p className="muted">collab に繋がらないため、単一ユーザー保存に切り替えました。</p>
          <button type="button" onClick={() => save(true)} disabled={pending}>
            本文を保存
          </button>
        </>
      )}
      <WikiHistory
        workspaceId={workspaceId}
        pageId={pageId}
        lockVersion={version}
        readOnly={readOnly}
        devUser={devUser}
        versions={versions}
      />
    </div>
  );
}
