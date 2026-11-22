import { describe, it, expect } from "vitest";
import { defaultNoteName } from "./defaultNoteName";

const timestampName = /^[0-9]{4}-[0-9]{2}-[0-9]{2} [0-9]{2}:[0-9]{2}:[0-9]{2}$/;

describe("defaultNoteName", () => {
  it("formats a single-digit local date and time", () => {
    const name = defaultNoteName(new Date(2026, 0, 5, 3, 4, 7));
    expect(name).toBe("2026-01-05 03:04:07");
    expect(name).toMatch(timestampName);
  });

  it("formats a double-digit local date and time", () => {
    const name = defaultNoteName(new Date(2026, 10, 22, 16, 27, 3));
    expect(name).toBe("2026-11-22 16:27:03");
    expect(name).toMatch(timestampName);
  });
});
