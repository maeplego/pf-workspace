"use client";

import dynamic from "next/dynamic";
import { useRouter } from "next/navigation";
import { useCallback, useState, useTransition } from "react";

import { updateDocumentTitle } from "../app/actions";
import { MarkdownPreview } from "./MarkdownPreview";

const CollabMarkdownEditor = dynamic(() => import("./CollabMarkdownEditor").then((m) => m.CollabMarkdownEditor), {
  ssr: false,
});

type Ticket = {
  ticket: string;
  collabDocumentId: string;
  readOnly: boolean;
};

type Props = {
  workspaceId: string;
  documentId: string;
  title: string;
  body: string;
  collabDocumentId: string;
  collab?: Ticket | null;
  collabWsUrl?: string;
  userName: string;
  readOnly: boolean;
  devUser?: string;
};

export function DocEditor({
  workspaceId,
  documentId,
  title,
  body,
  collabDocumentId,
  collab,
  collabWsUrl,
  userName,
  readOnly,
  devUser,
}: Props) {
  const router = useRouter();
  const [pending, startTransition] = useTransition();
  const [t, setT] = useState(title);
  const [error, setError] = useState("");
  const [useCollab, setUseCollab] = useState(Boolean(collab && collabWsUrl));
  const onFallback = useCallback(() => setUseCollab(false), []);

  function saveTitle() {
    startTransition(async () => {
      try {
        await updateDocumentTitle(workspaceId, documentId, t, devUser);
        setError("");
        router.refresh();
      } catch {
        setError("タイトルの保存に失敗しました");
      }
    });
  }

  return (
    <div>
      {error ? <p style={{ color: "#bf2600" }}>{error}</p> : null}
      {!readOnly ? (
        <label style={{ display: "block", marginBottom: "0.75rem" }}>
          タイトル
          <span style={{ display: "flex", gap: "0.5rem", marginTop: "0.25rem" }}>
            <input value={t} onChange={(e) => setT(e.target.value)} style={{ flex: 1, padding: "0.4rem" }} />
            <button type="button" onClick={saveTitle} disabled={pending}>
              保存
            </button>
          </span>
        </label>
      ) : (
        <h1>{title}</h1>
      )}
      {useCollab && collab && collabWsUrl ? (
        <CollabMarkdownEditor
          wsUrl={collabWsUrl}
          documentName={collabDocumentId}
          ticket={collab.ticket}
          userName={userName}
          readOnly={readOnly || collab.readOnly}
          onFallback={onFallback}
        />
      ) : (
        <MarkdownPreview source={body} />
      )}
    </div>
  );
}
