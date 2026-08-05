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

    // Change value
    rerender({ value: "changed" });

    // Advance past delay
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
    vi.advanceTimersByTime(500); // Not yet

    rerender({ value: "v3" });

    await act(async () => {
      vi.advanceTimersByTime(1500);
    });

    // Should only save the last value
    expect(saveFn).toHaveBeenCalledTimes(1);
    expect(saveFn).toHaveBeenCalledWith("v3");
  });

  it("reports hasUnsavedChanges correctly", async () => {
    const saveFn = vi.fn().mockResolvedValue(undefined);
    const { result, rerender } = renderHook(
      ({ value }) => useAutoSave(value, saveFn, 1000),
      { initialProps: { value: "initial" } },
    );

    // Initially no unsaved changes
    expect(result.current.hasUnsavedChanges).toBe(false);

    // Change value
    rerender({ value: "modified" });
    expect(result.current.hasUnsavedChanges).toBe(true);

    // Wait for save to complete
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

    // Trigger the timer
    await act(async () => {
      vi.advanceTimersByTime(600);
    });

    expect(result.current.isSaving).toBe(true);

    // Resolve the save
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

    // Load completes: value changes while still disabled
    rerender({ value: "loaded from server", enabled: false });

    // Enable autosave (baseline should snap to "loaded from server")
    rerender({ value: "loaded from server", enabled: true });

    // Advance well past debounce delay
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

    // Change value, starting the 2000ms debounce timer.
    rerender({ value: "changed", saveFn: saveFnA });

    // Let most of the delay elapse.
    await act(async () => {
      vi.advanceTimersByTime(1500);
    });
    expect(saveFnA).not.toHaveBeenCalled();

    // Swap only the saveFunction reference (value unchanged) — this must not
    // reset the pending timer.
    rerender({ value: "changed", saveFn: saveFnB });

    // Advance the remaining 600ms (cumulative 2100ms since the value change).
    await act(async () => {
      vi.advanceTimersByTime(600);
    });

    // Save should have fired using the latest saveFunction, not the stale one.
    expect(saveFnB).toHaveBeenCalledWith("changed");
    expect(saveFnA).not.toHaveBeenCalled();
  });
});
