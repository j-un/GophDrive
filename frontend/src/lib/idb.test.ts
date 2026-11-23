import { describe, it, expect } from "vitest";
import "fake-indexeddb/auto";
import {
  saveNoteLocal,
  getNoteLocal,
  getAllNotesLocal,
  deleteNoteLocal,
  openDB,
  type LocalNoteDraft,
} from "./idb";

const subject = "user-test";

let testCounter = 0;
function uniqueId() {
  return `note-${++testCounter}-${Date.now()}`;
}

describe("IndexedDB (idb.ts)", () => {
  it("saves and retrieves a note", async () => {
    const id = uniqueId();
    const note: LocalNoteDraft = {
      id,
      subject,
      name: "Test Note",
      content: "# Hello",
      modifiedTime: new Date().toISOString(),
      dirty: false,
    };

    await saveNoteLocal(note);
    const retrieved = await getNoteLocal(id, subject);
    expect(retrieved).toBeDefined();
    expect(retrieved!.id).toBe(id);
    expect(retrieved!.content).toBe("# Hello");
  });

  it("returns undefined for non-existent note", async () => {
    const result = await getNoteLocal("nonexistent-" + Date.now(), subject);
    expect(result).toBeUndefined();
  });

  it("getAllNotesLocal returns saved notes", async () => {
    const id1 = uniqueId();
    const id2 = uniqueId();

    await saveNoteLocal({
      id: id1,
      subject,
      name: "Note 1",
      content: "A",
      modifiedTime: new Date().toISOString(),
    });
    await saveNoteLocal({
      id: id2,
      subject,
      name: "Note 2",
      content: "B",
      modifiedTime: new Date().toISOString(),
    });

    const all = await getAllNotesLocal(subject);
    const ids = all.map((n) => n.id);
    expect(ids).toContain(id1);
    expect(ids).toContain(id2);
  });

  it("deletes a note and cannot retrieve it", async () => {
    const id = uniqueId();
    await saveNoteLocal({
      id,
      subject,
      name: "To Delete",
      content: "bye",
      modifiedTime: new Date().toISOString(),
    });
    await deleteNoteLocal(id);
    const result = await getNoteLocal(id, subject);
    expect(result).toBeUndefined();
  });

  it("does not return a note cached for another subject", async () => {
    const id = uniqueId();
    await saveNoteLocal({
      id,
      subject: "user-a",
      name: "Alice secret",
      content: "alice body",
      modifiedTime: new Date().toISOString(),
    });

    expect(await getNoteLocal(id, "user-b")).toBeUndefined();
    const visible = await getAllNotesLocal("user-b");
    expect(visible.map((n) => n.id)).not.toContain(id);
  });

  it("does not return a cached note that has no subject", async () => {
    const id = uniqueId();
    await saveNoteLocal({
      id,
      name: "Legacy",
      content: "old body",
      modifiedTime: new Date().toISOString(),
    } as LocalNoteDraft);

    expect(await getNoteLocal(id, "user-a")).toBeUndefined();
  });

  it("rejects a note whose modifiedTime is not UTC ISO", async () => {
    const id = uniqueId();
    await expect(
      saveNoteLocal({
        id,
        subject,
        name: "Bad time",
        content: "nope",
        modifiedTime: "yesterday",
      }),
    ).rejects.toThrow(Error);
    expect(await getNoteLocal(id, subject)).toBeUndefined();
  });

  it("does not return a stored note whose modifiedTime is not UTC ISO", async () => {
    const id = uniqueId();
    const db = await openDB();
    await new Promise<void>((resolve, reject) => {
      const tx = db.transaction("notes", "readwrite");
      const req = tx.objectStore("notes").put({
        id,
        subject,
        name: "Bad time",
        content: "nope",
        modifiedTime: "yesterday",
      });
      req.onsuccess = () => resolve();
      req.onerror = () => reject(req.error);
    });

    expect(await getNoteLocal(id, subject)).toBeUndefined();
    const visible = await getAllNotesLocal(subject);
    expect(visible.map((note) => note.id)).not.toContain(id);
  });

  it("returns a note cached for the same subject", async () => {
    const id = uniqueId();
    await saveNoteLocal({
      id,
      subject: "user-a",
      name: "Alice secret",
      content: "alice body",
      modifiedTime: new Date().toISOString(),
    });

    expect((await getNoteLocal(id, "user-a"))?.content).toBe("alice body");
    const visible = await getAllNotesLocal("user-a");
    expect(visible.map((n) => n.id)).toContain(id);
  });
});
