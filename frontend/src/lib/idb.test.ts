import { describe, it, expect } from "vitest";
import "fake-indexeddb/auto";
import {
  saveNoteLocal,
  getNoteLocal,
  getAllNotesLocal,
  deleteNoteLocal,
  LocalNote,
} from "./idb";

// Use unique IDs per test to avoid cross-test state issues
let testCounter = 0;
function uniqueId() {
  return `note-${++testCounter}-${Date.now()}`;
}

describe("IndexedDB (idb.ts)", () => {
  it("saves and retrieves a note", async () => {
    const id = uniqueId();
    const note: LocalNote = {
      id,
      name: "Test Note",
      content: "# Hello",
      modifiedTime: new Date().toISOString(),
      dirty: false,
    };

    await saveNoteLocal(note);
    const retrieved = await getNoteLocal(id);
    expect(retrieved).toBeDefined();
    expect(retrieved!.id).toBe(id);
    expect(retrieved!.content).toBe("# Hello");
  });

  it("returns undefined for non-existent note", async () => {
    const result = await getNoteLocal("nonexistent-" + Date.now());
    expect(result).toBeUndefined();
  });

  it("getAllNotesLocal returns saved notes", async () => {
    const id1 = uniqueId();
    const id2 = uniqueId();

    await saveNoteLocal({
      id: id1,
      name: "Note 1",
      content: "A",
      modifiedTime: new Date().toISOString(),
    });
    await saveNoteLocal({
      id: id2,
      name: "Note 2",
      content: "B",
      modifiedTime: new Date().toISOString(),
    });

    const all = await getAllNotesLocal();
    const ids = all.map((n) => n.id);
    expect(ids).toContain(id1);
    expect(ids).toContain(id2);
  });

  it("deletes a note and cannot retrieve it", async () => {
    const id = uniqueId();
    await saveNoteLocal({
      id,
      name: "To Delete",
      content: "bye",
      modifiedTime: new Date().toISOString(),
    });
    await deleteNoteLocal(id);
    const result = await getNoteLocal(id);
    expect(result).toBeUndefined();
  });
});
