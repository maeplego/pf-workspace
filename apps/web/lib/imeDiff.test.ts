import assert from "node:assert/strict";
import test from "node:test";

import { diffEnds, shouldSyncToYjs } from "./imeDiff.ts";

test("diffEnds replaces only the composed middle", () => {
  const d = diffEnds("abXYcd", "abZcd");
  assert.deepEqual(d, { start: 2, oldMiddle: "XY", newMiddle: "Z" });
});

test("diffEnds handles identical strings", () => {
  assert.deepEqual(diffEnds("same", "same"), { start: 4, oldMiddle: "", newMiddle: "" });
});

test("shouldSyncToYjs is false during IME composition", () => {
  assert.equal(shouldSyncToYjs(true, true, false), false);
  assert.equal(shouldSyncToYjs(false, true, false), true);
  assert.equal(shouldSyncToYjs(false, true, true), false);
  assert.equal(shouldSyncToYjs(false, false, false), false);
});
