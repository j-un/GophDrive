import React, { useEffect, useState, useRef } from "react";
import { Link, useNavigate } from "react-router";
import { Folder, ChevronRight, Star } from "lucide-react";
import {
  FileItem,
  duplicateNote,
  renameNote,
  moveFile,
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
import { MoveDialog } from "@/components/MoveDialog";
import { useOffline } from "@/hooks/useOffline";
import { deleteNoteLocal, getAllNotesLocal } from "@/lib/idb";
import { buildSearchRequest } from "@/lib/searchQuery";
import { partitionFilesByKind } from "@/lib/partitionFiles";
import styles from "./NoteList.module.css";

interface NoteListProps {
  folderId?: string;
  searchQuery?: string;
  tagFilter?: string[];
  onAfterMutation?: () => void;
  /** Bumped by DrivePage (button click or ⌘N) to open the inline "new note" row. */
  createNoteSignal?: number;
}

function formatRowDate(dateStr: string): string {
  return new Date(dateStr).toLocaleDateString(undefined, {
    month: "long",
    day: "numeric",
  });
}

export default function NoteList({
  folderId,
  searchQuery,
  tagFilter,
  onAfterMutation,
  createNoteSignal = 0,
}: NoteListProps) {
  const navigate = useNavigate();
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

  // Move State
  const [moveItemId, setMoveItemId] = useState<string | null>(null);
  const [moveItemName, setMoveItemName] = useState<string>("");
  const [moveItemIsFolder, setMoveItemIsFolder] = useState(false);
  const [moveItemParentId, setMoveItemParentId] = useState<string>("");

  const isOffline = useOffline();

  // Folders section only makes sense when browsing (not searching/tag-filtering).
  const isBrowseMode = !searchQuery && !(tagFilter && tagFilter.length > 0);

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

  const loadNotesRef = useRef(loadNotes);
  useEffect(() => {
    loadNotesRef.current = loadNotes;
  });

  useEffect(() => {
    loadNotesRef.current();
  }, [folderId, searchQuery, tagFilter, isOffline]);

  // Opens the inline "new note" row whenever DrivePage bumps createNoteSignal
  // (button click or ⌘N). Adjusted during render (React's documented pattern
  // for "reset/react to a changed prop") rather than in an effect — skipped
  // on mount since prevCreateSignal starts equal to createNoteSignal.
  const [prevCreateSignal, setPrevCreateSignal] = useState(createNoteSignal);
  if (createNoteSignal !== prevCreateSignal) {
    setPrevCreateSignal(createNoteSignal);
    if (isBrowseMode) {
      setIsCreating(true);
      setNewNoteName("");
    }
  }

  const handleCreateNote = async (name: string) => {
    if (isSubmitting) return;
    setIsSubmitting(true);
    try {
      const note = await apiCreateNote(name, "# " + name, folderId);
      navigate(`/note/?id=${note.id}`);
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

  const requestMoveNote = (e: React.MouseEvent, note: FileItem) => {
    e.preventDefault();
    e.stopPropagation();
    setActiveMenuId(null);
    setMoveItemId(note.id);
    setMoveItemName(note.name || "Untitled Note");
    setMoveItemIsFolder(false);
    setMoveItemParentId(note.parents?.[0] ?? "");
  };

  const requestMoveFolder = (e: React.MouseEvent, folder: FileItem) => {
    e.preventDefault();
    e.stopPropagation();
    setActiveMenuId(null);
    setMoveItemId(folder.id);
    setMoveItemName(folder.name || "Untitled Folder");
    setMoveItemIsFolder(true);
    setMoveItemParentId(folder.parents?.[0] ?? "");
  };

  const executeMove = async (dest: string) => {
    if (!moveItemId) return;
    try {
      await moveFile(moveItemId, dest);
      setMoveItemId(null);
      setMoveItemParentId("");
      loadNotes();
      onAfterMutation?.();
    } catch (error) {
      const err = error as Error;
      console.error(err);
      alert(err.message || "Failed to move item");
    }
  };

  if (error) {
    return (
      <div className={styles.errorBox}>
        <p className={styles.errorText}>{error}</p>
        <button type="button" onClick={loadNotes} className={styles.retryBtn}>
          Retry
        </button>
      </div>
    );
  }

  const showNoteSkeleton = loading && notes.length === 0;
  const showEmptyNotes =
    !loading && notes.length === 0 && !(isCreating && isBrowseMode);

  return (
    <div>
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
      <MoveDialog
        isOpen={!!moveItemId}
        itemId={moveItemId ?? ""}
        itemName={moveItemName}
        itemIsFolder={moveItemIsFolder}
        currentParentId={moveItemParentId}
        onMove={executeMove}
        onCancel={() => {
          setMoveItemId(null);
          setMoveItemParentId("");
        }}
      />

      {isBrowseMode && (
        <div>
          {loading
            ? Array.from({ length: 3 }).map((_, i) => (
                <SkeletonRow key={`skeleton-folder-${i}`} />
              ))
            : folders.map((folder) => (
                <FolderRow
                  key={folder.id}
                  folder={folder}
                  isMenuOpen={activeMenuId === folder.id}
                  onToggleMenu={(e) => {
                    e.preventDefault();
                    e.stopPropagation();
                    setActiveMenuId(
                      activeMenuId === folder.id ? null : folder.id,
                    );
                  }}
                  onCloseMenu={() => setActiveMenuId(null)}
                  onDelete={(e) => requestDeleteFolder(e, folder)}
                  onRename={(e) => requestRenameFolder(e, folder)}
                  onMove={(e) => requestMoveFolder(e, folder)}
                  onToggleStar={(e) => handleToggleFolderStar(e, folder)}
                />
              ))}

          {isCreatingFolder ? (
            <div className={styles.inlineRow}>
              <input
                autoFocus
                type="text"
                placeholder="Folder name..."
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
                className={styles.inlineInput}
              />
            </div>
          ) : (
            <button
              type="button"
              className={styles.newRow}
              onClick={() => {
                setIsCreatingFolder(true);
                setNewFolderName("");
              }}
            >
              + New folder
            </button>
          )}
        </div>
      )}

      <div>
        {isCreating && isBrowseMode && (
          <div className={styles.inlineRow}>
            <input
              autoFocus
              type="text"
              placeholder="Note name..."
              value={newNoteName}
              onChange={(e) => setNewNoteName(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter") {
                  e.preventDefault();
                  if (e.nativeEvent.isComposing) return;
                  if (newNoteName.trim()) handleCreateNote(newNoteName.trim());
                }
                if (e.key === "Escape") setIsCreating(false);
              }}
              onBlur={() => {
                if (!newNoteName.trim()) setIsCreating(false);
              }}
              className={styles.inlineInput}
            />
          </div>
        )}

        {showNoteSkeleton
          ? Array.from({ length: 5 }).map((_, i) => (
              <SkeletonRow key={`skeleton-note-${i}`} />
            ))
          : notes.map((note) => (
              <NoteRow
                key={note.id}
                note={note}
                isMenuOpen={activeMenuId === note.id}
                onToggleMenu={(e) => {
                  e.preventDefault();
                  e.stopPropagation();
                  setActiveMenuId(activeMenuId === note.id ? null : note.id);
                }}
                onCloseMenu={() => setActiveMenuId(null)}
                onDelete={(e) => requestDeleteNote(e, note)}
                onDuplicate={(e) => handleDuplicateNote(e, note)}
                onRename={(e) => requestRenameNote(e, note)}
                onMove={(e) => requestMoveNote(e, note)}
                onToggleStar={(e) => handleToggleStar(e, note)}
              />
            ))}

        {showEmptyNotes && <div className={styles.empty}>No notes found.</div>}
      </div>
    </div>
  );
}

function SkeletonRow() {
  return (
    <div className={styles.skeletonRow} aria-hidden="true">
      <div
        className={styles.skeletonBar}
        style={{ width: "16px", height: "16px", flexShrink: 0 }}
      />
      <div
        className={styles.skeletonBar}
        style={{ flex: 1, height: "14px", maxWidth: "240px" }}
      />
      <div
        className={styles.skeletonBar}
        style={{ width: "64px", height: "12px" }}
      />
    </div>
  );
}

interface FolderRowProps {
  folder: FileItem;
  isMenuOpen: boolean;
  onToggleMenu: (e: React.MouseEvent) => void;
  onCloseMenu: () => void;
  onDelete: (e: React.MouseEvent) => void;
  onRename: (e: React.MouseEvent) => void;
  onMove: (e: React.MouseEvent) => void;
  onToggleStar: (e: React.MouseEvent) => void;
}

// Stretched-link row: `.rowLink` is a real <Link> whose ::after covers the
// whole `.row` (position: relative), so the entire row is clickable/keyboard-
// focusable without nesting interactive elements inside the anchor. NoteMenu
// sits on top via z-index so it stays independently clickable.
function FolderRow({
  folder,
  isMenuOpen,
  onToggleMenu,
  onCloseMenu,
  onDelete,
  onRename,
  onMove,
  onToggleStar,
}: FolderRowProps) {
  return (
    <div className={styles.row}>
      <Link to={`/drive/?folderId=${folder.id}`} className={styles.rowLink}>
        <Folder size={16} strokeWidth={1.8} className={styles.folderIcon} />
        <span className={styles.rowName} title={folder.name}>
          {folder.name}
        </span>
      </Link>
      <ChevronRight size={14} strokeWidth={1.8} className={styles.chevron} />
      <div className={styles.menuWrap}>
        <NoteMenu
          isOpen={isMenuOpen}
          onToggle={onToggleMenu}
          onClose={onCloseMenu}
          onDelete={onDelete}
          onRename={onRename}
          onMove={onMove}
          onStar={onToggleStar}
          isStarred={folder.starred}
          align="right"
        />
      </div>
    </div>
  );
}

interface NoteRowProps {
  note: FileItem;
  isMenuOpen: boolean;
  onToggleMenu: (e: React.MouseEvent) => void;
  onCloseMenu: () => void;
  onDelete: (e: React.MouseEvent) => void;
  onDuplicate: (e: React.MouseEvent) => void;
  onRename: (e: React.MouseEvent) => void;
  onMove: (e: React.MouseEvent) => void;
  onToggleStar: (e: React.MouseEvent) => void;
}

// Stretched-link row: `.rowLink` is a real <Link> whose ::after covers the
// whole `.row` (position: relative), so the entire row is clickable/keyboard-
// focusable (incl. middle-click / ⌘-click to open in a new tab) without
// nesting interactive elements inside the anchor. Tags and NoteMenu sit on
// top via z-index so they stay independently clickable.
function NoteRow({
  note,
  isMenuOpen,
  onToggleMenu,
  onCloseMenu,
  onDelete,
  onDuplicate,
  onRename,
  onMove,
  onToggleStar,
}: NoteRowProps) {
  return (
    <div className={styles.row}>
      <Link to={`/note/?id=${note.id}`} className={styles.rowLink}>
        <span className={styles.rowName} title={note.name}>
          {note.name}
        </span>
        {note.starred && (
          <Star
            size={12}
            strokeWidth={1.8}
            fill="var(--star)"
            style={{ color: "var(--star)", flexShrink: 0 }}
          />
        )}
        <span className={styles.date}>{formatRowDate(note.modifiedTime)}</span>
      </Link>
      {note.tags && note.tags.length > 0 && (
        <div className={styles.tags}>
          {note.tags.map((tag) => (
            <Link
              key={tag}
              to={`/drive/?tag=${encodeURIComponent(tag)}`}
              className={styles.tag}
            >
              #{tag}
            </Link>
          ))}
        </div>
      )}
      <div className={styles.menuWrap}>
        <NoteMenu
          isOpen={isMenuOpen}
          onToggle={onToggleMenu}
          onClose={onCloseMenu}
          onDelete={onDelete}
          onDuplicate={onDuplicate}
          onRename={onRename}
          onMove={onMove}
          onStar={onToggleStar}
          isStarred={note.starred}
        />
      </div>
    </div>
  );
}
