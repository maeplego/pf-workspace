import { createServer } from "node:http";

import { Hocuspocus } from "@hocuspocus/server";
import * as Y from "yjs";
import { WebSocketServer } from "ws";

import { validRoom } from "./room.mjs";

const PORT = Number(process.env.COLLAB_PORT || process.env.COLLAB_HTTP_PORT || 8097);
const API = (process.env.COLLAB_API_URL || "http://localhost:8096").replace(/\/$/, "");
const TOKEN = process.env.COLLAB_INTERNAL_TOKEN || process.env.WORKSPACE_INTERNAL_TOKEN || "";
const MAX_UPDATE_BYTES = 512 * 1024;
const MAX_PLAINTEXT = 100000;

const ydocs = new Map();

async function apiJSON(path, body) {
  const res = await fetch(`${API}${path}`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      Authorization: `Bearer ${TOKEN}`,
    },
    body: JSON.stringify(body),
  });
  if (!res.ok) {
    const text = await res.text();
    const err = new Error(text || `api ${res.status}`);
    err.status = res.status;
    throw err;
  }
  return res.json();
}

const hocuspocus = new Hocuspocus({
  debounce: 2000,
  maxDebounce: 10000,
  async onAuthenticate({ token, documentName, connection }) {
    if (!validRoom(documentName)) {
      throw new Error("invalid room");
    }
    const auth = await apiJSON("/internal/v1/collab/authorize", {
      ticket: token,
      documentName,
    });
    if (auth.readOnly) {
      connection.readOnly = true;
    }
    return { name: auth.sub || "user" };
  },
  async onLoadDocument({ documentName, document }) {
    const stored = ydocs.get(documentName);
    if (stored) {
      Y.applyUpdate(document, stored);
      return document;
    }
    try {
      const { plaintext } = await apiJSON("/internal/v1/collab/plaintext", {
        collabDocumentId: documentName,
      });
      if (typeof plaintext === "string" && plaintext.length > 0) {
        document.getText("content").insert(0, plaintext.slice(0, MAX_PLAINTEXT));
      }
    } catch (err) {
      console.warn("plaintext seed failed", err.message || err);
    }
    return document;
  },
  async onStoreDocument({ documentName, document }) {
    const update = Y.encodeStateAsUpdate(document);
    if (update.byteLength > MAX_UPDATE_BYTES) {
      throw new Error("document too large");
    }
    const plaintext = document.getText("content").toString();
    if (plaintext.length > MAX_PLAINTEXT) {
      throw new Error("plaintext too large");
    }
    ydocs.set(documentName, update);
    try {
      await apiJSON("/internal/v1/collab/snapshot", {
        collabDocumentId: documentName,
        plaintext,
      });
    } catch (err) {
      console.warn("snapshot failed", err.message || err);
    }
  },
});

const httpServer = createServer((req, res) => {
  if (req.url === "/health" || req.url === "/ready") {
    res.writeHead(200, { "Content-Type": "application/json" });
    res.end(JSON.stringify({ ok: true }));
    return;
  }
  res.writeHead(404);
  res.end();
});

const wss = new WebSocketServer({ noServer: true });

httpServer.on("upgrade", (request, socket, head) => {
  wss.handleUpgrade(request, socket, head, (ws) => {
    hocuspocus.handleConnection(ws, request);
  });
});

httpServer.listen(PORT, () => {
  console.log(`workspace collab listening on ${PORT}`);
});
