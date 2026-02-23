"use client";

/**
 * Central API helper that adds Authorization header from stored token.
 */

const API_BASE = process.env.NEXT_PUBLIC_API_URL || "/api";

// Replaceable fetch function for testability
let fetchFn: typeof fetch = (...args: Parameters<typeof fetch>) =>
  fetch(...args);

export function setFetchFn(fn: typeof fetch) {
  fetchFn = fn;
}
export function resetFetchFn() {
  fetchFn = (...args: Parameters<typeof fetch>) => fetch(...args);
}

export function getToken(): string | null {
  if (typeof window === "undefined") return null;
  return localStorage.getItem("session_token");
}

export function setToken(token: string) {
  localStorage.setItem("session_token", token);
}

export function clearToken() {
  localStorage.removeItem("session_token");
}

export function isLoggedIn(): boolean {
  return !!getToken();
}

export async function apiFetch(
  path: string,
  init?: RequestInit,
): Promise<Response> {
  const token = getToken();
  const headers: Record<string, string> = {
    ...((init?.headers as Record<string, string>) || {}),
  };
  if (token) {
    headers["Authorization"] = `Bearer ${token}`;
  }
  // Append robust cache buster (timestamp) to query
  const timestamp = Date.now();
  const separator = path.includes("?") ? "&" : "?";
  const url = `${API_BASE}${path}${separator}_t=${timestamp}`;

  const res = await fetchFn(url, {
    ...init,
    headers,
    credentials: "include",
    cache: "no-store",
    next: { revalidate: 0 }, // For Next.js App Router (if used in server components too)
  });

  if (res.status === 401 && headers["Authorization"]) {
    clearToken();
  }

  return res;
}

export interface FileItem {
  id: string;
  name: string;
  mimeType: string;
  modifiedTime: string;
  size: number;
  parents?: string[];
  starred?: boolean;
}

async function handleError(res: Response, defaultMsg: string): Promise<never> {
  const text = await res.text();
  throw new Error(`${defaultMsg}${text ? `: ${text}` : ""}`);
}

export async function getHealth(): Promise<{ status: string }> {
  const res = await apiFetch("/health");
  if (!res.ok) return handleError(res, "Health check failed");
  return res.json();
}

export async function listFiles(folderId?: string): Promise<FileItem[]> {
  const query = folderId ? `?folderId=${encodeURIComponent(folderId)}` : "";
  const res = await apiFetch(`/notes${query}`);
  if (!res.ok) return handleError(res, "Failed to list files");
  return res.json();
}

export async function listStarred(): Promise<FileItem[]> {
  const res = await apiFetch("/starred");
  if (!res.ok) return handleError(res, "Failed to list starred files");
  return res.json();
}

export async function getFile(fileId: string): Promise<FileItem> {
  const res = await apiFetch(`/notes/${fileId}`);
  if (!res.ok) return handleError(res, "Failed to fetch file");
  return res.json();
}

export async function createFolder(
  name: string,
  parentId?: string,
): Promise<FileItem> {
  const res = await apiFetch("/folders", {
    method: "POST",
    body: JSON.stringify({ name, parentId }),
    headers: { "Content-Type": "application/json" },
  });
  if (!res.ok) return handleError(res, "Failed to create folder");
  return res.json();
}

export async function createNote(
  name: string,
  content: string,
  parentId?: string,
): Promise<FileItem> {
  const res = await apiFetch("/notes", {
    method: "POST",
    body: JSON.stringify({ name, content, parentId }),
    headers: { "Content-Type": "application/json" },
  });
  if (!res.ok) return handleError(res, "Failed to create note");
  return res.json();
}

export async function updateNote(
  id: string,
  content: string,
  etag?: string,
): Promise<FileItem> {
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
  };
  if (etag) {
    headers["If-Match"] = etag;
  }
  const res = await apiFetch(`/notes/${id}`, {
    method: "PUT",
    headers,
    body: JSON.stringify({ content }),
  });
  if (res.status === 412) {
    throw new Error("Conflict");
  }
  if (!res.ok) return handleError(res, "Failed to update note");
  return res.json();
}

export async function renameNote(id: string, name: string): Promise<FileItem> {
  const res = await apiFetch(`/notes/${id}`, {
    method: "PATCH",
    body: JSON.stringify({ name }),
    headers: { "Content-Type": "application/json" },
  });
  if (!res.ok) return handleError(res, "Failed to rename note");
  return res.json();
}

export async function duplicateNote(id: string): Promise<FileItem> {
  const res = await apiFetch(`/notes/${id}/copy`, { method: "POST" });
  if (!res.ok) return handleError(res, "Failed to duplicate note");
  return res.json();
}

export async function starFile(
  id: string,
  starred: boolean,
): Promise<FileItem> {
  const res = await apiFetch(`/notes/${id}`, {
    method: "PATCH",
    body: JSON.stringify({ starred }),
    headers: { "Content-Type": "application/json" },
  });
  if (!res.ok) return handleError(res, "Failed to update starred status");
  return res.json();
}

export async function deleteFile(fileId: string): Promise<void> {
  const res = await apiFetch(`/notes/${fileId}/delete`, {
    method: "POST",
  });
  if (!res.ok) return handleError(res, "Failed to delete file");
}

export interface BreadcrumbItem {
  id: string;
  name: string;
}

export async function getBreadcrumbs(
  folderId: string,
): Promise<BreadcrumbItem[]> {
  const breadcrumbs: BreadcrumbItem[] = [];
  let currentId: string | undefined = folderId;
  let baseFolderId: string | undefined;

  try {
    const user = await getUser();
    baseFolderId = user.base_folder_id;
  } catch (e) {
    console.warn("Failed to get user for breadcrumbs", e);
  }

  while (currentId) {
    // If we reached the base folder, stop (don't include it in breadcrumbs, as 'Home' represents it)
    if (baseFolderId && currentId === baseFolderId) {
      break;
    }

    try {
      const file = await getFile(currentId);

      // Double check in case logic above didn't catch it (e.g. initial folderId IS baseFolderId)
      if (baseFolderId && file.id === baseFolderId) {
        break;
      }

      breadcrumbs.unshift({ id: file.id, name: file.name });

      if (file.parents && file.parents.length > 0) {
        currentId = file.parents[0];
      } else {
        currentId = undefined;
      }
    } catch (error) {
      const e = error as Error;
      console.warn(
        `Failed to fetch breadcrumb for ${currentId}. This might be due to permissions (e.g. parent folder created outside the app). Stopping traversal.`,
      );
      // If it's a 404/403 (likely), we just stop here and show what we have.
      // If it's a 401, apiFetch would have cleared the token and redirected?
      // "Failed to fetch file" comes from apiFetch throwing error.
      if (e.message) {
        console.warn("Breadcrumb Error Message:", e.message);
      }
      break;
    }
  }

  // Add Home at the beginning
  breadcrumbs.unshift({ id: "", name: "Home" });
  return breadcrumbs;
}

export async function listDriveFolders(): Promise<FileItem[]> {
  const res = await apiFetch("/auth/drive/folders");
  if (!res.ok) return handleError(res, "Failed to list drive folders");
  return res.json();
}

export async function updateUser(settings: {
  base_folder_id?: string;
}): Promise<void> {
  const res = await apiFetch("/auth/user", {
    method: "PATCH",
    body: JSON.stringify(settings),
    headers: { "Content-Type": "application/json" },
  });
  if (!res.ok) return handleError(res, "Failed to update user settings");
}
export interface User {
  id: string;
  base_folder_id: string;
}

export async function getUser(): Promise<User> {
  const res = await apiFetch("/auth/user");
  if (!res.ok) return handleError(res, "Failed to get user profile");
  return res.json();
}

export async function searchFiles(query: string): Promise<FileItem[]> {
  const res = await apiFetch(`/search?q=${encodeURIComponent(query)}`);
  if (!res.ok) return handleError(res, "Failed to search files");
  return res.json();
}
