"use client";

import React, { useEffect, useState, useRef } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { FileText, Plus, RefreshCw, Folder } from "lucide-react";
import {
  FileItem,
  duplicateNote,
  renameNote,
  deleteFile,
  createNote as apiCreateNote,
  listFiles,
} from "@/lib/api";
import { NoteMenu } from "@/components/NoteMenu";
import { ConfirmDialog } from "@/components/ConfirmDialog";
import { RenameDialog } from "@/components/RenameDialog";
import { useOffline } from "@/hooks/useOffline";
import { deleteNoteLocal, getAllNotesLocal } from "@/lib/idb";

interface NoteListProps {
  folderId?: string;
  searchQuery?: string;
}

export default function NoteList({ folderId, searchQuery }: NoteListProps) {
  const router = useRouter();
  const [notes, setNotes] = useState<FileItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [isCreating, setIsCreating] = useState(false);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [newNoteName, setNewNoteName] = useState("");
  const [activeMenuId, setActiveMenuId] = useState<string | null>(null);
  const menuRef = useRef<HTMLDivElement>(null);

  // Delete State
  const [deleteNoteId, setDeleteNoteId] = useState<string | null>(null);
  const [deleteNoteName, setDeleteNoteName] = useState<string>("");

  // Rename State
  const [renameNoteId, setRenameNoteId] = useState<string | null>(null);
  const [renameNoteName, setRenameNoteName] = useState<string>("");

  const isOffline = useOffline();

  // Determine view mode: Force list view for search, otherwise grid
  const viewMode = searchQuery ? "list" : "grid";

  const loadNotes = async () => {
    setLoading(true);
    setError(null);
    try {
      // Search Mode
      if (searchQuery) {
        // Important: Use the imported searchFiles function
        const { searchFiles } = await import("@/lib/api");
        const results = await searchFiles(searchQuery);
        setNotes(results || []);
        setLoading(false);
        return;
      }

      // Offline Mode
      if (isOffline) {
        const localNotes = await getAllNotesLocal();
        // Map LocalNote to FileItem
        const fileItems: FileItem[] = localNotes.map((n) => ({
          id: n.id,
          name: n.name,
          mimeType: "application/vnd.google-apps.document", // Assuming local notes are docs
          parents: [],
          modifiedTime: n.modifiedTime,
          size: 0, // Placeholder
          version: 1, // Placeholder
          etag: "",
          createdTime: n.modifiedTime, // Placeholder
          kind: "drive#file",
          trashed: false,
        }));

        setNotes(
          fileItems.sort(
            (a, b) =>
              new Date(b.modifiedTime).getTime() -
              new Date(a.modifiedTime).getTime(),
          ),
        );
        setLoading(false);
        return;
      }

      // Online Mode
      const items = await listFiles(folderId);
      // Filter out folders if needed, or handle them. The original code filtered them out?
      // "const noteItems = (items || []).filter(item => item.mimeType !== 'application/vnd.google-apps.folder');"
      // But we want to show folders too if we support them.
      // Let's keep the filter if that's what was there, OR enable folders if we are ready.
      // The previous code had: `const noteItems = (items || []).filter(item => item.mimeType !== 'application/vnd.google-apps.folder');`
      // But I see `createFolder` code, so maybe folders are supported?
      // I'll stick to the fetchFiles result which should handle what we want.

      const noteItems = (items || []).filter(
        (item) => item.mimeType !== "application/vnd.google-apps.folder",
      );
      setNotes(
        noteItems.sort(
          (a, b) =>
            new Date(b.modifiedTime).getTime() -
            new Date(a.modifiedTime).getTime(),
        ),
      );
    } catch (error) {
      const err = error as Error;
      setError(err.message || String(error));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadNotes();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [folderId, searchQuery, isOffline]);

  // Close menu when clicking outside
  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (menuRef.current && !menuRef.current.contains(event.target as Node)) {
        setActiveMenuId(null);
      }
    };
    document.addEventListener("mousedown", handleClickOutside);
    return () => document.removeEventListener("mousedown", handleClickOutside);
  }, []);

  const navigateToFolder = (id: string | null) => {
    if (id) {
      router.push(`?folderId=${id}`);
    } else {
      router.push("/notes");
    }
  };

  const handleCreateNote = async (name: string) => {
    // Validate folderId if we think we are in a folder
    // (Note: folderId can be undefined for Root, but if we are in a "loading" state or invalid state, we should be careful)
    if (isSubmitting) return;
    setIsSubmitting(true);

    try {
      const note = await apiCreateNote(name, "# " + name, folderId);
      router.push(`/note?id=${note.id}`);
    } catch (error) {
      const err = error as Error;
      console.error("Failed to create note:", err);
      alert(err.message || "Failed to create note. Please try again.");
      setIsSubmitting(false); // Only reset on error, or if we weren't navigating away
    }
  };

  const requestDeleteNote = (e: React.MouseEvent, note: FileItem) => {
    e.preventDefault();
    e.stopPropagation();
    setActiveMenuId(null);
    setDeleteNoteId(note.id);
    setDeleteNoteName(note.name || "Untitled Note");
  };

  const handleDuplicateNote = async (e: React.MouseEvent, note: FileItem) => {
    e.preventDefault();
    e.stopPropagation();
    setActiveMenuId(null);
    try {
      await duplicateNote(note.id);
      loadNotes();
    } catch (error) {
      const err = error as Error;
      console.error(err);
      alert(err.message || "Failed to duplicate note");
    }
  };

  const requestRenameNote = (e: React.MouseEvent, note: FileItem) => {
    e.preventDefault();
    e.stopPropagation();
    setActiveMenuId(null);
    setRenameNoteId(note.id);
    setRenameNoteName(note.name || "Untitled Note");
  };

  const executeRename = async (newName: string) => {
    if (!renameNoteId) return;
    try {
      await renameNote(renameNoteId, newName);
      setRenameNoteId(null);
      loadNotes();
    } catch (error) {
      const err = error as Error;
      console.error(err);
      alert(err.message || "Failed to rename note");
    }
  };

  const confirmDeleteNote = async () => {
    if (!deleteNoteId) return;
    try {
      if (isOffline) {
        await deleteNoteLocal(deleteNoteId);
      } else {
        await deleteFile(deleteNoteId);
        // Also remove from local
        await deleteNoteLocal(deleteNoteId);
      }
      setDeleteNoteId(null);
      loadNotes();
    } catch (error) {
      const err = error as Error;
      console.error(err);
      alert(err.message || "Failed to delete note");
    }
  };

  if (loading && notes.length === 0) {
    return (
      <div
        style={{ display: "flex", justifyContent: "center", padding: "3rem" }}
      >
        <div className="w-8 h-8 border-4 border-[var(--primary)] border-t-transparent rounded-full animate-spin"></div>
      </div>
    );
  }

  if (error) {
    return (
      <div
        style={{
          textAlign: "center",
          padding: "2rem",
          background: "var(--muted)",
          borderRadius: "0.5rem",
          border: "1px solid var(--destructive)",
          margin: "2rem auto",
          maxWidth: "600px",
        }}
      >
        <p style={{ color: "var(--destructive)", marginBottom: "1rem" }}>
          {error}
        </p>
        <button onClick={loadNotes} className="btn btn-primary">
          Retry
        </button>
      </div>
    );
  }

  return (
    <div
      style={{
        padding: "2rem",
        flex: 1,
        overflowY: "auto",
        background: "var(--background)",
        color: "var(--foreground)",
      }}
    >
      <ConfirmDialog
        isOpen={!!deleteNoteId} // Use deleteNoteId for isOpen
        title="Delete Note"
        message={`Are you sure you want to delete note "${deleteNoteName}"? This cannot be undone.`} // Use deleteNoteName
        onConfirm={confirmDeleteNote}
        onCancel={() => setDeleteNoteId(null)} // Reset deleteNoteId on cancel
      />
      <RenameDialog
        isOpen={!!renameNoteId}
        initialName={renameNoteName}
        onRename={executeRename}
        onCancel={() => setRenameNoteId(null)}
      />
      <div
        style={{
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          marginBottom: "2rem",
        }}
      >
        <h2 style={{ fontSize: "1.5rem", fontWeight: "bold" }}>
          {searchQuery ? `Search results for "${searchQuery}"` : "Notes"}
        </h2>
        <div className="flex gap-2">
          <button
            onClick={loadNotes}
            className="btn"
            style={{
              background: "var(--card)",
              color: "var(--foreground)",
              fontSize: "0.875rem",
              display: "flex",
              alignItems: "center",
              gap: "0.5rem",
              border: "1px solid var(--border)",
            }}
          >
            <RefreshCw size={16} /> Refresh
          </button>
        </div>
      </div>

      {viewMode === "list" ? (
        <div
          style={{ display: "flex", flexDirection: "column", gap: "0.5rem" }}
        >
          {/* List View Implementation */}
          {notes.length === 0 && (
            <div className="text-center py-10 text-gray-500">
              No notes found.
            </div>
          )}
          {notes.map((note) => (
            <div key={note.id} style={{ position: "relative" }}>
              <Link
                href={
                  note.mimeType === "application/vnd.google-apps.folder"
                    ? `?folderId=${note.id}`
                    : `/note?id=${note.id}`
                }
                className="glass group"
                style={{
                  padding: "1rem",
                  borderRadius: "0.5rem",
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "space-between",
                  background: "var(--card)",
                  color: "var(--card-foreground)",
                  textDecoration: "none",
                  border: "1px solid var(--border)",
                }}
              >
                <div
                  style={{
                    display: "flex",
                    alignItems: "center",
                    gap: "1rem",
                    flex: 1,
                  }}
                >
                  {note.mimeType === "application/vnd.google-apps.folder" ? (
                    <Folder size={20} style={{ color: "var(--primary)" }} />
                  ) : (
                    <FileText size={20} style={{ color: "var(--primary)" }} />
                  )}
                  <div>
                    <div style={{ fontWeight: 500 }}>{note.name}</div>
                    <div
                      style={{
                        fontSize: "0.75rem",
                        opacity: 0.6,
                        fontFamily: "monospace",
                      }}
                    >
                      ID: {note.id.substring(0, 8)}...
                    </div>
                  </div>
                </div>
                <div style={{ fontSize: "0.875rem", opacity: 0.6 }}>
                  {new Date(note.modifiedTime).toLocaleString(undefined, {
                    year: "numeric",
                    month: "numeric",
                    day: "numeric",
                    hour: "2-digit",
                    minute: "2-digit",
                  })}
                </div>
              </Link>
              <div
                style={{
                  position: "absolute",
                  top: "50%",
                  right: "1rem",
                  transform: "translateY(-50%)",
                  opacity: 0,
                }}
                className="group-hover:opacity-100 transition-opacity"
              >
                <NoteMenu
                  isOpen={activeMenuId === note.id}
                  onToggle={(e) => {
                    e.preventDefault();
                    e.stopPropagation();
                    setActiveMenuId(activeMenuId === note.id ? null : note.id);
                  }}
                  onClose={() => setActiveMenuId(null)}
                  onDelete={(e) => requestDeleteNote(e, note)}
                  onDuplicate={(e) => handleDuplicateNote(e, note)}
                  onRename={(e) => requestRenameNote(e, note)}
                />
              </div>
            </div>
          ))}
        </div>
      ) : (
        <div
          style={{
            display: "grid",
            gridTemplateColumns: "repeat(auto-fill, minmax(240px, 1fr))",
            gap: "1.5rem",
            alignItems: "start",
          }}
        >
          {/* New Note Button (Only in Grid/Normal Mode) */}
          {!searchQuery &&
            (isCreating ? (
              <div
                className="glass"
                style={{
                  ...cardStyle,
                  background: "var(--card)",
                  color: "var(--card-foreground)",
                }}
              >
                <input
                  autoFocus
                  type="text"
                  placeholder="Note Name..."
                  value={newNoteName}
                  onChange={(e) => setNewNoteName(e.target.value)}
                  onKeyDown={(e) => {
                    if (e.key === "Enter") {
                      e.preventDefault();
                      if (e.nativeEvent.isComposing) return;
                      if (newNoteName.trim())
                        handleCreateNote(newNoteName.trim());
                    }
                    if (e.key === "Escape") setIsCreating(false);
                  }}
                  onBlur={() => {
                    if (!newNoteName.trim()) setIsCreating(false);
                  }}
                  style={{
                    background: "transparent",
                    border: "none",
                    borderBottom: "1px solid var(--border)",
                    textAlign: "center",
                    outline: "none",
                    width: "80%",
                    color: "inherit",
                  }}
                />
                <span style={{ fontSize: "0.75rem", opacity: 0.6 }}>
                  Press Enter
                </span>
              </div>
            ) : (
              <button
                onClick={() => {
                  setIsCreating(true);
                  setNewNoteName("");
                }}
                className="glass"
                style={{
                  ...cardStyle,
                  cursor: "pointer",
                  border: "1px dashed var(--border)",
                  background: "transparent",
                  color: "var(--muted-foreground)",
                }}
              >
                <Plus size={32} style={{ color: "var(--primary)" }} />
                <span>New Note</span>
              </button>
            ))}

          {notes.map((note) => (
            <div key={note.id} style={{ position: "relative" }}>
              <Link
                href={
                  note.mimeType === "application/vnd.google-apps.folder"
                    ? `?folderId=${note.id}`
                    : `/note?id=${note.id}`
                }
                onClick={(e: React.MouseEvent<HTMLAnchorElement>) => {
                  if (note.mimeType === "application/vnd.google-apps.folder") {
                    e.preventDefault();
                    navigateToFolder(note.id);
                  }
                }}
                className="glass"
                style={{
                  ...cardStyle,
                  textDecoration: "none",
                  color: "var(--card-foreground)",
                  background: "var(--card)",
                  justifyContent: "space-between",
                  alignItems: "stretch",
                }}
              >
                <div>
                  <div
                    style={{
                      display: "flex",
                      alignItems: "center",
                      gap: "0.75rem",
                      marginBottom: "0.5rem",
                    }}
                  >
                    {note.mimeType === "application/vnd.google-apps.folder" ? (
                      <Folder style={{ color: "var(--primary)" }} />
                    ) : (
                      <FileText style={{ color: "var(--primary)" }} />
                    )}
                    <h3
                      style={{
                        fontWeight: "bold",
                        overflow: "hidden",
                        textOverflow: "ellipsis",
                        whiteSpace: "nowrap",
                      }}
                      title={note.name}
                    >
                      {note.name}
                    </h3>
                  </div>
                  <p
                    style={{
                      fontSize: "0.75rem",
                      opacity: 0.6,
                      fontFamily: "monospace",
                    }}
                  >
                    ID: {note.id.substring(0, 8)}...
                  </p>
                </div>
                <p
                  style={{
                    fontSize: "0.75rem",
                    opacity: 0.6,
                    textAlign: "right",
                  }}
                >
                  {new Date(note.modifiedTime).toLocaleString(undefined, {
                    year: "numeric",
                    month: "numeric",
                    day: "numeric",
                    hour: "2-digit",
                    minute: "2-digit",
                  })}
                </p>
              </Link>

              <div
                style={{ position: "absolute", top: "0.5rem", right: "0.5rem" }}
              >
                <NoteMenu
                  isOpen={activeMenuId === note.id}
                  onToggle={(e) => {
                    e.preventDefault();
                    e.stopPropagation();
                    setActiveMenuId(activeMenuId === note.id ? null : note.id);
                  }}
                  onClose={() => setActiveMenuId(null)}
                  onDelete={(e) => requestDeleteNote(e, note)}
                  onDuplicate={(e) => handleDuplicateNote(e, note)}
                  onRename={(e) => requestRenameNote(e, note)}
                />
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

const cardStyle: React.CSSProperties = {
  padding: "1.25rem",
  borderRadius: "0.75rem",
  display: "flex",
  flexDirection: "column",
  justifyContent: "space-between",
  height: "11rem",
  transition: "transform 0.2s, box-shadow 0.2s",
  border: "1px solid var(--border)",
};
