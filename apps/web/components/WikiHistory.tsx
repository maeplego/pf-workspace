"use client";

import { useRouter } from "next/navigation";
import { useState, useTransition } from "react";

import { apiFetchForPage, restorePageVersion } from "../app/actions";

type VersionInfo = {
  pageId: string;
  number: number;
  title: string;
  sub?: string;
  createdAt: string;
};

type DiffLine = { op: "equal" | "delete" | "insert"; text: string };

type Props = {
  workspaceId: string;
  pageId: string;
  lockVersion: number;
  readOnly: boolean;
  devUser?: string;
  versions: VersionInfo[];
};

export function WikiHistory({ workspaceId, pageId, lockVersion, readOnly, devUser, versions }: Props) {
  const router = useRouter();
  const [pending, startTransition] = useTransition();
  const [from, setFrom] = useState(versions[0]?.number || 1);
  const [to, setTo] = useState(versions[versions.length - 1]?.number || 1);
  const [lines, setLines] = useState<DiffLine[]>([]);
  const [titleNote, setTitleNote] = useState("");
  const [error, setError] = useState("");

  function loadDiff() {
    if (from === to) {
      setLines([]);
      setTitleNote("同じ版は比較しません");
      return;
    }
    startTransition(async () => {
      try {
        const diff = (await apiFetchForPage(`/v1/pages/${pageId}/diff?from=${from}&to=${to}`, devUser)) as {
          titleChanged: boolean;
          fromTitle: string;
          toTitle: string;
          lines: DiffLine[];
        };
        setLines(diff.lines || []);
        setTitleNote(diff.titleChanged ? `タイトル: ${diff.fromTitle} → ${diff.toTitle}` : "");
        setError("");
      } catch {
        setError("diff に失敗しました");
      }
    });
  }

  function restore(number: number) {
    startTransition(async () => {
      try {
        await restorePageVersion(workspaceId, pageId, number, lockVersion, devUser);
        setError("");
        router.refresh();
      } catch {
        setError("復元に失敗しました（version 競合の可能性）");
        router.refresh();
      }
    });
  }

  return (
    <section className="card-surface" style={{ marginTop: "1.5rem" }}>
      <h2 style={{ marginTop: 0, fontSize: "1rem" }}>履歴</h2>
      <p className="muted">API の title+body スナップショット（Y.Doc バイトではない）。collab 接続中の復元は再読込後に反映されます。</p>
      {error ? <p style={{ color: "#bf2600" }}>{error}</p> : null}
      {versions.length === 0 ? (
        <p className="muted">版がありません。</p>
      ) : (
        <>
          <ol className="version-list">
            {versions.map((v) => (
              <li key={v.number}>
                v{v.number} {v.title}
                <span className="muted"> · {new Date(v.createdAt).toISOString()}</span>
                {!readOnly ? (
                  <>
                    {" "}
                    <button type="button" onClick={() => restore(v.number)} disabled={pending}>
                      この版に戻す
                    </button>
                  </>
                ) : null}
              </li>
            ))}
          </ol>
          <div style={{ display: "flex", gap: "0.5rem", alignItems: "center", flexWrap: "wrap" }}>
            <label>
              from
              <select value={from} onChange={(e) => setFrom(Number(e.target.value))} style={{ marginLeft: "0.25rem" }}>
                {versions.map((v) => (
                  <option key={v.number} value={v.number}>
                    {v.number}
                  </option>
                ))}
              </select>
            </label>
            <label>
              to
              <select value={to} onChange={(e) => setTo(Number(e.target.value))} style={{ marginLeft: "0.25rem" }}>
                {versions.map((v) => (
                  <option key={v.number} value={v.number}>
                    {v.number}
                  </option>
                ))}
              </select>
            </label>
            <button type="button" onClick={loadDiff} disabled={pending}>
              比較
            </button>
          </div>
          {titleNote ? <p className="muted">{titleNote}</p> : null}
          {lines.length > 0 ? (
            <pre className="wiki-diff">
              {lines.map((ln, i) => (
                <div key={i} className={`diff-${ln.op}`}>
                  {(ln.op === "insert" ? "+" : ln.op === "delete" ? "-" : " ") + ln.text}
                </div>
              ))}
            </pre>
          ) : null}
        </>
      )}
    </section>
  );
}
