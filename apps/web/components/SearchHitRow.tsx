"use client";

import Link from "next/link";
import type { ReactNode } from "react";

type Hit = {
  type: "page" | "document" | "board" | "card" | "channel" | "message";
  id: string;
  title: string;
  context?: string;
  matchLabel?: string;
  snippet: string;
  hrefHints: Record<string, string>;
};

function highlightMatch(text: string, query: string): ReactNode {
  const q = query.trim();
  if (!q || !text) return text;
  const lower = text.toLowerCase();
  const qLower = q.toLowerCase();
  const idx = lower.indexOf(qLower);
  if (idx < 0) return text;
  return (
    <>
      {text.slice(0, idx)}
      <mark className="search-highlight">{text.slice(idx, idx + q.length)}</mark>
      {text.slice(idx + q.length)}
    </>
  );
}

export function SearchHitRow({ hit, query, href }: { hit: Hit; query: string; href: string }) {
  const nameMatch = hit.matchLabel === "ボード名" || hit.matchLabel === "チャンネル名";
  const showSnippet = hit.snippet && hit.snippet !== hit.title && !nameMatch;
  const primaryText = nameMatch ? hit.title : hit.title || hit.snippet;

  return (
    <li className="card-surface" style={{ marginBottom: "0.75rem" }}>
      <span className={`search-badge search-badge-${hit.type}`}>{hit.type}</span>{" "}
      {hit.matchLabel ? (
        <Link href={href}>
          <span className="muted">{hit.matchLabel}：</span>
          {highlightMatch(primaryText, query)}
        </Link>
      ) : (
        <Link href={href}>{highlightMatch(hit.title || hit.id, query)}</Link>
      )}
      {hit.context ? (
        <p className="muted" style={{ margin: "0.2rem 0 0" }}>
          {hit.context}
        </p>
      ) : null}
      {showSnippet ? (
        <p className="muted" style={{ margin: "0.35rem 0 0" }}>
          {highlightMatch(hit.snippet, query)}
        </p>
      ) : null}
    </li>
  );
}

export type { Hit };
