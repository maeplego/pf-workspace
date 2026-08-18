"use client";

import { markdown } from "@codemirror/lang-markdown";
import { EditorState } from "@codemirror/state";
import { EditorView } from "@codemirror/view";
import { HocuspocusProvider } from "@hocuspocus/provider";
import { basicSetup } from "codemirror";
import { useEffect, useRef, useState } from "react";
import * as Y from "yjs";
import { yCollabIME } from "./yCollabIME";

import { MarkdownPreview } from "./MarkdownPreview";

const COLORS = ["#e03131", "#2f9e44", "#1971c2", "#f08c00", "#9c36b5"];

function colorFor(name: string) {
  let n = 0;
  for (let i = 0; i < name.length; i++) n += name.charCodeAt(i);
  return COLORS[n % COLORS.length];
}

type Props = {
  wsUrl: string;
  documentName: string;
  ticket: string;
  userName: string;
  readOnly: boolean;
  onFallback?: () => void;
};

export function CollabMarkdownEditor({ wsUrl, documentName, ticket, userName, readOnly, onFallback }: Props) {
  const parent = useRef<HTMLDivElement>(null);
  const [status, setStatus] = useState("connecting");
  const [preview, setPreview] = useState("");
  const fallbackTimer = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    const el = parent.current;
    if (!el) return;

    const ydoc = new Y.Doc();
    const ytext = ydoc.getText("content");
    const provider = new HocuspocusProvider({
      url: wsUrl,
      name: documentName,
      document: ydoc,
      token: ticket,
    });
    const awareness = provider.awareness;
    if (!awareness) {
      provider.destroy();
      ydoc.destroy();
      onFallback?.();
      return;
    }
    awareness.setLocalStateField("user", {
      name: userName,
      color: colorFor(userName),
    });

    const updatePreview = () => setPreview(ytext.toString());
    ytext.observe(updatePreview);
    updatePreview();

    let connected = false;
    const onStatus = ({ status: next }: { status: string }) => {
      setStatus(next);
      if (next === "connected") {
        connected = true;
        if (fallbackTimer.current) {
          clearTimeout(fallbackTimer.current);
          fallbackTimer.current = null;
        }
      }
    };
    provider.on("status", onStatus);

    fallbackTimer.current = setTimeout(() => {
      if (!connected) {
        onFallback?.();
      }
    }, 4000);

    const state = EditorState.create({
      doc: ytext.toString(),
      extensions: [
        basicSetup,
        markdown(),
        EditorState.readOnly.of(readOnly),
        yCollabIME(ytext, awareness),
        EditorView.lineWrapping,
      ],
    });
    const view = new EditorView({ state, parent: el });

    return () => {
      if (fallbackTimer.current) clearTimeout(fallbackTimer.current);
      ytext.unobserve(updatePreview);
      provider.off("status", onStatus);
      view.destroy();
      provider.destroy();
      ydoc.destroy();
    };
  }, [wsUrl, documentName, ticket, userName, readOnly, onFallback]);

  return (
    <div>
      <p className="muted">共同編集: {status}（部屋 {documentName}）</p>
      <div ref={parent} className="cm-host" />
      <h3 style={{ fontSize: "0.95rem" }}>プレビュー</h3>
      <MarkdownPreview source={preview} />
    </div>
  );
}
