const DB_NAME = "gophdrive-db";
const DB_VERSION = 1;
const STORE_NOTES = "notes";

declare const isoDateTimeBrand: unique symbol;

export type IsoDateTime = string & { readonly [isoDateTimeBrand]: never };

const UTC_ISO = /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d{1,3})?Z$/;

export function isoDateTime(value: string): IsoDateTime {
  if (!UTC_ISO.test(value)) {
    throw new Error("modifiedTime must be UTC ISO-8601");
  }
  return value as IsoDateTime;
}

export interface LocalNote {
  id: string;
  subject: string;
  name: string;
  content: string;
  modifiedTime: IsoDateTime;
  dirty?: boolean; // True if modified locally and needs sync
}

export type LocalNoteDraft = Omit<LocalNote, "modifiedTime"> & {
  modifiedTime: string;
};

export const openDB = (): Promise<IDBDatabase> => {
  if (typeof window === "undefined") return Promise.reject("Window undefined");

  return new Promise((resolve, reject) => {
    const request = indexedDB.open(DB_NAME, DB_VERSION);
    request.onerror = () => reject(request.error);
    request.onsuccess = () => resolve(request.result);
    request.onupgradeneeded = (event) => {
      const db = (event.target as IDBOpenDBRequest).result;
      if (!db.objectStoreNames.contains(STORE_NOTES)) {
        db.createObjectStore(STORE_NOTES, { keyPath: "id" });
      }
    };
  });
};

export const saveNoteLocal = async (note: LocalNoteDraft) => {
  const stored: LocalNote = {
    ...note,
    modifiedTime: isoDateTime(note.modifiedTime),
  };
  const db = await openDB();
  return new Promise<void>((resolve, reject) => {
    const tx = db.transaction(STORE_NOTES, "readwrite");
    const store = tx.objectStore(STORE_NOTES);
    const req = store.put(stored);
    req.onsuccess = () => resolve();
    req.onerror = () => reject(req.error);
  });
};

type StoredNote = {
  id: string;
  subject?: string;
  name: string;
  content: string;
  modifiedTime?: string;
  dirty?: boolean;
};

function ownedNote(
  note: StoredNote | undefined,
  subject: string,
): LocalNote | undefined {
  if (!subject || !note || note.subject !== subject) return undefined;
  if (!note.modifiedTime || !UTC_ISO.test(note.modifiedTime)) return undefined;
  return {
    id: note.id,
    subject: note.subject,
    name: note.name,
    content: note.content,
    modifiedTime: note.modifiedTime as IsoDateTime,
    dirty: note.dirty,
  };
}

export const getNoteLocal = async (
  id: string,
  subject: string,
): Promise<LocalNote | undefined> => {
  const db = await openDB();
  return new Promise((resolve, reject) => {
    const tx = db.transaction(STORE_NOTES, "readonly");
    const store = tx.objectStore(STORE_NOTES);
    const req = store.get(id);
    req.onsuccess = () => resolve(ownedNote(req.result, subject));
    req.onerror = () => reject(req.error);
  });
};

export const getAllNotesLocal = async (
  subject: string,
): Promise<LocalNote[]> => {
  const db = await openDB();
  return new Promise((resolve, reject) => {
    const tx = db.transaction(STORE_NOTES, "readonly");
    const store = tx.objectStore(STORE_NOTES);
    const req = store.getAll();
    req.onsuccess = () => {
      const notes = (req.result ?? []) as StoredNote[];
      resolve(
        notes.flatMap((note) => {
          const owned = ownedNote(note, subject);
          return owned ? [owned] : [];
        }),
      );
    };
    req.onerror = () => reject(req.error);
  });
};

export const deleteNoteLocal = async (id: string): Promise<void> => {
  const db = await openDB();
  return new Promise((resolve, reject) => {
    const tx = db.transaction(STORE_NOTES, "readwrite");
    const store = tx.objectStore(STORE_NOTES);
    const req = store.delete(id);
    req.onsuccess = () => resolve();
    req.onerror = () => reject(req.error);
  });
};

export const syncLocalNotesToSubject = async (
  subject: string | null,
): Promise<void> => {
  const db = await openDB();
  return new Promise((resolve, reject) => {
    const tx = db.transaction(STORE_NOTES, "readwrite");
    const store = tx.objectStore(STORE_NOTES);
    tx.oncomplete = () => resolve();
    tx.onerror = () => reject(tx.error);
    tx.onabort = () => reject(tx.error);

    if (!subject) {
      store.clear();
      return;
    }

    const req = store.openCursor();
    req.onerror = () => reject(req.error);
    req.onsuccess = () => {
      const cursor = req.result;
      if (!cursor) return;
      const value = cursor.value as Partial<LocalNote>;
      if (value.subject !== subject) cursor.delete();
      cursor.continue();
    };
  });
};
