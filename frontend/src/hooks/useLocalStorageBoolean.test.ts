import { describe, it, expect } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { useLocalStorageBoolean } from "./useLocalStorageBoolean";

const createFakeStorage = (initial: Record<string, string> = {}) => {
  const store: Record<string, string> = { ...initial };
  return {
    raw: store,
    getItem: (key: string) =>
      Object.prototype.hasOwnProperty.call(store, key) ? store[key] : null,
    setItem: (key: string, value: string) => {
      store[key] = value;
    },
  };
};

describe("useLocalStorageBoolean", () => {
  it("falls back to the default when storage is empty", () => {
    const storage = createFakeStorage();
    const { result } = renderHook(() =>
      useLocalStorageBoolean("k", false, storage),
    );

    expect(result.current[0]).toBe(false);
  });

  it("restores true from storage", () => {
    const storage = createFakeStorage({ k: "true" });
    const { result } = renderHook(() =>
      useLocalStorageBoolean("k", false, storage),
    );

    expect(result.current[0]).toBe(true);
  });

  it("restores false from storage even when default is true", () => {
    const storage = createFakeStorage({ k: "false" });
    const { result } = renderHook(() =>
      useLocalStorageBoolean("k", true, storage),
    );

    expect(result.current[0]).toBe(false);
  });

  it("ignores malformed values and uses the default", () => {
    const storage = createFakeStorage({ k: "yes" });
    const { result } = renderHook(() =>
      useLocalStorageBoolean("k", true, storage),
    );

    expect(result.current[0]).toBe(true);
  });

  it("persists toggled value back to storage", () => {
    const storage = createFakeStorage();
    const { result } = renderHook(() =>
      useLocalStorageBoolean("k", false, storage),
    );

    act(() => result.current[1](true));

    expect(result.current[0]).toBe(true);
    expect(storage.getItem("k")).toBe("true");

    act(() => result.current[1]((prev) => !prev));

    expect(result.current[0]).toBe(false);
    expect(storage.getItem("k")).toBe("false");
  });

  it("does not rewrite the same value back to storage on mount", () => {
    const storage = createFakeStorage({ k: "true" });
    let writes = 0;
    const tracking = {
      getItem: storage.getItem,
      setItem: (key: string, value: string) => {
        writes += 1;
        storage.setItem(key, value);
      },
    };

    renderHook(() => useLocalStorageBoolean("k", false, tracking));

    expect(writes).toBe(0);
  });
});
