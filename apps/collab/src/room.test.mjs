import assert from "node:assert/strict";
import test from "node:test";

import { ticketMatchesRoom, validRoom } from "./room.mjs";

test("validRoom accepts ULID and rejects paths", () => {
  assert.equal(validRoom("01ARZ3NDEKTSV4RRFFQ69G5FAV"), true);
  assert.equal(validRoom("../etc/passwd"), false);
  assert.equal(validRoom(""), false);
  assert.equal(validRoom("not-a-ulid"), false);
});

test("ticketMatchesRoom rejects a different document name", () => {
  const a = "01ARZ3NDEKTSV4RRFFQ69G5FAV";
  const b = "01BX5ZZKBKACTAV9WEVGEMMVRZ";
  assert.equal(ticketMatchesRoom(a, a), true);
  assert.equal(ticketMatchesRoom(a, b), false);
});
