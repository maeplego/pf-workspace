"use client";

import { useCallback, useEffect, useRef, useState } from "react";

import { apiFetchForPage, postMessage, uploadWorkspaceFile } from "../app/actions";

export type ChatMsg = {
  id: string;
  sub: string;
  body: string;
  mentions?: string[];
  attachmentFileId?: string;
  seq: number;
  createdAt: string;
};

type Member = { sub: string; role: string };

type Props = {
  workspaceId: string;
  channelId: string;
  initial: ChatMsg[];
  ticket: string;
  wsBase: string;
  userSub: string;
  members: Member[];
  readOnly: boolean;
  devUser?: string;
};

function mentionQuery(text: string): string | null {
  const m = text.match(/@([A-Za-z0-9._-]*)$/);
  return m ? m[1] : null;
}

function renderBody(body: string, members: Member[]) {
  const parts = body.split(/(@[A-Za-z0-9._-]+)/g);
  const known = new Set(members.map((m) => m.sub));
  return parts.map((part, i) => {
    if (part.startsWith("@") && known.has(part.slice(1))) {
      return (
        <span key={i} className="mention-token">
          {part}
        </span>
      );
    }
    return <span key={i}>{part}</span>;
  });
}

function AttachmentThumb({ fileId, devUser }: { fileId: string; devUser?: string }) {
  const [url, setUrl] = useState("");
  useEffect(() => {
    let cancelled = false;
    apiFetchForPage(`/v1/files/${fileId}`, devUser)
      .then((f: { url?: string }) => {
        if (!cancelled && f.url) setUrl(f.url);
      })
      .catch(() => undefined);
    return () => {
      cancelled = true;
    };
  }, [fileId, devUser]);
  if (!url) return null;
  return <img src={url} alt="" style={{ maxWidth: 240, display: "block", marginTop: "0.35rem" }} />;
}

export function ChatRoom({
  workspaceId,
  channelId,
  initial,
  ticket,
  wsBase,
  userSub,
  members,
  readOnly,
  devUser,
}: Props) {
  const [messages, setMessages] = useState(initial);
  const [status, setStatus] = useState("connecting");
  const [typing, setTyping] = useState<string[]>([]);
  const [draft, setDraft] = useState("");
  const [pending, setPending] = useState(false);
  const [file, setFile] = useState<File | null>(null);
  const lastSeq = useRef(0);
  const listRef = useRef<HTMLDivElement>(null);
  const wsRef = useRef<WebSocket | null>(null);
  const typingGate = useRef(false);

  useEffect(() => {
    lastSeq.current = initial.reduce((m, x) => Math.max(m, x.seq), 0);
    setMessages(initial);
  }, [initial]);

  const catchUp = useCallback(async () => {
    const data = (await apiFetchForPage(`/v1/channels/${channelId}/messages?afterSeq=${lastSeq.current}`, devUser)) as {
      messages: ChatMsg[];
    };
    if (!data.messages?.length) return;
    setMessages((prev) => {
      const seen = new Set(prev.map((m) => m.id));
      const extra = data.messages.filter((m) => !seen.has(m.id));
      for (const m of extra) {
        lastSeq.current = Math.max(lastSeq.current, m.seq);
      }
      return extra.length ? [...prev, ...extra] : prev;
    });
  }, [channelId, devUser]);

  useEffect(() => {
    const url = `${wsBase}?ticket=${encodeURIComponent(ticket)}&channelId=${encodeURIComponent(channelId)}`;
    let closed = false;
    const typingSeen = new Map<string, ReturnType<typeof setTimeout>>();

    function connect() {
      const ws = new WebSocket(url);
      wsRef.current = ws;
      ws.onopen = () => {
        setStatus("connected");
        void catchUp();
      };
      ws.onclose = () => {
        setStatus("disconnected");
        if (!closed) setTimeout(connect, 1500);
      };
      ws.onmessage = (ev) => {
        try {
          const data = JSON.parse(String(ev.data)) as { type: string; sub?: string; message?: ChatMsg };
          if (data.type === "message" && data.message && data.message.seq > lastSeq.current) {
            lastSeq.current = data.message.seq;
            setMessages((prev) => (prev.some((m) => m.id === data.message!.id) ? prev : [...prev, data.message!]));
          }
          if (data.type === "typing" && data.sub && data.sub !== userSub) {
            const sub = data.sub;
            const prev = typingSeen.get(sub);
            if (prev) clearTimeout(prev);
            typingSeen.set(
              sub,
              setTimeout(() => {
                typingSeen.delete(sub);
                setTyping([...typingSeen.keys()]);
              }, 2000),
            );
            setTyping([...typingSeen.keys()]);
          }
        } catch {
          /* ignore malformed frames */
        }
      };
    }
    connect();
    return () => {
      closed = true;
      wsRef.current?.close();
      wsRef.current = null;
    };
  }, [channelId, ticket, wsBase, userSub, catchUp]);

  useEffect(() => {
    listRef.current?.scrollTo({ top: listRef.current.scrollHeight });
  }, [messages]);

  function sendTyping() {
    if (readOnly || typingGate.current) return;
    typingGate.current = true;
    setTimeout(() => {
      typingGate.current = false;
    }, 400);
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify({ type: "typing" }));
    }
  }

  const q = mentionQuery(draft);
  const suggestions =
    q === null ? [] : members.filter((m) => m.sub !== userSub && m.sub.toLowerCase().startsWith(q.toLowerCase())).slice(0, 6);

  function pickMention(sub: string) {
    setDraft(draft.replace(/@([A-Za-z0-9._-]*)$/, `@${sub} `));
  }

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    const body = draft.trim();
    if (readOnly || (!body && !file)) return;
    setPending(true);
    try {
      let attachmentFileId = "";
      if (file) {
        const fd = new FormData();
        fd.set("file", file);
        const uploaded = await uploadWorkspaceFile(workspaceId, "chat", fd, devUser);
        attachmentFileId = uploaded.id;
      }
      const msg = await postMessage(channelId, body, attachmentFileId || undefined, devUser);
      if (msg.seq >= lastSeq.current) {
        lastSeq.current = msg.seq;
        setMessages((prev) => (prev.some((m) => m.id === msg.id) ? prev : [...prev, msg]));
      }
      setDraft("");
      setFile(null);
    } finally {
      setPending(false);
    }
  }

  return (
    <div>
      <p className="muted">chat: {status}</p>
      <div ref={listRef} className="chat-log">
        {messages.map((m) => {
          const mine = (m.mentions || []).includes(userSub);
          return (
            <p key={m.id} className={mine ? "mention-self" : undefined} style={{ margin: "0.35rem 0" }}>
              <strong>{m.sub}</strong> <span className="muted">#{m.seq}</span>
              <br />
              {renderBody(m.body, members)}
              {m.attachmentFileId ? <AttachmentThumb fileId={m.attachmentFileId} devUser={devUser} /> : null}
            </p>
          );
        })}
      </div>
      {typing.length ? <p className="muted">{typing.join(", ")} が入力中…</p> : null}
      {readOnly ? (
        <p className="muted">guest は投稿できません。</p>
      ) : (
        <form onSubmit={submit} style={{ marginTop: "0.75rem" }}>
          {suggestions.length ? (
            <ul className="card-surface" style={{ listStyle: "none", padding: "0.35rem 0.5rem", margin: "0 0 0.35rem" }}>
              {suggestions.map((m) => (
                <li key={m.sub}>
                  <button type="button" onClick={() => pickMention(m.sub)}>
                    @{m.sub}
                  </button>
                </li>
              ))}
            </ul>
          ) : null}
          <div style={{ display: "flex", gap: "0.5rem" }}>
            <input
              value={draft}
              onChange={(e) => {
                setDraft(e.target.value);
                sendTyping();
              }}
              placeholder="メッセージ（@sub でメンション）"
              style={{ flex: 1, padding: "0.5rem" }}
            />
            <button type="submit" disabled={pending}>
              送信
            </button>
          </div>
          <input
            type="file"
            accept="image/*"
            onChange={(e) => setFile(e.target.files?.[0] || null)}
            style={{ marginTop: "0.5rem" }}
          />
        </form>
      )}
    </div>
  );
}
