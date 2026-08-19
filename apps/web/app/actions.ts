"use server";

import { revalidatePath } from "next/cache";

import { type WorkspaceSession, requireWorkspaceSession } from "../lib/session";

const API = process.env.WORKSPACE_API_URL || "http://localhost:8096";

function authHeaders(session: WorkspaceSession): Record<string, string> {
  if (session.accessToken) {
    return { Authorization: `Bearer ${session.accessToken}` };
  }
  return { "X-Dev-User-Sub": session.sub };
}

async function apiFetch(path: string, session: WorkspaceSession, init?: RequestInit) {
  const res = await fetch(`${API}${path}`, {
    ...init,
    headers: {
      ...(init?.headers || {}),
      ...authHeaders(session),
      "Content-Type": "application/json",
    },
    cache: "no-store",
  });
  if (!res.ok) {
    const text = await res.text();
    throw new Error(text || res.statusText);
  }
  if (res.status === 204) return {};
  const text = await res.text();
  return text ? JSON.parse(text) : {};
}

export async function apiFetchForPage(path: string, devUser?: string) {
  const session = await requireWorkspaceSession(devUser);
  return apiFetch(path, session);
}

export async function createWorkspace(formData: FormData, devUser?: string) {
  const session = await requireWorkspaceSession(devUser);
  const name = String(formData.get("name") || "").trim();
  if (!name) return "名前を入力してください";
  await apiFetch("/v1/workspaces", session, {
    method: "POST",
    body: JSON.stringify({ name }),
  });
  revalidatePath("/");
}

export async function createBoard(workspaceId: string, formData: FormData, devUser?: string) {
  const session = await requireWorkspaceSession(devUser);
  const name = String(formData.get("name") || "").trim() || "Main board";
  const data = await apiFetch(`/v1/workspaces/${workspaceId}/boards`, session, {
    method: "POST",
    body: JSON.stringify({ name }),
  });
  revalidatePath("/");
  return data.id as string;
}

export async function createCard(columnId: string, title: string, devUser?: string) {
  const session = await requireWorkspaceSession(devUser);
  await apiFetch(`/v1/columns/${columnId}/cards`, session, {
    method: "POST",
    body: JSON.stringify({ title, description: "" }),
  });
}

export async function moveCard(
  boardId: string,
  cardId: string,
  columnId: string,
  position: number,
  version: number,
  devUser?: string,
) {
  const session = await requireWorkspaceSession(devUser);
  await apiFetch(`/v1/cards/${cardId}/move`, session, {
    method: "PATCH",
    body: JSON.stringify({ columnId, position, version }),
  });
  revalidatePath(`/boards/${boardId}`);
}

export async function createPage(workspaceId: string, formData: FormData, devUser?: string) {
  const session = await requireWorkspaceSession(devUser);
  const title = String(formData.get("title") || "").trim();
  if (!title) return;
  const parentId = String(formData.get("parentId") || "").trim();
  const status = String(formData.get("status") || "draft");
  const page = await apiFetch(`/v1/workspaces/${workspaceId}/pages`, session, {
    method: "POST",
    body: JSON.stringify({ title, parentId: parentId || undefined, status, body: "" }),
  });
  revalidatePath(`/wiki/${workspaceId}`);
  return page.id as string;
}

export async function updatePage(
  workspaceId: string,
  pageId: string,
  title: string,
  body: string | undefined,
  status: string,
  version: number,
  devUser?: string,
) {
  const session = await requireWorkspaceSession(devUser);
  const payload: Record<string, unknown> = { title, status, version };
  if (body !== undefined) payload.body = body;
  await apiFetch(`/v1/pages/${pageId}`, session, {
    method: "PATCH",
    body: JSON.stringify(payload),
  });
  revalidatePath(`/wiki/${workspaceId}`);
  revalidatePath(`/wiki/${workspaceId}/pages/${pageId}`);
}

export async function issueCollabTicket(collabDocumentId: string, devUser?: string) {
  const session = await requireWorkspaceSession(devUser);
  return apiFetch("/v1/collab-tickets", session, {
    method: "POST",
    body: JSON.stringify({ collabDocumentId }),
  }) as Promise<{ ticket: string; collabDocumentId: string; readOnly: boolean; expiresAt: string }>;
}

export async function createDocument(workspaceId: string, formData: FormData, devUser?: string) {
  const session = await requireWorkspaceSession(devUser);
  const title = String(formData.get("title") || "").trim();
  if (!title) return;
  const doc = await apiFetch(`/v1/workspaces/${workspaceId}/documents`, session, {
    method: "POST",
    body: JSON.stringify({ title, body: "" }),
  });
  revalidatePath(`/docs/${workspaceId}`);
  return doc.id as string;
}

export async function updateDocumentTitle(workspaceId: string, documentId: string, title: string, devUser?: string) {
  const session = await requireWorkspaceSession(devUser);
  await apiFetch(`/v1/documents/${documentId}`, session, {
    method: "PATCH",
    body: JSON.stringify({ title }),
  });
  revalidatePath(`/docs/${workspaceId}`);
  revalidatePath(`/docs/${workspaceId}/${documentId}`);
}

export async function addMember(workspaceId: string, formData: FormData, devUser?: string) {
  const session = await requireWorkspaceSession(devUser);
  const sub = String(formData.get("sub") || "").trim();
  const role = String(formData.get("role") || "member");
  if (!sub) return "sub を入力してください";
  await apiFetch(`/v1/workspaces/${workspaceId}/members`, session, {
    method: "POST",
    body: JSON.stringify({ sub, role }),
  });
  revalidatePath("/");
}

export async function createChannel(workspaceId: string, formData: FormData, devUser?: string) {
  const session = await requireWorkspaceSession(devUser);
  const name = String(formData.get("name") || "").trim();
  if (!name) return;
  const ch = await apiFetch(`/v1/workspaces/${workspaceId}/channels`, session, {
    method: "POST",
    body: JSON.stringify({ name }),
  });
  revalidatePath(`/chat/${workspaceId}`);
  return ch.id as string;
}

export async function postMessage(channelId: string, body: string, attachmentFileId?: string, devUser?: string) {
  const session = await requireWorkspaceSession(devUser);
  const payload: Record<string, unknown> = { body };
  if (attachmentFileId) payload.attachmentFileId = attachmentFileId;
  return apiFetch(`/v1/channels/${channelId}/messages`, session, {
    method: "POST",
    body: JSON.stringify(payload),
  }) as Promise<{
    id: string;
    channelId: string;
    sub: string;
    body: string;
    mentions: string[];
    attachmentFileId?: string;
    seq: number;
    createdAt: string;
  }>;
}

type FileView = {
  id: string;
  url: string;
  name: string;
  contentType: string;
  purpose: string;
};

async function apiSend(path: string, session: WorkspaceSession, init?: RequestInit) {
  const headers: Record<string, string> = { ...authHeaders(session) };
  const extra = (init?.headers || {}) as Record<string, string>;
  Object.assign(headers, extra);
  const res = await fetch(`${API}${path}`, {
    ...init,
    headers,
    cache: "no-store",
  });
  if (!res.ok) {
    const text = await res.text();
    throw new Error(text || res.statusText);
  }
  const text = await res.text();
  return text ? JSON.parse(text) : {};
}

async function mediaJSON(base: string, path: string, session: WorkspaceSession, init: RequestInit) {
  const res = await fetch(`${base}${path}`, {
    ...init,
    headers: { ...authHeaders(session), "Content-Type": "application/json", ...(init.headers || {}) },
    cache: "no-store",
  });
  if (!res.ok) throw new Error((await res.text()) || res.statusText);
  return res.json();
}

export async function uploadWorkspaceFile(
  workspaceId: string,
  purpose: "wiki" | "chat",
  formData: FormData,
  devUser?: string,
): Promise<FileView> {
  const session = await requireWorkspaceSession(devUser);
  const file = formData.get("file");
  if (!(file instanceof File) || file.size === 0) {
    throw new Error("ファイルを選んでください");
  }
  const cfg = (await apiFetch("/v1/uploads/config", session)) as {
    provider: string;
    maxBytes: number;
    mediaApiUrl?: string;
  };
  if (file.size > (cfg.maxBytes || 20 * 1024 * 1024)) {
    throw new Error("ファイルが大きすぎます（20MBまで）");
  }
  if (cfg.provider === "p03" && cfg.mediaApiUrl) {
    return uploadViaMedia(session, cfg.mediaApiUrl, workspaceId, purpose, file);
  }
  const body = new FormData();
  body.set("workspaceId", workspaceId);
  body.set("purpose", purpose);
  body.set("file", file);
  return apiSend("/v1/uploads", session, { method: "POST", body }) as Promise<FileView>;
}

async function uploadViaMedia(
  session: WorkspaceSession,
  mediaBase: string,
  workspaceId: string,
  purpose: "wiki" | "chat",
  file: File,
): Promise<FileView> {
  const presign = await mediaJSON(mediaBase, "/v1/uploads/presign", session, {
    method: "POST",
    body: JSON.stringify({
      contentType: file.type || "image/png",
      size: file.size,
      purpose,
    }),
  });
  const put = await fetch(presign.uploadUrl, {
    method: "PUT",
    headers: { "Content-Type": file.type || "image/png" },
    body: Buffer.from(await file.arrayBuffer()),
  });
  if (!put.ok) {
    throw new Error(`object upload failed (${put.status})`);
  }
  await mediaJSON(mediaBase, "/v1/uploads/complete", session, {
    method: "POST",
    body: JSON.stringify({ fileId: presign.fileId, etag: put.headers.get("etag") || "" }),
  });
  return apiFetch("/v1/uploads/link", session, {
    method: "POST",
    body: JSON.stringify({
      workspaceId,
      purpose,
      fileId: presign.fileId,
      name: file.name,
      contentType: file.type || "image/png",
      size: file.size,
    }),
  }) as Promise<FileView>;
}

export async function attachPageFile(pageId: string, fileId: string, devUser?: string) {
  const session = await requireWorkspaceSession(devUser);
  return apiFetch(`/v1/pages/${pageId}/attachments`, session, {
    method: "POST",
    body: JSON.stringify({ fileId }),
  }) as Promise<FileView>;
}

export async function issueChatTicket(channelId: string, devUser?: string) {
  const session = await requireWorkspaceSession(devUser);
  return apiFetch("/v1/chat-tickets", session, {
    method: "POST",
    body: JSON.stringify({ channelId }),
  }) as Promise<{ ticket: string; channelId: string; readOnly: boolean; expiresAt: string }>;
}

export async function updateCardDetails(
  boardId: string,
  cardId: string,
  title: string,
  description: string,
  version: number,
  devUser?: string,
) {
  const session = await requireWorkspaceSession(devUser);
  await apiFetch(`/v1/cards/${cardId}`, session, {
    method: "PATCH",
    body: JSON.stringify({ title, description, version }),
  });
  revalidatePath(`/boards/${boardId}`);
}
