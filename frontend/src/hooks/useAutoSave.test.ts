import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { useAutoSave } from "./useAutoSave";

describe("useAutoSave", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  it("does not call save when value is unchanged", () => {
    const saveFn = vi.fn().mockResolvedValue(undefined);
    renderHook(() => useAutoSave("initial", saveFn, 1000));

    vi.advanceTimersByTime(2000);
    expect(saveFn).not.toHaveBeenCalled();
  });

  it("saves after delay when value changes", async () => {
    const saveFn = vi.fn().mockResolvedValue(undefined);
    const { rerender } = renderHook(
      ({ value }) => useAutoSave(value, saveFn, 1000),
      { initialProps: { value: "initial" } },
    );

    rerender({ value: "changed" });

    await act(async () => {
      vi.advanceTimersByTime(1500);
    });

    expect(saveFn).toHaveBeenCalledWith("changed");
  });

  it("debounces rapid changes (saves only the last value)", async () => {
    const saveFn = vi.fn().mockResolvedValue(undefined);
    const { rerender } = renderHook(
      ({ value }) => useAutoSave(value, saveFn, 1000),
      { initialProps: { value: "v1" } },
    );

    rerender({ value: "v2" });
    vi.advanceTimersByTime(500);

    rerender({ value: "v3" });

    await act(async () => {
      vi.advanceTimersByTime(1500);
    });

    expect(saveFn).toHaveBeenCalledTimes(1);
    expect(saveFn).toHaveBeenCalledWith("v3");
  });

  it("reports hasUnsavedChanges correctly", async () => {
    const saveFn = vi.fn().mockResolvedValue(undefined);
    const { result, rerender } = renderHook(
      ({ value }) => useAutoSave(value, saveFn, 1000),
      { initialProps: { value: "initial" } },
    );

    expect(result.current.hasUnsavedChanges).toBe(false);

    rerender({ value: "modified" });
    expect(result.current.hasUnsavedChanges).toBe(true);

    await act(async () => {
      vi.advanceTimersByTime(1500);
    });

    expect(result.current.hasUnsavedChanges).toBe(false);
  });

  it("reports isSaving during save", async () => {
    let resolvePromise: () => void;
    const saveFn = vi.fn().mockImplementation(
      () =>
        new Promise<void>((resolve) => {
          resolvePromise = resolve;
        }),
    );

    const { result, rerender } = renderHook(
      ({ value }) => useAutoSave(value, saveFn, 500),
      { initialProps: { value: "v1" } },
    );

    rerender({ value: "v2" });

    await act(async () => {
      vi.advanceTimersByTime(600);
    });

    expect(result.current.isSaving).toBe(true);

    await act(async () => {
      resolvePromise!();
    });

    expect(result.current.isSaving).toBe(false);
  });

  it("does not save when enabled transitions false → value change → true (initial baseline snapped)", async () => {
    const saveFn = vi.fn().mockResolvedValue(undefined);
    const { result, rerender } = renderHook(
      ({ value, enabled }: { value: string; enabled: boolean }) =>
        useAutoSave(value, saveFn, 2000, enabled),
      { initialProps: { value: "", enabled: false } },
    );

    rerender({ value: "loaded from server", enabled: false });

    rerender({ value: "loaded from server", enabled: true });

    await act(async () => {
      vi.advanceTimersByTime(3000);
    });

    expect(saveFn).not.toHaveBeenCalled();
    expect(result.current.hasUnsavedChanges).toBe(false);
  });

  it("does not reset the pending debounce when saveFunction's reference changes", async () => {
    const saveFnA = vi.fn().mockResolvedValue(undefined);
    const saveFnB = vi.fn().mockResolvedValue(undefined);
    const { rerender } = renderHook(
      ({ value, saveFn }) => useAutoSave(value, saveFn, 2000),
      { initialProps: { value: "initial", saveFn: saveFnA } },
    );

    rerender({ value: "changed", saveFn: saveFnA });

    await act(async () => {
      vi.advanceTimersByTime(1500);
    });
    expect(saveFnA).not.toHaveBeenCalled();

    rerender({ value: "changed", saveFn: saveFnB });

    await act(async () => {
      vi.advanceTimersByTime(600);
    });

    expect(saveFnB).toHaveBeenCalledWith("changed");
    expect(saveFnA).not.toHaveBeenCalled();
  });
});
