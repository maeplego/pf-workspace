"use client";

import { useState } from "react";

import { attachPageFile, uploadWorkspaceFile } from "../app/actions";

export function WikiAttach({
  workspaceId,
  pageId,
  readOnly,
  devUser,
}: {
  workspaceId: string;
  pageId: string;
  readOnly: boolean;
  devUser?: string;
}) {
  const [pending, setPending] = useState(false);
  const [snippet, setSnippet] = useState("");
  const [error, setError] = useState("");

  if (readOnly) return null;

  async function onFile(file: File | undefined) {
    if (!file) return;
    setPending(true);
    setError("");
    try {
      const fd = new FormData();
      fd.set("file", file);
      const uploaded = await uploadWorkspaceFile(workspaceId, "wiki", fd, devUser);
      const bound = await attachPageFile(pageId, uploaded.id, devUser);
      setSnippet(`![${bound.name || "image"}](${bound.url})`);
    } catch {
      setError("添付に失敗しました（guest は不可。20MB まで）");
    } finally {
      setPending(false);
    }
  }

  return (
    <div className="card-surface" style={{ marginBottom: "0.75rem" }}>
      <p style={{ marginTop: 0 }}>画像を添付（Markdown に貼る。Yjs には fileId だけ）</p>
      <input
        type="file"
        accept="image/*"
        disabled={pending}
        onChange={(e) => {
          const f = e.target.files?.[0];
          void onFile(f);
          e.target.value = "";
        }}
      />
      {error ? <p style={{ color: "#bf2600" }}>{error}</p> : null}
      {snippet ? (
        <p className="muted" style={{ marginBottom: 0 }}>
          エディタに貼り付け: <code>{snippet}</code>
        </p>
      ) : null}
    </div>
  );
}
