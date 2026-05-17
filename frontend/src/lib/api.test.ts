import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import {
  getToken,
  setToken,
  clearToken,
  isLoggedIn,
  apiFetch,
  parseJson,
  listFiles,
  createNote,
  updateNote,
  moveFile,
  deleteFile,
  searchFiles,
  listTags,
  exportNotes,
  isValidNoteId,
  getBreadcrumbs,
  setFetchFn,
  resetFetchFn,
} from "./api";

beforeEach(() => {
  vi.spyOn(console, "error").mockImplementation(() => {});
  vi.spyOn(console, "log").mockImplementation(() => {});
});
afterEach(() => {
  vi.restoreAllMocks();
});

// Mock localStorage
const localStorageMock = (() => {
  let store: Record<string, string> = {};
  return {
    getItem: (key: string) => store[key] ?? null,
    setItem: (key: string, value: string) => {
      store[key] = value;
    },
    removeItem: (key: string) => {
      delete store[key];
    },
    clear: () => {
      store = {};
    },
  };
})();

Object.defineProperty(globalThis, "localStorage", { value: localStorageMock });

// Fake fetch helper
function fakeFetch(
  status: number,
  body: unknown = {},
  contentType = "application/json",
): typeof fetch {
  return vi.fn().mockResolvedValue({
    ok: status >= 200 && status < 300,
    status,
    headers: {
      get: (name: string) => {
        if (name.toLowerCase() === "content-type") return contentType;
        return null;
      },
    },
    json: () => Promise.resolve(body),
    text: () => Promise.resolve(JSON.stringify(body)),
  } as unknown as Response);
}

describe("Token management", () => {
  beforeEach(() => localStorageMock.clear());

  it("setToken stores and getToken retrieves", () => {
    setToken("abc123");
    expect(getToken()).toBe("abc123");
  });

  it("clearToken removes token", () => {
    setToken("abc123");
    clearToken();
    expect(getToken()).toBeNull();
  });

  it("isLoggedIn returns true when token exists", () => {
    setToken("abc123");
    expect(isLoggedIn()).toBe(true);
  });

  it("isLoggedIn returns false when no token", () => {
    expect(isLoggedIn()).toBe(false);
  });
});

describe("apiFetch", () => {
  beforeEach(() => {
    localStorageMock.clear();
  });
  afterEach(() => {
    resetFetchFn();
  });

  it("adds Authorization header when token is set", async () => {
    const mockFetch = fakeFetch(200, { ok: true });
    setFetchFn(mockFetch);
    setToken("my-token");

    await apiFetch("/test");

    expect(mockFetch).toHaveBeenCalledTimes(1);
    const [, init] = (mockFetch as ReturnType<typeof vi.fn>).mock.calls[0];
    expect(init.headers["Authorization"]).toBe("Bearer my-token");
  });

  it("does not add Authorization header when no token", async () => {
    const mockFetch = fakeFetch(200);
    setFetchFn(mockFetch);

    await apiFetch("/test");

    const [, init] = (mockFetch as ReturnType<typeof vi.fn>).mock.calls[0];
    expect(init.headers["Authorization"]).toBeUndefined();
  });

  it("clears token on 401 response", async () => {
    const mockFetch = fakeFetch(401);
    setFetchFn(mockFetch);
    setToken("old-token");

    await apiFetch("/test");

    expect(getToken()).toBeNull();
  });

  it("adds cache buster timestamp to URL", async () => {
    const mockFetch = fakeFetch(200);
    setFetchFn(mockFetch);

    await apiFetch("/test");

    const [url] = (mockFetch as ReturnType<typeof vi.fn>).mock.calls[0];
    expect(url).toMatch(/\/test\?_t=\d+/);
  });

  it("uses & separator when path already has query params", async () => {
    const mockFetch = fakeFetch(200);
    setFetchFn(mockFetch);

    await apiFetch("/test?foo=bar");

    const [url] = (mockFetch as ReturnType<typeof vi.fn>).mock.calls[0];
    expect(url).toMatch(/\/test\?foo=bar&_t=\d+/);
  });
});

describe("API functions", () => {
  afterEach(() => {
    resetFetchFn();
    localStorageMock.clear();
  });

  it("listFiles returns parsed file array", async () => {
    const files = [{ id: "1", name: "test.md" }];
    setFetchFn(fakeFetch(200, files));

    const result = await listFiles();
    expect(result).toEqual(files);
  });

  it("listFiles passes folderId as query parameter", async () => {
    const mockFetch = fakeFetch(200, []);
    setFetchFn(mockFetch);

    await listFiles("folder-123");

    const [url] = (mockFetch as ReturnType<typeof vi.fn>).mock.calls[0];
    expect(url).toContain("folderId=folder-123");
  });

  it("createNote sends name, content, parentId in body", async () => {
    const mockFetch = fakeFetch(201, { id: "new", name: "note.md" });
    setFetchFn(mockFetch);

    await createNote("note.md", "# Hello", "parent-1");

    const [, init] = (mockFetch as ReturnType<typeof vi.fn>).mock.calls[0];
    const body = JSON.parse(init.body);
    expect(body.name).toBe("note.md");
    expect(body.content).toBe("# Hello");
    expect(body.parentId).toBe("parent-1");
  });

  it("updateNote sends If-Match header when etag is provided", async () => {
    const mockFetch = fakeFetch(200, { id: "1", name: "note.md" });
    setFetchFn(mockFetch);

    await updateNote("1", "new content", "etag-abc");

    const [, init] = (mockFetch as ReturnType<typeof vi.fn>).mock.calls[0];
    expect(init.headers["If-Match"]).toBe("etag-abc");
  });

  it("updateNote throws Conflict on 412", async () => {
    setFetchFn(fakeFetch(412));

    await expect(updateNote("1", "content", "stale-etag")).rejects.toThrow(
      "Conflict",
    );
  });

  it("moveFile sends PATCH with parentId in body", async () => {
    const mockFetch = fakeFetch(200, {
      id: "1",
      name: "note.md",
      parents: ["dest-folder"],
    });
    setFetchFn(mockFetch);

    await moveFile("note-id", "dest-folder");

    const [url, init] = (mockFetch as ReturnType<typeof vi.fn>).mock.calls[0];
    expect(url).toContain("/notes/note-id");
    expect(init.method).toBe("PATCH");
    const body = JSON.parse(init.body);
    expect(body.parentId).toBe("dest-folder");
  });

  it("moveFile sends empty string parentId for root", async () => {
    const mockFetch = fakeFetch(200, {
      id: "1",
      name: "note.md",
      parents: ["root"],
    });
    setFetchFn(mockFetch);

    await moveFile("note-id", "");

    const [, init] = (mockFetch as ReturnType<typeof vi.fn>).mock.calls[0];
    const body = JSON.parse(init.body);
    expect(body.parentId).toBe("");
  });

  it("deleteFile succeeds on 2xx", async () => {
    setFetchFn(fakeFetch(200));

    await expect(deleteFile("1")).resolves.toBeUndefined();
  });

  it("searchFiles encodes query parameter", async () => {
    const mockFetch = fakeFetch(200, []);
    setFetchFn(mockFetch);

    await searchFiles("hello world");

    const [url] = (mockFetch as ReturnType<typeof vi.fn>).mock.calls[0];
    expect(url).toContain("q=hello+world");
  });

  it("searchFiles appends tag params", async () => {
    const mockFetch = fakeFetch(200, []);
    setFetchFn(mockFetch);

    await searchFiles("", ["develop", "work/q3"]);

    const [url] = (mockFetch as ReturnType<typeof vi.fn>).mock.calls[0];
    expect(url).toContain("tag=develop");
    expect(url).toContain("tag=work%2Fq3");
  });

  it("listTags returns tag list", async () => {
    const mockFetch = fakeFetch(200, [
      { name: "develop", count: 5 },
      { name: "work/q3", count: 2 },
    ]);
    setFetchFn(mockFetch);

    const result = await listTags();

    expect(result).toEqual([
      { name: "develop", count: 5 },
      { name: "work/q3", count: 2 },
    ]);
    const [url] = (mockFetch as ReturnType<typeof vi.fn>).mock.calls[0];
    expect(url).toContain("/api/tags");
  });

  it("listTags passes limit query param", async () => {
    const mockFetch = fakeFetch(200, [{ name: "develop", count: 5 }]);
    setFetchFn(mockFetch);

    await listTags(50);

    const [url] = (mockFetch as ReturnType<typeof vi.fn>).mock.calls[0];
    expect(url).toContain("/api/tags?limit=50");
  });

  it("searchFiles returns empty array without fetching when both query and tags are empty", async () => {
    const mockFetch = fakeFetch(200, []);
    setFetchFn(mockFetch);

    const result = await searchFiles("", undefined);

    expect(result).toEqual([]);
    expect((mockFetch as ReturnType<typeof vi.fn>).mock.calls).toHaveLength(0);
  });

  it("searchFiles sends both q and tag params", async () => {
    const mockFetch = fakeFetch(200, []);
    setFetchFn(mockFetch);

    await searchFiles("sprint", ["develop"]);

    const [url] = (mockFetch as ReturnType<typeof vi.fn>).mock.calls[0];
    expect(url).toContain("q=sprint");
    expect(url).toContain("tag=develop");
  });

  it("listTags throws on server error", async () => {
    setFetchFn(
      vi.fn().mockResolvedValue({
        ok: false,
        status: 500,
        headers: { get: () => "application/json" },
        json: () => Promise.resolve({ error: "internal" }),
        text: () => Promise.resolve("internal server error"),
      } as unknown as Response),
    );

    await expect(listTags()).rejects.toThrow("Failed to list tags");
  });

  it("listTags throws on HTML response", async () => {
    setFetchFn(fakeFetch(200, [], "text/html"));

    await expect(listTags()).rejects.toThrow(
      "Server returned an unexpected response",
    );
  });

  it("exportNotes returns blob and parses Content-Disposition filename", async () => {
    const fakeBlob = { size: 42 } as Blob;
    const headers: Record<string, string> = {
      "Content-Disposition":
        'attachment; filename="gophdrive-export-20260511.zip"',
      "Content-Type": "application/zip",
    };
    const mockFetch = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      headers: {
        get: (name: string) => headers[name] ?? null,
      },
      blob: () => Promise.resolve(fakeBlob),
    } as unknown as Response);
    setFetchFn(mockFetch as typeof fetch);

    const result = await exportNotes();
    expect(result.blob).toBe(fakeBlob);
    expect(result.filename).toBe("gophdrive-export-20260511.zip");
  });

  it("exportNotes falls back to default filename when header is missing", async () => {
    const fakeBlob = { size: 0 } as Blob;
    const mockFetch = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      headers: { get: () => null },
      blob: () => Promise.resolve(fakeBlob),
    } as unknown as Response);
    setFetchFn(mockFetch as typeof fetch);

    const result = await exportNotes();
    expect(result.filename).toBe("gophdrive-export.zip");
  });
});

describe("isValidNoteId", () => {
  it("returns true for a valid UUID v4", () => {
    expect(isValidNoteId("550e8400-e29b-41d4-a716-446655440000")).toBe(true);
  });

  it("returns false for a Google Drive-style ID", () => {
    expect(isValidNoteId("1IZs1hz3OOdkQI-dlnjm4PSwucbSNHwtB")).toBe(false);
  });

  it("returns false for empty string", () => {
    expect(isValidNoteId("")).toBe(false);
  });

  it("returns false for a UUID with wrong version", () => {
    expect(isValidNoteId("f81d4fae-7dec-11d0-a765-00a0c91e6bf6")).toBe(false);
  });

  it("returns true for a valid UUID v4 with uppercase hex", () => {
    expect(isValidNoteId("550E8400-E29B-41D4-A716-446655440000")).toBe(true);
  });

  it("returns false for a UUID with leading/trailing whitespace", () => {
    expect(isValidNoteId(" 550e8400-e29b-41d4-a716-446655440000 ")).toBe(false);
  });

  it("returns false for a UUID with invalid variant nibble", () => {
    expect(isValidNoteId("550e8400-e29b-41d4-c716-446655440000")).toBe(false);
  });
});

describe("getBreadcrumbs", () => {
  const BASE_ID = "00000000-0000-4000-8000-000000000000";
  const FOLDER_A = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa";
  const FOLDER_B = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb";
  const FOLDER_C = "cccccccc-cccc-4ccc-8ccc-cccccccccccc";

  function makeRoutedFetch(
    routes: Record<string, { status: number; body: unknown }>,
  ): typeof fetch {
    return vi.fn().mockImplementation(async (url: string) => {
      for (const [pattern, { status, body }] of Object.entries(routes)) {
        if (url.includes(pattern)) {
          return {
            ok: status >= 200 && status < 300,
            status,
            headers: {
              get: (name: string) =>
                name.toLowerCase() === "content-type"
                  ? "application/json"
                  : null,
            },
            json: () => Promise.resolve(body),
            text: () => Promise.resolve(JSON.stringify(body)),
          } as unknown as Response;
        }
      }
      return {
        ok: false,
        status: 404,
        headers: { get: () => "application/json" },
        json: () => Promise.resolve({ error: "not found" }),
        text: () => Promise.resolve("not found"),
      } as unknown as Response;
    });
  }

  beforeEach(() => {
    localStorageMock.clear();
    vi.spyOn(console, "warn").mockImplementation(() => {});
  });

  afterEach(() => {
    resetFetchFn();
  });

  it("returns full ancestor chain for 3-level hierarchy", async () => {
    setFetchFn(
      makeRoutedFetch({
        "/auth/user": {
          status: 200,
          body: { id: "u1", base_folder_id: BASE_ID },
        },
        [`/notes/${FOLDER_C}`]: {
          status: 200,
          body: { id: FOLDER_C, name: "FolderC", parents: [FOLDER_B] },
        },
        [`/notes/${FOLDER_B}`]: {
          status: 200,
          body: { id: FOLDER_B, name: "FolderB", parents: [FOLDER_A] },
        },
        [`/notes/${FOLDER_A}`]: {
          status: 200,
          body: { id: FOLDER_A, name: "FolderA", parents: [BASE_ID] },
        },
      }),
    );

    const crumbs = await getBreadcrumbs(FOLDER_C);
    expect(crumbs).toEqual([
      { id: "", name: "Home" },
      { id: FOLDER_A, name: "FolderA" },
      { id: FOLDER_B, name: "FolderB" },
      { id: FOLDER_C, name: "FolderC" },
    ]);
  });

  it("returns only Home when given the base folder ID", async () => {
    setFetchFn(
      makeRoutedFetch({
        "/auth/user": {
          status: 200,
          body: { id: "u1", base_folder_id: BASE_ID },
        },
      }),
    );

    const crumbs = await getBreadcrumbs(BASE_ID);
    expect(crumbs).toEqual([{ id: "", name: "Home" }]);
  });

  it("stops traversal when a parent ID is not a valid UUID", async () => {
    setFetchFn(
      makeRoutedFetch({
        "/auth/user": {
          status: 200,
          body: { id: "u1", base_folder_id: BASE_ID },
        },
        [`/notes/${FOLDER_A}`]: {
          status: 200,
          body: { id: FOLDER_A, name: "FolderA", parents: ["not-a-uuid"] },
        },
      }),
    );

    const crumbs = await getBreadcrumbs(FOLDER_A);
    expect(crumbs).toEqual([
      { id: "", name: "Home" },
      { id: FOLDER_A, name: "FolderA" },
    ]);
  });

  it("returns partial chain when an ancestor fetch fails", async () => {
    setFetchFn(
      makeRoutedFetch({
        "/auth/user": {
          status: 200,
          body: { id: "u1", base_folder_id: BASE_ID },
        },
        [`/notes/${FOLDER_A}`]: {
          status: 200,
          body: { id: FOLDER_A, name: "FolderA", parents: [FOLDER_B] },
        },
        // FOLDER_B not in routes → 404 → traversal stops
      }),
    );

    const crumbs = await getBreadcrumbs(FOLDER_A);
    expect(crumbs).toEqual([
      { id: "", name: "Home" },
      { id: FOLDER_A, name: "FolderA" },
    ]);
  });
});

describe("parseJson", () => {
  it("parses JSON when Content-Type is application/json", async () => {
    const res = {
      headers: { get: () => "application/json" },
      json: () => Promise.resolve({ ok: true }),
    } as unknown as Response;

    const result = await parseJson(res);
    expect(result).toEqual({ ok: true });
  });

  it("throws when Content-Type is text/html", async () => {
    const res = {
      headers: { get: () => "text/html" },
      json: () => Promise.resolve({}),
    } as unknown as Response;

    await expect(parseJson(res)).rejects.toThrow(
      "Server returned an unexpected response",
    );
  });

  it("throws when Content-Type is missing", async () => {
    const res = {
      headers: { get: () => null },
      json: () => Promise.resolve({}),
    } as unknown as Response;

    await expect(parseJson(res)).rejects.toThrow(
      "Server returned an unexpected response",
    );
  });

  it("listFiles throws on HTML response from server", async () => {
    setFetchFn(fakeFetch(200, [], "text/html"));

    await expect(listFiles()).rejects.toThrow(
      "Server returned an unexpected response",
    );
    resetFetchFn();
  });
});
