import { vi } from "vitest";

window.alert = vi.fn();
window.confirm = vi.fn(() => true);
window.prompt = vi.fn(() => null);
