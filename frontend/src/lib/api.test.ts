import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import {
  getToken,
  setToken,
  clearToken,
  isLoggedIn,
  apiFetch,
  listFiles,
  createNote,
  updateNote,
  deleteFile,
  searchFiles,
  setFetchFn,
  resetFetchFn,
} from "./api";

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
function fakeFetch(status: number, body: unknown = {}): typeof fetch {
  return vi.fn().mockResolvedValue({
    ok: status >= 200 && status < 300,
    status,
    json: () => Promise.resolve(body),
    text: () => Promise.resolve(JSON.stringify(body)),
  } as Response);
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

  it("deleteFile succeeds on 2xx", async () => {
    setFetchFn(fakeFetch(200));

    await expect(deleteFile("1")).resolves.toBeUndefined();
  });

  it("searchFiles encodes query parameter", async () => {
    const mockFetch = fakeFetch(200, []);
    setFetchFn(mockFetch);

    await searchFiles("hello world");

    const [url] = (mockFetch as ReturnType<typeof vi.fn>).mock.calls[0];
    expect(url).toContain("q=hello%20world");
  });
});
