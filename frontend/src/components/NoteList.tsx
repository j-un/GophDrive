"use client";

import React, { useEffect, useState, useRef } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { FileText, Plus, RefreshCw, Folder, FolderPlus } from "lucide-react";
import {
  FileItem,
  duplicateNote,
  renameNote,
  deleteFile,
  starFile,
  createNote as apiCreateNote,
  createFolder,
  listFiles,
  searchFiles,
} from "@/lib/api";
import { NoteMenu } from "@/components/NoteMenu";
import { ConfirmDialog } from "@/components/ConfirmDialog";
import { RenameDialog } from "@/components/RenameDialog";
import { useOffline } from "@/hooks/useOffline";
import { deleteNoteLocal, getAllNotesLocal } from "@/lib/idb";
import { buildSearchRequest } from "@/lib/searchQuery";
import { partitionFilesByKind } from "@/lib/partitionFiles";

interface NoteListProps {
  folderId?: string;
  searchQuery?: string;
  tagFilter?: string[];
  onAfterMutation?: () => void;
}

export default function NoteList({
  folderId,
  searchQuery,
  tagFilter,
  onAfterMutation,
}: NoteListProps) {
  const router = useRouter();
  const [notes, setNotes] = useState<FileItem[]>([]);
  const [folders, setFolders] = useState<FileItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // New Note state
  const [isCreating, setIsCreating] = useState(false);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [newNoteName, setNewNoteName] = useState("");

  // New Folder state
  const [isCreatingFolder, setIsCreatingFolder] = useState(false);
  const [isSubmittingFolder, setIsSubmittingFolder] = useState(false);
  const [newFolderName, setNewFolderName] = useState("");

  const [activeMenuId, setActiveMenuId] = useState<string | null>(null);
  const menuRef = useRef<HTMLDivElement>(null);
  const loadRequestRef = useRef(0);

  // Delete State
  const [deleteNoteId, setDeleteNoteId] = useState<string | null>(null);
  const [deleteNoteName, setDeleteNoteName] = useState<string>("");

  // Rename State
  const [renameNoteId, setRenameNoteId] = useState<string | null>(null);
  const [renameNoteName, setRenameNoteName] = useState<string>("");

  // Folder Delete/Rename State
  const [deleteFolderId, setDeleteFolderId] = useState<string | null>(null);
  const [deleteFolderName, setDeleteFolderName] = useState<string>("");
  const [renameFolderId, setRenameFolderId] = useState<string | null>(null);
  const [renameFolderName, setRenameFolderName] = useState<string>("");

  const isOffline = useOffline();

  // List view for search/tag filter, grid for normal browse
  const viewMode = searchQuery || tagFilter?.length ? "list" : "grid";

  const loadNotes = async () => {
    const requestId = ++loadRequestRef.current;
    setLoading(true);
    setError(null);
    try {
      // Search / Tag filter mode — backend returns notes only
      if (searchQuery || tagFilter?.length) {
        const { text, tags } = buildSearchRequest(searchQuery, tagFilter);
        const results = await searchFiles(text, tags);
        if (requestId !== loadRequestRef.current) return;
        setFolders([]);
        setNotes(results || []);
        return;
      }

      // Offline Mode — notes only from IndexedDB
      if (isOffline) {
        const localNotes = await getAllNotesLocal();
        if (requestId !== loadRequestRef.current) return;
        const fileItems: FileItem[] = localNotes.map((n) => ({
          id: n.id,
          name: n.name,
          mimeType: "application/vnd.google-apps.document",
          parents: [],
          modifiedTime: n.modifiedTime,
          size: 0,
          version: 1,
          etag: "",
          createdTime: n.modifiedTime,
          kind: "drive#file",
          trashed: false,
        }));
        setFolders([]);
        setNotes(
          fileItems.sort(
            (a, b) =>
              new Date(b.modifiedTime).getTime() -
              new Date(a.modifiedTime).getTime(),
          ),
        );
        return;
      }

      // Online browse mode — split into folders and notes
      const items = await listFiles(folderId);
      if (requestId !== loadRequestRef.current) return;
      const { folders: folderItems, notes: noteItems } = partitionFilesByKind(
        items || [],
      );
      setFolders(folderItems);
      setNotes(noteItems);
    } catch (error) {
      if (requestId !== loadRequestRef.current) return;
      const err = error as Error;
      setError(err.message || String(error));
    } finally {
      if (requestId === loadRequestRef.current) setLoading(false);
    }
  };

  useEffect(() => {
    loadNotes();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [folderId, searchQuery, tagFilter, isOffline]);

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
      router.push(`/drive?folderId=${id}`);
    } else {
      router.push("/drive");
    }
  };

  const handleCreateNote = async (name: string) => {
    if (isSubmitting) return;
    setIsSubmitting(true);
    try {
      const note = await apiCreateNote(name, "# " + name, folderId);
      router.push(`/note?id=${note.id}`);
    } catch (error) {
      const err = error as Error;
      console.error("Failed to create note:", err);
      alert(err.message || "Failed to create note. Please try again.");
      setIsSubmitting(false);
    }
  };

  const handleCreateFolder = async (name: string) => {
    if (!name.trim() || isSubmittingFolder) return;
    setIsSubmittingFolder(true);
    try {
      await createFolder(name, folderId);
      setIsCreatingFolder(false);
      setNewFolderName("");
      loadNotes();
      onAfterMutation?.();
    } catch (error) {
      const err = error as Error;
      alert(err.message || "Failed to create folder");
    } finally {
      setIsSubmittingFolder(false);
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
      onAfterMutation?.();
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
      onAfterMutation?.();
    } catch (error) {
      const err = error as Error;
      console.error(err);
      alert(err.message || "Failed to rename note");
    }
  };

  const handleToggleStar = async (e: React.MouseEvent, note: FileItem) => {
    e.preventDefault();
    e.stopPropagation();
    setActiveMenuId(null);
    try {
      await starFile(note.id, !note.starred);
      loadNotes();
      onAfterMutation?.();
    } catch (error) {
      const err = error as Error;
      console.error(err);
      alert(err.message || "Failed to toggle star");
    }
  };

  const confirmDeleteNote = async () => {
    if (!deleteNoteId) return;
    try {
      if (isOffline) {
        await deleteNoteLocal(deleteNoteId);
      } else {
        await deleteFile(deleteNoteId);
        await deleteNoteLocal(deleteNoteId);
        onAfterMutation?.();
      }
      setDeleteNoteId(null);
      loadNotes();
    } catch (error) {
      const err = error as Error;
      console.error(err);
      alert(err.message || "Failed to delete note");
    }
  };

  const requestDeleteFolder = (e: React.MouseEvent, folder: FileItem) => {
    e.preventDefault();
    e.stopPropagation();
    setActiveMenuId(null);
    setDeleteFolderId(folder.id);
    setDeleteFolderName(folder.name || "Untitled Folder");
  };

  const confirmDeleteFolder = async () => {
    if (!deleteFolderId) return;
    try {
      await deleteFile(deleteFolderId);
      setDeleteFolderId(null);
      loadNotes();
      onAfterMutation?.();
    } catch (error) {
      const err = error as Error;
      console.error(err);
      alert(err.message || "Failed to delete folder");
    }
  };

  const requestRenameFolder = (e: React.MouseEvent, folder: FileItem) => {
    e.preventDefault();
    e.stopPropagation();
    setActiveMenuId(null);
    setRenameFolderId(folder.id);
    setRenameFolderName(folder.name || "Untitled Folder");
  };

  const executeFolderRename = async (newName: string) => {
    if (!renameFolderId) return;
    try {
      await renameNote(renameFolderId, newName);
      setRenameFolderId(null);
      loadNotes();
      onAfterMutation?.();
    } catch (error) {
      const err = error as Error;
      console.error(err);
      alert(err.message || "Failed to rename folder");
    }
  };

  const handleToggleFolderStar = async (
    e: React.MouseEvent,
    folder: FileItem,
  ) => {
    e.preventDefault();
    e.stopPropagation();
    setActiveMenuId(null);
    try {
      await starFile(folder.id, !folder.starred);
      loadNotes();
      onAfterMutation?.();
    } catch (error) {
      const err = error as Error;
      console.error(err);
      alert(err.message || "Failed to toggle star");
    }
  };

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
        isOpen={!!deleteNoteId}
        title="Delete"
        message={`Are you sure you want to delete "${deleteNoteName}"? This cannot be undone.`}
        onConfirm={confirmDeleteNote}
        onCancel={() => setDeleteNoteId(null)}
      />
      <RenameDialog
        isOpen={!!renameNoteId}
        initialName={renameNoteName}
        onRename={executeRename}
        onCancel={() => setRenameNoteId(null)}
        title="Rename"
      />
      <ConfirmDialog
        isOpen={!!deleteFolderId}
        title="Delete Folder"
        message={`Are you sure you want to delete "${deleteFolderName}"? This cannot be undone.`}
        onConfirm={confirmDeleteFolder}
        onCancel={() => setDeleteFolderId(null)}
      />
      <RenameDialog
        isOpen={!!renameFolderId}
        initialName={renameFolderName}
        onRename={executeFolderRename}
        onCancel={() => setRenameFolderId(null)}
        title="Rename Folder"
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
          {searchQuery
            ? `Search results for "${searchQuery}"`
            : tagFilter?.length
              ? `Tag: ${tagFilter.join(", ")}`
              : "Files"}
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
        /* Search / tag filter — list view, notes only */
        <div
          style={{ display: "flex", flexDirection: "column", gap: "0.5rem" }}
        >
          {loading && notes.length === 0 ? (
            <div
              className="animate-fade-in"
              style={{
                display: "flex",
                flexDirection: "column",
                gap: "0.5rem",
              }}
            >
              {Array.from({ length: 5 }).map((_, i) => (
                <div
                  key={`skeleton-list-${i}`}
                  className="glass animate-pulse"
                  style={{
                    padding: "1rem",
                    borderRadius: "0.5rem",
                    display: "flex",
                    alignItems: "center",
                    justifyContent: "space-between",
                    background: "var(--card)",
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
                    <div
                      style={{
                        width: "20px",
                        height: "20px",
                        borderRadius: "4px",
                        background: "var(--border)",
                        flexShrink: 0,
                      }}
                    />
                    <div
                      style={{
                        display: "flex",
                        flexDirection: "column",
                        gap: "0.4rem",
                        width: "100%",
                      }}
                    >
                      <div
                        style={{
                          width: "30%",
                          height: "16px",
                          borderRadius: "4px",
                          background: "var(--border)",
                        }}
                      />
                      <div
                        style={{
                          width: "20%",
                          height: "12px",
                          borderRadius: "4px",
                          background: "var(--border)",
                        }}
                      />
                    </div>
                  </div>
                  <div
                    style={{
                      width: "80px",
                      height: "14px",
                      borderRadius: "4px",
                      background: "var(--border)",
                    }}
                  />
                </div>
              ))}
            </div>
          ) : (
            <div
              className="animate-fade-in"
              style={{
                display: "flex",
                flexDirection: "column",
                gap: "0.5rem",
              }}
            >
              {notes.length === 0 && (
                <div className="text-center py-10 text-gray-500">
                  No notes found.
                </div>
              )}
              {notes.map((note) => (
                <div key={note.id} style={{ position: "relative" }}>
                  <Link
                    href={`/note?id=${note.id}`}
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
                      <FileText size={20} style={{ color: "var(--primary)" }} />
                      <div>
                        <div style={{ fontWeight: 500 }}>{note.name}</div>
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
                        setActiveMenuId(
                          activeMenuId === note.id ? null : note.id,
                        );
                      }}
                      onClose={() => setActiveMenuId(null)}
                      onDelete={(e) => requestDeleteNote(e, note)}
                      onDuplicate={(e) => handleDuplicateNote(e, note)}
                      onRename={(e) => requestRenameNote(e, note)}
                      onStar={(e) => handleToggleStar(e, note)}
                      isStarred={note.starred}
                    />
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      ) : (
        /* Browse mode — grid with Folders section then Notes section */
        <div
          className={
            !loading && (folders.length > 0 || notes.length > 0)
              ? "animate-fade-in"
              : ""
          }
        >
          {/* Folders Section — always shown in browse mode to expose New Folder button */}
          <div style={{ marginBottom: "2rem" }}>
            <h3
              style={{
                fontSize: "0.75rem",
                fontWeight: "bold",
                textTransform: "uppercase",
                letterSpacing: "0.05em",
                color: "var(--muted-foreground)",
                display: "flex",
                alignItems: "center",
                gap: "0.375rem",
                marginBottom: "1rem",
              }}
            >
              <Folder size={14} /> Folders
            </h3>

            <div
              style={{
                display: "grid",
                gridTemplateColumns: "repeat(auto-fill, minmax(240px, 1fr))",
                gap: "1rem",
                alignItems: "start",
              }}
            >
              {/* New Folder dashed card — same pattern as New Note */}
              {!isCreatingFolder && (
                <button
                  onClick={() => {
                    setIsCreatingFolder(true);
                    setNewFolderName("");
                  }}
                  className="glass"
                  style={{
                    ...folderCardStyle,
                    cursor: "pointer",
                    border: "1px dashed var(--border)",
                    background: "transparent",
                    color: "var(--muted-foreground)",
                    flexDirection: "row",
                    alignItems: "center",
                    justifyContent: "flex-start",
                    gap: "0.5rem",
                    padding: "1rem 1.25rem",
                  }}
                >
                  <FolderPlus size={20} style={{ color: "var(--primary)" }} />
                  <span>New Folder</span>
                </button>
              )}

              {/* Inline New Folder input */}
              {isCreatingFolder && (
                <div
                  className="glass"
                  style={{
                    ...folderCardStyle,
                    background: "var(--card)",
                    color: "var(--card-foreground)",
                    flexDirection: "column",
                    justifyContent: "center",
                    alignItems: "center",
                    gap: "0.5rem",
                  }}
                >
                  <input
                    autoFocus
                    type="text"
                    placeholder="Folder Name..."
                    value={newFolderName}
                    onChange={(e) => setNewFolderName(e.target.value)}
                    onKeyDown={(e) => {
                      if (e.key === "Enter") {
                        e.preventDefault();
                        if (e.nativeEvent.isComposing) return;
                        if (newFolderName.trim())
                          handleCreateFolder(newFolderName.trim());
                      }
                      if (e.key === "Escape") setIsCreatingFolder(false);
                    }}
                    onBlur={() => {
                      if (!newFolderName.trim()) setIsCreatingFolder(false);
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
              )}

              {/* Folder skeletons */}
              {loading &&
                Array.from({ length: 3 }).map((_, i) => (
                  <div
                    key={`skeleton-folder-${i}`}
                    className="glass animate-pulse"
                    style={{
                      ...folderCardStyle,
                      background: "var(--card)",
                      border: "1px solid var(--border)",
                    }}
                  >
                    <div
                      style={{
                        display: "flex",
                        alignItems: "center",
                        gap: "0.75rem",
                      }}
                    >
                      <div
                        style={{
                          width: "20px",
                          height: "20px",
                          borderRadius: "4px",
                          background: "var(--border)",
                          flexShrink: 0,
                        }}
                      />
                      <div
                        style={{
                          width: "60%",
                          height: "18px",
                          borderRadius: "4px",
                          background: "var(--border)",
                        }}
                      />
                    </div>
                  </div>
                ))}

              {/* Folder cards */}
              {!loading &&
                folders.map((folder) => (
                  <div
                    key={folder.id}
                    role="button"
                    tabIndex={0}
                    onClick={() => navigateToFolder(folder.id)}
                    onKeyDown={(e) => {
                      if (e.key === "Enter" || e.key === " ") {
                        e.preventDefault();
                        navigateToFolder(folder.id);
                      }
                    }}
                    className="glass"
                    style={{
                      ...folderCardStyle,
                      cursor: "pointer",
                      background: "var(--card)",
                      color: "var(--card-foreground)",
                      border: "1px solid var(--border)",
                      flexDirection: "row",
                      alignItems: "center",
                      justifyContent: "space-between",
                      height: "auto",
                      padding: "1rem 1.25rem",
                    }}
                  >
                    <div
                      style={{
                        display: "flex",
                        alignItems: "center",
                        gap: "0.75rem",
                        overflow: "hidden",
                        flex: 1,
                      }}
                    >
                      <Folder
                        size={20}
                        style={{ color: "var(--primary)", flexShrink: 0 }}
                      />
                      <span
                        style={{
                          fontWeight: 600,
                          overflow: "hidden",
                          textOverflow: "ellipsis",
                          whiteSpace: "nowrap",
                        }}
                        title={folder.name}
                      >
                        {folder.name}
                      </span>
                    </div>
                    <div
                      onClick={(e) => e.stopPropagation()}
                      onKeyDown={(e) => e.stopPropagation()}
                    >
                      <NoteMenu
                        isOpen={activeMenuId === folder.id}
                        onToggle={(e) => {
                          e.preventDefault();
                          e.stopPropagation();
                          setActiveMenuId(
                            activeMenuId === folder.id ? null : folder.id,
                          );
                        }}
                        onClose={() => setActiveMenuId(null)}
                        onDelete={(e) => requestDeleteFolder(e, folder)}
                        onRename={(e) => requestRenameFolder(e, folder)}
                        onStar={(e) => handleToggleFolderStar(e, folder)}
                        isStarred={folder.starred}
                        align="right"
                      />
                    </div>
                  </div>
                ))}
            </div>
          </div>

          {/* Notes Section */}
          <div>
            {!searchQuery && !tagFilter?.length && (
              <div
                style={{
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "space-between",
                  marginBottom: "1rem",
                }}
              >
                <h3
                  style={{
                    fontSize: "0.75rem",
                    fontWeight: "bold",
                    textTransform: "uppercase",
                    letterSpacing: "0.05em",
                    color: "var(--muted-foreground)",
                    display: "flex",
                    alignItems: "center",
                    gap: "0.375rem",
                  }}
                >
                  <FileText size={14} /> Notes
                </h3>
              </div>
            )}

            <div
              style={{
                display: "grid",
                gridTemplateColumns: "repeat(auto-fill, minmax(240px, 1fr))",
                gap: "1.5rem",
                alignItems: "start",
              }}
            >
              {/* New Note button */}
              {!searchQuery &&
                !tagFilter?.length &&
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

              {/* Note skeletons */}
              {loading &&
                notes.length === 0 &&
                Array.from({ length: 7 }).map((_, i) => (
                  <div
                    key={`skeleton-grid-${i}`}
                    className="glass animate-pulse"
                    style={{
                      ...cardStyle,
                      background: "var(--card)",
                      border: "1px solid var(--border)",
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
                        <div
                          style={{
                            width: "20px",
                            height: "20px",
                            borderRadius: "4px",
                            background: "var(--border)",
                            flexShrink: 0,
                          }}
                        />
                        <div
                          style={{
                            width: "60%",
                            height: "18px",
                            borderRadius: "4px",
                            background: "var(--border)",
                          }}
                        />
                      </div>
                      <div
                        style={{
                          width: "40%",
                          height: "12px",
                          borderRadius: "4px",
                          background: "var(--border)",
                        }}
                      />
                    </div>
                    <div
                      style={{
                        width: "30%",
                        height: "12px",
                        borderRadius: "4px",
                        background: "var(--border)",
                        alignSelf: "flex-end",
                        marginTop: "1rem",
                      }}
                    />
                  </div>
                ))}

              {/* Note cards */}
              {!loading &&
                notes.map((note) => (
                  <div key={note.id} style={{ position: "relative" }}>
                    <Link
                      href={`/note?id=${note.id}`}
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
                          <FileText style={{ color: "var(--primary)" }} />
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
                      style={{
                        position: "absolute",
                        top: "0.5rem",
                        right: "0.5rem",
                      }}
                    >
                      <NoteMenu
                        isOpen={activeMenuId === note.id}
                        onToggle={(e) => {
                          e.preventDefault();
                          e.stopPropagation();
                          setActiveMenuId(
                            activeMenuId === note.id ? null : note.id,
                          );
                        }}
                        onClose={() => setActiveMenuId(null)}
                        onDelete={(e) => requestDeleteNote(e, note)}
                        onDuplicate={(e) => handleDuplicateNote(e, note)}
                        onRename={(e) => requestRenameNote(e, note)}
                        onStar={(e) => handleToggleStar(e, note)}
                        isStarred={note.starred}
                      />
                    </div>
                  </div>
                ))}
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

const folderCardStyle: React.CSSProperties = {
  padding: "0.875rem 1.25rem",
  borderRadius: "0.5rem",
  display: "flex",
  transition: "transform 0.1s, box-shadow 0.1s",
  border: "1px solid var(--border)",
};

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
