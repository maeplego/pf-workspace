"use server";

import { revalidatePath } from "next/cache";
import { redirect } from "next/navigation";

import { type WorkspaceSession, requireWorkspaceSession } from "../lib/session";

const API = process.env.WORKSPACE_API_URL || "http://localhost:8096";

function authHeaders(session: WorkspaceSession): Record<string, string> {
  if (session.accessToken) {
    return { Authorization: `Bearer ${session.accessToken}` };
  }
  const headers: Record<string, string> = { "X-Dev-User-Sub": session.sub };
  if (session.email) {
    headers["X-Dev-User-Email"] = session.email;
  }
  if (session.orgId) {
    headers["X-Dev-User-Org"] = session.orgId;
  }
  return headers;
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

export async function searchOrgMembers(q: string, devUser?: string) {
  const session = await requireWorkspaceSession(devUser);
  const query = encodeURIComponent(q.trim());
  return apiFetch(`/v1/org-members?q=${query}`, session) as Promise<{
    members: { sub: string; role: string; email?: string; displayName?: string }[];
  }>;
}

export async function switchActiveOrg(orgId: string, devUser?: string) {
  const session = await requireWorkspaceSession(devUser);
  const next = orgId.trim();
  if (!next) return;

  if (session.devMode) {
    const { cookies } = await import("next/headers");
    const jar = await cookies();
    jar.set("dev_org", next, { httpOnly: true, sameSite: "lax", path: "/", maxAge: 60 * 60 * 24 * 7 });
    revalidatePath("/");
    return;
  }

  if (!session.accessToken) throw new Error("unauthorized");
  const { internalBase, clientId } = await import("../lib/oidc/env");
  const { readCookie } = await import("../lib/oidc/cookies");
  const switchRes = await fetch(`${internalBase()}/v1/active-org`, {
    method: "PUT",
    headers: {
      Authorization: `Bearer ${session.accessToken}`,
      "Content-Type": "application/json",
    },
    body: JSON.stringify({ orgId: next }),
    cache: "no-store",
  });
  if (!switchRes.ok) {
    throw new Error(await switchRes.text());
  }

  const refresh = await readCookie("rp_refresh");
  if (refresh) {
    const body = new URLSearchParams({
      grant_type: "refresh_token",
      client_id: clientId(),
      refresh_token: refresh,
    });
    const tokenRes = await fetch(`${internalBase()}/token`, {
      method: "POST",
      headers: { "Content-Type": "application/x-www-form-urlencoded" },
      body,
      cache: "no-store",
    });
    if (tokenRes.ok) {
      const tokens = (await tokenRes.json()) as {
        access_token?: string;
        id_token?: string;
        refresh_token?: string;
      };
      const { cookies } = await import("next/headers");
      const { cookieKey } = await import("../lib/oidc/env");
      const jar = await cookies();
      const base = { httpOnly: true, sameSite: "lax" as const, path: "/", maxAge: 60 * 60 * 24 * 7 };
      if (tokens.access_token) jar.set(cookieKey("rp_access"), tokens.access_token, base);
      if (tokens.id_token) jar.set(cookieKey("rp_id"), tokens.id_token, base);
      if (tokens.refresh_token) jar.set(cookieKey("rp_refresh"), tokens.refresh_token, base);
    }
  }
  revalidatePath("/");
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
	const name = String(formData.get("name") || "").trim();
  if (!name) return "ボード名を入力してください";
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

export async function syncMemberDisplayName(workspaceId: string, displayName: string, devUser?: string) {
  const session = await requireWorkspaceSession(devUser);
  const name = displayName.trim();
  if (!name) return;
  await apiFetch(`/v1/workspaces/${workspaceId}/members/me`, session, {
    method: "PUT",
    body: JSON.stringify({ displayName: name }),
  });
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

export async function updateMemberRole(workspaceId: string, memberSub: string, role: string, devUser?: string) {
  const session = await requireWorkspaceSession(devUser);
  await apiFetch(`/v1/workspaces/${workspaceId}/members/${encodeURIComponent(memberSub)}`, session, {
    method: "PATCH",
    body: JSON.stringify({ role }),
  });
  revalidatePath("/");
}

export async function removeMember(workspaceId: string, memberSub: string, devUser?: string) {
  const session = await requireWorkspaceSession(devUser);
  await apiFetch(`/v1/workspaces/${workspaceId}/members/${encodeURIComponent(memberSub)}`, session, {
    method: "DELETE",
  });
  revalidatePath("/");
}

export async function leaveWorkspace(workspaceId: string, devUser?: string) {
  const session = await requireWorkspaceSession(devUser);
  await apiFetch(`/v1/workspaces/${workspaceId}/leave`, session, {
    method: "POST",
  });
  revalidatePath("/");
}

export async function createInvitation(workspaceId: string, formData: FormData, devUser?: string) {
  const session = await requireWorkspaceSession(devUser);
  const role = String(formData.get("role") || "member");
  const invitedEmail = String(formData.get("invitedEmail") || "").trim();
  const maxUsesRaw = Number(String(formData.get("maxUses") || "1"));
  const ttlHoursRaw = Number(String(formData.get("ttlHours") || "72"));
  const data = (await apiFetch(`/v1/workspaces/${workspaceId}/invitations`, session, {
    method: "POST",
    body: JSON.stringify({
      role,
      invitedEmail: invitedEmail || undefined,
      maxUses: Number.isFinite(maxUsesRaw) ? maxUsesRaw : 1,
      ttlHours: Number.isFinite(ttlHoursRaw) ? ttlHoursRaw : 72,
    }),
  })) as { token: string };
  revalidatePath("/");
  return data.token;
}

export async function revokeInvitation(workspaceId: string, invitationId: string, devUser?: string) {
  const session = await requireWorkspaceSession(devUser);
  await apiFetch(`/v1/workspaces/${workspaceId}/invitations/${invitationId}/revoke`, session, {
    method: "POST",
    body: "{}",
  });
  revalidatePath("/");
}

export async function resendInvitation(workspaceId: string, invitationId: string, devUser?: string) {
  const session = await requireWorkspaceSession(devUser);
  const data = (await apiFetch(`/v1/workspaces/${workspaceId}/invitations/${invitationId}/resend`, session, {
    method: "POST",
    body: "{}",
  })) as { token: string };
  revalidatePath("/");
  return data.token;
}

export async function updateInvitationPolicy(workspaceId: string, invitationId: string, formData: FormData, devUser?: string) {
  const session = await requireWorkspaceSession(devUser);
  const role = String(formData.get("role") || "").trim();
  const invitedEmail = String(formData.get("invitedEmail") || "").trim();
  const maxUsesRaw = Number(String(formData.get("maxUses") || ""));
  const ttlHoursRaw = Number(String(formData.get("ttlHours") || ""));
  const body: Record<string, unknown> = {};
  if (role) body.role = role;
  if (Number.isFinite(maxUsesRaw) && maxUsesRaw > 0) body.maxUses = maxUsesRaw;
  if (Number.isFinite(ttlHoursRaw) && ttlHoursRaw > 0) body.ttlHours = ttlHoursRaw;
  body.invitedEmail = invitedEmail;
  await apiFetch(`/v1/workspaces/${workspaceId}/invitations/${invitationId}`, session, {
    method: "PATCH",
    body: JSON.stringify(body),
  });
  revalidatePath("/");
}

export async function previewInvitation(token: string, devUser?: string) {
  const session = await requireWorkspaceSession(devUser);
  return apiFetch(`/v1/invitations/${encodeURIComponent(token)}`, session) as Promise<{
    workspace: { id: string; name: string };
    invitation: { id: string; role: string; maxUses: number; useCount: number; expiresAt: string; invitedEmail?: string };
  }>;
}

export async function acceptInvitation(token: string, devUser?: string) {
  const session = await requireWorkspaceSession(devUser);
  return apiFetch(`/v1/invitations/${encodeURIComponent(token)}/accept`, session, {
    method: "POST",
    body: "{}",
  }) as Promise<{
    member: { workspaceId: string; sub: string; role: string; joinedAt: string };
    workspace: { id: string; name: string };
  }>;
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

export async function archiveBoard(boardId: string, devUser?: string) {
  const session = await requireWorkspaceSession(devUser);
  await apiFetch(`/v1/boards/${boardId}/archive`, session, { method: "POST", body: "{}" });
  revalidatePath("/");
}

export async function unarchiveBoard(boardId: string, devUser?: string) {
  const session = await requireWorkspaceSession(devUser);
  await apiFetch(`/v1/boards/${boardId}/unarchive`, session, { method: "POST", body: "{}" });
  revalidatePath("/");
}

export async function archivePage(workspaceId: string, pageId: string, devUser?: string) {
  const session = await requireWorkspaceSession(devUser);
  await apiFetch(`/v1/pages/${pageId}/archive`, session, { method: "POST", body: "{}" });
  revalidatePath(`/wiki/${workspaceId}`);
  redirect(devUser ? `/wiki/${workspaceId}?user=${encodeURIComponent(devUser)}` : `/wiki/${workspaceId}`);
}

export async function unarchivePage(workspaceId: string, pageId: string, devUser?: string) {
  const session = await requireWorkspaceSession(devUser);
  await apiFetch(`/v1/pages/${pageId}/unarchive`, session, { method: "POST", body: "{}" });
  revalidatePath(`/wiki/${workspaceId}`);
}

export async function trashDocument(workspaceId: string, documentId: string, devUser?: string) {
  const session = await requireWorkspaceSession(devUser);
  await apiFetch(`/v1/documents/${documentId}/trash`, session, { method: "POST", body: "{}" });
  revalidatePath(`/docs/${workspaceId}`);
  redirect(devUser ? `/docs/${workspaceId}?user=${encodeURIComponent(devUser)}` : `/docs/${workspaceId}`);
}

export async function untrashDocument(workspaceId: string, documentId: string, devUser?: string) {
  const session = await requireWorkspaceSession(devUser);
  await apiFetch(`/v1/documents/${documentId}/untrash`, session, { method: "POST", body: "{}" });
  revalidatePath(`/docs/${workspaceId}`);
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
  sprintId: string | undefined,
  devUser?: string,
) {
  const session = await requireWorkspaceSession(devUser);
  const payload: Record<string, unknown> = { title, description, version };
  if (sprintId !== undefined) payload.sprintId = sprintId;
  await apiFetch(`/v1/cards/${cardId}`, session, {
    method: "PATCH",
    body: JSON.stringify(payload),
  });
  revalidatePath(`/boards/${boardId}`);
  revalidatePath(`/boards/${boardId}/sprints`);
}

export async function createSprint(boardId: string, formData: FormData, devUser?: string) {
  const session = await requireWorkspaceSession(devUser);
  const name = String(formData.get("name") || "").trim();
  const startRaw = String(formData.get("startAt") || "");
  const endRaw = String(formData.get("endAt") || "");
  if (!name || !startRaw || !endRaw) return "名前と期間を入力してください";
  const startAt = new Date(startRaw).toISOString();
  const endAt = new Date(endRaw).toISOString();
  await apiFetch(`/v1/boards/${boardId}/sprints`, session, {
    method: "POST",
    body: JSON.stringify({ name, startAt, endAt }),
  });
  revalidatePath(`/boards/${boardId}`);
  revalidatePath(`/boards/${boardId}/sprints`);
}

export async function restorePageVersion(
  workspaceId: string,
  pageId: string,
  number: number,
  version: number,
  devUser?: string,
) {
  const session = await requireWorkspaceSession(devUser);
  await apiFetch(`/v1/pages/${pageId}/restore`, session, {
    method: "POST",
    body: JSON.stringify({ number, version }),
  });
  revalidatePath(`/wiki/${workspaceId}`);
  revalidatePath(`/wiki/${workspaceId}/pages/${pageId}`);
}
