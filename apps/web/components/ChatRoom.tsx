"use client";

import { useCallback, useEffect, useRef, useState } from "react";

import { apiFetchForPage, postMessage } from "../app/actions";

export type ChatMsg = {
  id: string;
  sub: string;
  body: string;
  seq: number;
  createdAt: string;
};

type Props = {
  channelId: string;
  initial: ChatMsg[];
  ticket: string;
  wsBase: string;
  userSub: string;
  readOnly: boolean;
  devUser?: string;
};

export function ChatRoom({ channelId, initial, ticket, wsBase, userSub, readOnly, devUser }: Props) {
  const [messages, setMessages] = useState(initial);
  const [status, setStatus] = useState("connecting");
  const [typing, setTyping] = useState<string[]>([]);
  const [draft, setDraft] = useState("");
  const [pending, setPending] = useState(false);
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

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    const body = draft.trim();
    if (!body || readOnly) return;
    setPending(true);
    try {
      const msg = await postMessage(channelId, body, devUser);
      if (msg.seq >= lastSeq.current) {
        lastSeq.current = msg.seq;
        setMessages((prev) => (prev.some((m) => m.id === msg.id) ? prev : [...prev, msg]));
      }
      setDraft("");
    } finally {
      setPending(false);
    }
  }

  return (
    <div>
      <p className="muted">chat: {status}</p>
      <div ref={listRef} className="chat-log">
        {messages.map((m) => (
          <p key={m.id} style={{ margin: "0.35rem 0" }}>
            <strong>{m.sub}</strong> <span className="muted">#{m.seq}</span>
            <br />
            {m.body}
          </p>
        ))}
      </div>
      {typing.length ? <p className="muted">{typing.join(", ")} が入力中…</p> : null}
      {readOnly ? (
        <p className="muted">guest は投稿できません。</p>
      ) : (
        <form onSubmit={submit} style={{ display: "flex", gap: "0.5rem", marginTop: "0.75rem" }}>
          <input
            value={draft}
            onChange={(e) => {
              setDraft(e.target.value);
              sendTyping();
            }}
            placeholder="メッセージ"
            style={{ flex: 1, padding: "0.5rem" }}
          />
          <button type="submit" disabled={pending}>
            送信
          </button>
        </form>
      )}
    </div>
  );
}
