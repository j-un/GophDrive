import { describe, it, expect } from "vitest";
import { partitionFilesByKind, FOLDER_MIME_TYPE } from "./partitionFiles";
import type { FileItem } from "@/lib/api";

const file = (over: Partial<FileItem> & Pick<FileItem, "id">): FileItem => ({
  name: over.id,
  mimeType: "application/vnd.google-apps.document",
  modifiedTime: "2026-05-14T00:00:00Z",
  size: 0,
  ...over,
});

describe("partitionFilesByKind", () => {
  it("separates folders from notes by mimeType", () => {
    const { folders, notes } = partitionFilesByKind([
      file({ id: "n1" }),
      file({ id: "f1", mimeType: FOLDER_MIME_TYPE }),
    ]);

    expect(folders.map((f) => f.id)).toEqual(["f1"]);
    expect(notes.map((n) => n.id)).toEqual(["n1"]);
  });

  it("sorts folders by name ascending (locale-aware)", () => {
    const { folders } = partitionFilesByKind([
      file({ id: "b", name: "Banana", mimeType: FOLDER_MIME_TYPE }),
      file({ id: "a", name: "apple", mimeType: FOLDER_MIME_TYPE }),
      file({ id: "c", name: "Cherry", mimeType: FOLDER_MIME_TYPE }),
    ]);

    expect(folders.map((f) => f.name)).toEqual(["apple", "Banana", "Cherry"]);
  });

  it("sorts notes by modifiedTime descending (newest first)", () => {
    const { notes } = partitionFilesByKind([
      file({ id: "old", modifiedTime: "2026-01-01T00:00:00Z" }),
      file({ id: "new", modifiedTime: "2026-05-01T00:00:00Z" }),
      file({ id: "mid", modifiedTime: "2026-03-01T00:00:00Z" }),
    ]);

    expect(notes.map((n) => n.id)).toEqual(["new", "mid", "old"]);
  });

  it("returns empty arrays for empty input", () => {
    expect(partitionFilesByKind([])).toEqual({ folders: [], notes: [] });
  });
});
