/** Prefix/suffix diff so an IME commit is one Y.Text replace, not per-keystroke. */
export function diffEnds(a: string, b: string): { start: number; oldMiddle: string; newMiddle: string } {
  let start = 0;
  const maxStart = Math.min(a.length, b.length);
  while (start < maxStart && a[start] === b[start]) {
    start++;
  }
  let endA = a.length;
  let endB = b.length;
  while (endA > start && endB > start && a[endA - 1] === b[endB - 1]) {
    endA--;
    endB--;
  }
  return { start, oldMiddle: a.slice(start, endA), newMiddle: b.slice(start, endB) };
}

export function shouldSyncToYjs(composing: boolean, docChanged: boolean, fromYjs: boolean): boolean {
  return docChanged && !fromYjs && !composing;
}
