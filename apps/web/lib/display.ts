export function memberLabel(m: { sub: string; displayName?: string }, ambiguousNames: Set<string>): string {
  const name = (m.displayName || "").trim();
  if (!name) return m.sub;
  if (ambiguousNames.has(name)) return `${name} (${m.sub.slice(0, 8)}…)`;
  return name;
}

export function ambiguousDisplayNames(members: { displayName?: string }[]): Set<string> {
  const counts = new Map<string, number>();
  for (const m of members) {
    const name = (m.displayName || "").trim();
    if (!name) continue;
    counts.set(name, (counts.get(name) || 0) + 1);
  }
  const dupes = new Set<string>();
  for (const [name, n] of counts) {
    if (n > 1) dupes.add(name);
  }
  return dupes;
}
