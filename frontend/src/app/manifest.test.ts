import { describe, it, expect } from "vitest";
import manifest from "./manifest";

describe("PWA Manifest", () => {
  it("should return a valid manifest object", () => {
    const config = manifest();

    expect(config.name).toBe("GophDrive");
    expect(config.short_name).toBe("GophDrive");
    expect(config.display).toBe("standalone");
    expect(config.start_url).toBe("/");
  });

  it("should have the required icon sizes and purposes", () => {
    const config = manifest();
    const icons = config.icons || [];

    // Check for 192x192 icons
    const icon192 = icons.filter((i) => i.sizes === "192x192");
    expect(icon192.length).toBeGreaterThanOrEqual(1);
    expect(icon192.some((i) => i.purpose === "maskable")).toBe(true);

    // Check for 512x512 icons
    const icon512 = icons.filter((i) => i.sizes === "512x512");
    expect(icon512.length).toBeGreaterThanOrEqual(1);
    expect(icon512.some((i) => i.purpose === "maskable")).toBe(true);
  });
});
