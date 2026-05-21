"use client";

import React, {
  useEffect,
  useState,
  useCallback,
  Suspense,
  useRef,
} from "react";
import { useSearchParams, useRouter } from "next/navigation";
import Link from "next/link";
import {
  Check,
  Save,
  Share2,
  Home,
  ChevronRight,
  FileText,
  Columns2,
  Eye,
} from "lucide-react";
import { useAutoSave } from "@/hooks/useAutoSave";
import { useHeartbeat } from "@/hooks/useHeartbeat";
import { useOffline } from "@/hooks/useOffline";
import { saveNoteLocal, getNoteLocal, deleteNoteLocal } from "@/lib/idb";
import {
  apiFetch,
  parseJson,
  duplicateNote,
  deleteFile,
  renameNote,
  getBreadcrumbs,
  isValidNoteId,
  BreadcrumbItem,
  LinkRef,
  BacklinkEntry,
} from "@/lib/api";
import { Editor, type EditorHandle } from "@/components/Editor";
import { MobileMarkdownToolbar } from "@/components/MobileMarkdownToolbar";
import { Preview } from "@/components/Preview";
import { LockBanner } from "@/components/LockBanner";
import { ConflictDialog } from "@/components/ConflictDialog";
import { NoteMenu } from "@/components/NoteMenu";
import { ConfirmDialog } from "@/components/ConfirmDialog";
import { buildShareLink } from "@/lib/share";

interface RemoteData {
  content: string;
  etag: string;
}

function NoteContent() {
  const searchParams = useSearchParams();
  const router = useRouter();
  const id = searchParams.get("id");

  const [content, setContent] = useState("");
  const [title, setTitle] = useState("");
  const [modifiedTime, setModifiedTime] = useState<string | null>(null);
  const [etag, setEtag] = useState("");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [parentId, setParentId] = useState<string | null>(null);

  const [lockedBy, setLockedBy] = useState<string | null>(null);
  const [lockExpires, setLockExpires] = useState<number | null>(null);
  const [conflictLocal, setConflictLocal] = useState<string | null>(null);
  const [conflictRemote, setConflictRemote] = useState<RemoteData | null>(null);
  const [breadcrumbs, setBreadcrumbs] = useState<BreadcrumbItem[]>([]);
  const [noteLinks, setNoteLinks] = useState<LinkRef[]>([]);
  const [noteBacklinks, setNoteBacklinks] = useState<BacklinkEntry[]>([]);

  const [isEditingTitle, setIsEditingTitle] = useState(false);
  const [mobileTab, setMobileTab] = useState<"edit" | "preview">("edit");
  const [viewMode, setViewMode] = useState<"editor" | "split" | "preview">(
    "split",
  );
  const [shareCopied, setShareCopied] = useState(false);

  const isOffline = useOffline();

  const editorScrollRef = useRef<HTMLDivElement>(null);
  const previewScrollRef = useRef<HTMLDivElement>(null);
  const isSyncing = useRef(false);
  const copyTimerRef = useRef<number | null>(null);
  const editorRef = useRef<EditorHandle | null>(null);

  const handleEditorScroll = () => {
    if (
      isSyncing.current ||
      !editorScrollRef.current ||
      !previewScrollRef.current
    )
      return;
    isSyncing.current = true;
    const { scrollTop, scrollHeight, clientHeight } = editorScrollRef.current;
    const scrollPercentage = scrollTop / (scrollHeight - clientHeight || 1);
    const preview = previewScrollRef.current;
    preview.scrollTop =
      scrollPercentage * (preview.scrollHeight - preview.clientHeight);
    setTimeout(() => {
      isSyncing.current = false;
    }, 50);
  };

  const handlePreviewScroll = () => {
    if (
      isSyncing.current ||
      !editorScrollRef.current ||
      !previewScrollRef.current
    )
      return;
    isSyncing.current = true;
    const { scrollTop, scrollHeight, clientHeight } = previewScrollRef.current;
    const scrollPercentage = scrollTop / (scrollHeight - clientHeight || 1);
    const editor = editorScrollRef.current;
    editor.scrollTop =
      scrollPercentage * (editor.scrollHeight - editor.clientHeight);
    setTimeout(() => {
      isSyncing.current = false;
    }, 50);
  };

  useEffect(() => {
    if (!id) {
      setError("No Note ID provided");
      setLoading(false);
      return;
    }

    async function load() {
      // Try to load from local first if offline or as fallback
      const localNote = await getNoteLocal(id!);

      if (!navigator.onLine) {
        if (localNote) {
          setContent(localNote.content);
          setTitle(localNote.name);
          setLoading(false);
        } else {
          setError("Offline and no local copy found");
          setLoading(false);
        }
        return;
      }

      // All note IDs are server-minted UUID v4 (backend uuid.New()). Guard placed after
      // the offline branch so a locally-cached note still opens when offline.
      if (!isValidNoteId(id!)) {
        setError("Note not found");
        setLoading(false);
        return;
      }

      try {
        const lockRes = await apiFetch(`/sessions/${id}/lock`, {
          method: "POST",
        });
        if (lockRes.status === 409 || lockRes.status === 423) {
          try {
            const lockData = await parseJson<{
              user_id?: string;
              expires_at?: number;
            }>(lockRes);
            if (lockData.user_id) {
              setLockedBy(lockData.user_id);
              setLockExpires(lockData.expires_at ?? null);
            } else {
              setLockedBy("Another User");
              setLockExpires(Math.floor(Date.now() / 1000) + 300);
            }
          } catch {
            setLockedBy("Another User");
            setLockExpires(Math.floor(Date.now() / 1000) + 300);
          }
        } else if (!lockRes.ok) {
          console.error("Failed to acquire lock", lockRes.status);
        }
      } catch (e) {
        console.error("Lock error", e);
      }

      try {
        const res = await apiFetch(`/notes/${id}`);
        if (res.status === 401) {
          setError("Unauthorized. Please login.");
          return;
        }
        if (!res.ok) {
          if (res.status === 404) throw new Error("Note not found");
          throw new Error("Failed to load note");
        }

        const data = await parseJson<{
          content?: string;
          name?: string;
          modifiedTime?: string;
          parents?: string[];
          etag?: string;
          links?: LinkRef[];
          backlinks?: BacklinkEntry[];
        }>(res);
        setContent(data.content || "");
        setTitle(data.name || "Untitled Note");
        if (data.modifiedTime) setModifiedTime(data.modifiedTime);
        setNoteLinks(data.links ?? []);
        setNoteBacklinks(data.backlinks ?? []);

        if (data.parents && data.parents.length > 0) {
          setParentId(data.parents[0]);
        } else {
          setParentId(null);
        }

        const headerEtag = res.headers.get("ETag");
        setEtag(headerEtag || data.etag || "");

        // Update local cache
        saveNoteLocal({
          id: id!,
          content: data.content || "",
          name: data.name || "Untitled Note",
          modifiedTime: new Date().toISOString(),
          dirty: false,
        });
      } catch (error) {
        const e = error as Error;
        if (localNote) {
          setContent(localNote.content);
          setTitle(localNote.name);
          console.warn("Loaded from local cache due to API error");
        } else {
          setError(e.message || String(error));
        }
      } finally {
        setLoading(false);
      }
    }
    load();
  }, [id]);

  useEffect(() => {
    if (parentId) {
      getBreadcrumbs(parentId).then((crumbs) => {
        setBreadcrumbs(crumbs);
      });
    } else {
      setBreadcrumbs([{ id: "", name: "Home" }]);
    }
  }, [parentId]);

  useEffect(() => {
    return () => {
      if (copyTimerRef.current) window.clearTimeout(copyTimerRef.current);
    };
  }, []);

  const saveNote = useCallback(
    async (newContent: string) => {
      if (!id || lockedBy) return;

      // Offline Save
      if (isOffline) {
        await saveNoteLocal({
          id,
          content: newContent,
          name: title,
          modifiedTime: new Date().toISOString(),
          dirty: true,
        });
        console.log("Saved locally (offline)");
        return;
      }

      try {
        console.log("Saving...", { id, etag });
        const res = await apiFetch(`/notes/${id}`, {
          method: "PUT",
          body: JSON.stringify({ content: newContent }),
          headers: {
            "Content-Type": "application/json",
            "If-Match": etag,
          },
        });

        if (res.status === 412) {
          console.warn("Conflict detected (412)");
          const remoteRes = await apiFetch(`/notes/${id}`);
          if (remoteRes.ok) {
            const remoteData = await parseJson<{
              content?: string;
              etag?: string;
            }>(remoteRes);
            const remoteEtag = remoteRes.headers.get("ETag") || remoteData.etag;
            setConflictRemote({
              content: remoteData.content ?? "",
              etag: remoteEtag ?? "",
            });
            setConflictLocal(newContent);
          }
          throw new Error("Conflict detected");
        }

        if (!res.ok) throw new Error(`Failed to save: ${res.status}`);

        const data = await parseJson<{ etag?: string; links?: LinkRef[] }>(res);
        const newEtag = res.headers.get("ETag") || data.etag;
        setEtag(newEtag ?? "");
        if (data.links !== undefined) {
          const incoming = data.links;
          // The save response carries no currentTitle (enrichment is GET-only).
          // Preserve the last enriched currentTitle per targetId so a renamed
          // target keeps its up-to-date label until the next full GET.
          setNoteLinks((prev) => {
            const prevTitle = new Map(
              prev
                .filter((l) => l.targetId && l.currentTitle)
                .map((l) => [l.targetId, l.currentTitle]),
            );
            return incoming.map((l) =>
              l.targetId && !l.currentTitle && prevTitle.has(l.targetId)
                ? { ...l, currentTitle: prevTitle.get(l.targetId) }
                : l,
            );
          });
        }

        // Update local cache (clean)
        const now = new Date().toISOString();
        setModifiedTime(now);
        saveNoteLocal({
          id,
          content: newContent,
          name: title,
          modifiedTime: now,
          dirty: false,
        });

        console.log("Saved successfully. New Etag:", newEtag);
      } catch (e) {
        console.error("Save failed", e);
        const isNetworkError = e instanceof TypeError;
        if (isNetworkError || isOffline) {
          await saveNoteLocal({
            id,
            content: newContent,
            name: title,
            modifiedTime: new Date().toISOString(),
            dirty: true,
          });
          console.log("Saved locally due to error");
        }
        throw e;
      }
    },
    [id, lockedBy, etag, isOffline, title],
  );

  // Autosave
  const {
    isSaving,
    hasUnsavedChanges,
    error: autoSaveError,
  } = useAutoSave(content, saveNote, 2000, !loading && !error && !!id);

  // Heartbeat
  useHeartbeat(id ?? "", !lockedBy && !loading && !error && !!id);

  // Conflict Handlers
  const handleKeepLocal = () => {
    if (!conflictRemote || !id) return;

    const newContent = conflictLocal || content;

    apiFetch(`/notes/${id}`, {
      method: "PUT",
      body: JSON.stringify({ content: newContent }),
      headers: {
        "Content-Type": "application/json",
        "If-Match": conflictRemote.etag,
      },
    }).then(async (res) => {
      if (res.ok) {
        const data = await parseJson<{ etag?: string }>(res);
        setEtag(res.headers.get("ETag") || data.etag || "");
        setConflictRemote(null);
        setConflictLocal(null);
      } else {
        const text = await res.text();
        alert(`Failed to overwrite: ${text}`);
      }
    });
  };

  const handleKeepRemote = () => {
    if (!conflictRemote) return;
    setContent(conflictRemote.content);
    setEtag(conflictRemote.etag);
    setConflictRemote(null);
    setConflictLocal(null);
  };

  const handleTitleRename = async (newName: string) => {
    if (!id) return;
    if (newName === title || !newName.trim()) {
      setIsEditingTitle(false);
      return;
    }
    try {
      if (isOffline) {
        // Offline rename logic
        setTitle(newName);
        await saveNoteLocal({
          id,
          content,
          name: newName,
          modifiedTime: new Date().toISOString(),
          dirty: true,
        });
      } else {
        await renameNote(id, newName);
        setTitle(newName);
        // Update local
        await saveNoteLocal({
          id,
          content,
          name: newName,
          modifiedTime: new Date().toISOString(),
          dirty: false,
        });
      }
    } catch (error) {
      const e = error as Error;
      console.error("Rename failed", e);
      alert(e.message || "Failed to rename note");
    } finally {
      setIsEditingTitle(false);
    }
  };

  const [isMenuOpen, setIsMenuOpen] = useState(false);
  const [isDeleteDialogOpen, setIsDeleteDialogOpen] = useState(false);

  const handleDuplicate = async () => {
    if (!id) return;
    setIsMenuOpen(false);
    try {
      const newNote = await duplicateNote(id);
      router.push(`/note?id=${newNote.id}`);
    } catch (error) {
      const e = error as Error;
      console.error("Duplicate failed", e);
      alert(e.message || "Failed to duplicate note");
    }
  };

  const handleDelete = async () => {
    setIsMenuOpen(false);
    setIsDeleteDialogOpen(true);
  };

  const confirmDelete = async () => {
    if (!id) return;
    try {
      if (isOffline) {
        await deleteNoteLocal(id);
      } else {
        await deleteFile(id);
        // Also remove from local
        await deleteNoteLocal(id);
      }
      router.push("/drive");
    } catch (error) {
      const e = error as Error;
      console.error("Delete failed", e);
      alert(e.message || "Failed to delete note");
    }
  };

  if (loading)
    return (
      <div
        style={{
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
          padding: "3rem",
          height: "100vh",
        }}
      >
        <div className="w-8 h-8 border-4 border-[var(--primary)] border-t-transparent rounded-full animate-spin"></div>
      </div>
    );

  if (error)
    return (
      <div
        style={{
          display: "flex",
          flexDirection: "column",
          alignItems: "center",
          justifyContent: "center",
          height: "100vh",
          gap: "1rem",
        }}
      >
        <p style={{ color: "var(--destructive)", fontSize: "1.125rem" }}>
          {error}
        </p>
        <Link href="/" className="btn btn-primary">
          Go Home
        </Link>
      </div>
    );

  return (
    <div
      style={{
        display: "flex",
        flexDirection: "column",
        height: "100vh",
        background: "var(--background)",
        color: "var(--foreground)",
      }}
    >
      {/* Header */}
      <header
        className="note-header"
        style={{
          height: "auto",
          minHeight: "3.5rem",
          borderBottom: "1px solid var(--border)",
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          padding: "1rem",
          background: "var(--header-background)",
          flexShrink: 0,
          flexWrap: "wrap",
          gap: "0.5rem",
        }}
      >
        <div
          style={{
            display: "flex",
            flexDirection: "column",
            gap: "0.25rem",
            flex: 1,
            minWidth: 0,
          }}
        >
          <div
            style={{
              display: "flex",
              alignItems: "center",
              gap: "0.5rem",
              flexWrap: "wrap",
            }}
            className="note-breadcrumbs"
          >
            {breadcrumbs.map((bc, idx) => (
              <React.Fragment key={idx}>
                {idx > 0 && (
                  <span style={{ opacity: 0.5 }}>
                    <ChevronRight size={16} />
                  </span>
                )}
                <Link
                  href={bc.id ? `/drive?folderId=${bc.id}` : "/drive"}
                  className="hover:underline"
                  style={{
                    display: "flex",
                    alignItems: "center",
                    gap: "0.5rem",
                    background: "transparent",
                    border: "none",
                    color: "inherit",
                    textDecoration: "none",
                    opacity: 0.7,
                  }}
                >
                  {bc.id === "" && <Home size={16} />}
                  <span
                    style={{
                      fontSize: "1rem",
                      overflow: "hidden",
                      textOverflow: "ellipsis",
                      whiteSpace: "nowrap",
                      maxWidth: "10rem",
                    }}
                  >
                    {bc.name}
                  </span>
                </Link>
              </React.Fragment>
            ))}

            <span style={{ opacity: 0.5 }}>
              <ChevronRight size={16} />
            </span>
            {isEditingTitle ? (
              <input
                type="text"
                defaultValue={title}
                autoFocus
                onBlur={(e) => handleTitleRename(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === "Enter") {
                    handleTitleRename(e.currentTarget.value);
                  } else if (e.key === "Escape") {
                    setIsEditingTitle(false);
                  }
                }}
                style={{
                  fontSize: "1rem",
                  fontWeight: "bold",
                  margin: 0,
                  border: "1px solid var(--primary)",
                  borderRadius: "0.25rem",
                  padding: "0 0.25rem",
                  background: "var(--background)",
                  color: "var(--foreground)",
                  maxWidth: "20rem",
                }}
              />
            ) : (
              <h1
                onClick={() => !lockedBy && setIsEditingTitle(true)}
                title="Click to rename"
                style={{
                  fontWeight: "bold",
                  overflow: "hidden",
                  textOverflow: "ellipsis",
                  whiteSpace: "nowrap",
                  maxWidth: "20rem",
                  fontSize: "1rem",
                  margin: 0,
                  cursor: lockedBy ? "default" : "pointer",
                  border: "1px solid transparent", // To prevent layout shift
                }}
                onMouseEnter={(e) => {
                  if (!lockedBy)
                    e.currentTarget.style.textDecoration = "underline";
                }}
                onMouseLeave={(e) => {
                  e.currentTarget.style.textDecoration = "none";
                }}
              >
                {title}
              </h1>
            )}
          </div>

          <div
            className="note-header-meta"
            style={{
              display: "flex",
              alignItems: "center",
              gap: "0.5rem",
              flexWrap: "wrap",
            }}
          >
            {isSaving ? (
              <span
                style={{
                  fontSize: "0.75rem",
                  opacity: 0.5,
                  display: "flex",
                  alignItems: "center",
                  gap: "0.25rem",
                }}
              >
                <Save size={12} /> Saving...
              </span>
            ) : autoSaveError ? (
              <span
                style={{
                  fontSize: "0.75rem",
                  color: "var(--destructive)",
                  fontWeight: "bold",
                }}
                title={autoSaveError.message}
              >
                Save Failed
              </span>
            ) : hasUnsavedChanges ? (
              <span
                style={{
                  fontSize: "0.75rem",
                  opacity: 0.5,
                  color: "var(--warning)",
                  fontWeight: 500,
                }}
              >
                Unsaved
              </span>
            ) : (
              <span
                style={{
                  fontSize: "0.75rem",
                  opacity: 0.5,
                  display: "flex",
                  alignItems: "center",
                  gap: "0.25rem",
                  color: "var(--success)",
                }}
              >
                <Check size={12} /> Saved
              </span>
            )}
            {modifiedTime && !isSaving && !hasUnsavedChanges && (
              <span
                style={{
                  fontSize: "0.75rem",
                  opacity: 0.4,
                  whiteSpace: "nowrap",
                }}
              >
                Last modified:{" "}
                {new Date(modifiedTime).toLocaleString(undefined, {
                  year: "numeric",
                  month: "numeric",
                  day: "numeric",
                  hour: "2-digit",
                  minute: "2-digit",
                })}
              </span>
            )}
          </div>
        </div>
        <div style={{ display: "flex", alignItems: "center", gap: "1rem" }}>
          {lockedBy && (
            <span
              style={{
                color: "var(--warning)",
                fontSize: "0.875rem",
                fontWeight: "bold",
                background: "rgba(245,158,11,0.1)",
                padding: "0.25rem 0.5rem",
                borderRadius: "0.25rem",
              }}
            >
              Read Only
            </span>
          )}
          <div className="desktop-view-switcher">
            <button
              className={`view-btn ${viewMode === "editor" ? "active" : ""}`}
              onClick={() => setViewMode("editor")}
              title="Editor"
            >
              <FileText size={16} />
            </button>
            <button
              className={`view-btn ${viewMode === "split" ? "active" : ""}`}
              onClick={() => setViewMode("split")}
              title="Split"
            >
              <Columns2 size={16} />
            </button>
            <button
              className={`view-btn ${viewMode === "preview" ? "active" : ""}`}
              onClick={() => setViewMode("preview")}
              title="Preview"
            >
              <Eye size={16} />
            </button>
          </div>
          <button
            className="btn"
            style={{ background: "var(--card)", color: "var(--foreground)" }}
            title={shareCopied ? "Copied!" : "Share (Copy Markdown link)"}
            onClick={() => {
              if (!id) return;
              const url = `${window.location.origin}/note?id=${id}`;
              navigator.clipboard.writeText(buildShareLink(title, url));
              setShareCopied(true);
              if (copyTimerRef.current)
                window.clearTimeout(copyTimerRef.current);
              copyTimerRef.current = window.setTimeout(
                () => setShareCopied(false),
                1500,
              );
            }}
          >
            {shareCopied ? (
              <>
                <Check size={16} aria-hidden="true" /> Copied!
              </>
            ) : (
              <Share2 size={16} />
            )}
          </button>
          <span role="status" aria-live="polite" className="sr-only">
            {shareCopied ? "Link copied" : ""}
          </span>
          <NoteMenu
            isOpen={isMenuOpen}
            onToggle={(e) => {
              e.preventDefault();
              setIsMenuOpen(!isMenuOpen);
            }}
            onClose={() => setIsMenuOpen(false)}
            onDelete={handleDelete}
            onDuplicate={handleDuplicate}
            align="right"
          />
        </div>
      </header>

      <ConfirmDialog
        isOpen={isDeleteDialogOpen}
        title="Delete Note"
        message={`Are you sure you want to delete "${title}"? This cannot be undone.`}
        onConfirm={confirmDelete}
        onCancel={() => setIsDeleteDialogOpen(false)}
      />

      {lockedBy && lockExpires && (
        <LockBanner userId={lockedBy} expiresAt={lockExpires} />
      )}

      {/* Mobile Tab Bar */}
      <div className="editor-tab-bar">
        <button
          className={`editor-tab ${mobileTab === "edit" ? "active" : ""}`}
          onClick={() => setMobileTab("edit")}
        >
          MARKDOWN
        </button>
        <button
          className={`editor-tab ${mobileTab === "preview" ? "active" : ""}`}
          onClick={() => setMobileTab("preview")}
        >
          PREVIEW
        </button>
      </div>

      <div
        className={`view-mode-${viewMode}`}
        style={{ flex: 1, display: "flex", overflow: "hidden" }}
      >
        {/* Editor Pane */}
        <div className={`editor-pane ${mobileTab !== "edit" ? "hidden" : ""}`}>
          <div
            style={{
              background: "var(--card)",
              padding: "0.25rem 1rem",
              fontSize: "0.75rem",
              fontFamily: "monospace",
              opacity: 0.5,
              borderBottom: "1px solid var(--border)",
            }}
            className="note-header-meta editor-pane__label"
          >
            MARKDOWN
          </div>
          <MobileMarkdownToolbar editorRef={editorRef} readOnly={!!lockedBy} />
          <div
            style={{
              flex: 1,
              overflowY: "auto",
              display: "flex",
              flexDirection: "column",
            }}
            ref={editorScrollRef}
            onScroll={handleEditorScroll}
          >
            <Editor
              ref={editorRef}
              value={content}
              onChange={setContent}
              readOnly={!!lockedBy}
              className="min-h-full rounded-none border-0"
            />
          </div>
        </div>

        {/* Preview Pane */}
        <div
          className={`preview-pane ${mobileTab !== "preview" ? "hidden" : ""}`}
        >
          <div
            style={{
              background: "var(--card)",
              padding: "0.25rem 1rem",
              fontSize: "0.75rem",
              fontFamily: "monospace",
              opacity: 0.5,
              borderBottom: "1px solid var(--border)",
            }}
            className="note-header-meta"
          >
            PREVIEW
          </div>
          <div
            style={{
              flex: 1,
              overflowY: "auto",
              display: "flex",
              flexDirection: "column",
            }}
            ref={previewScrollRef}
            onScroll={handlePreviewScroll}
          >
            <div className="flex-1">
              <Preview
                markdown={content}
                links={noteLinks}
                className="h-full max-w-3xl mx-auto"
              />
              {noteBacklinks.length > 0 && (
                <div
                  style={{
                    maxWidth: "48rem",
                    margin: "0 auto",
                    padding: "1rem",
                    borderTop: "1px solid var(--border)",
                  }}
                >
                  <p
                    style={{
                      fontSize: "0.75rem",
                      fontWeight: 600,
                      opacity: 0.5,
                      textTransform: "uppercase",
                      letterSpacing: "0.05em",
                      marginBottom: "0.5rem",
                    }}
                  >
                    Backlinks
                  </p>
                  <ul
                    style={{
                      listStyle: "none",
                      padding: 0,
                      margin: 0,
                      display: "flex",
                      flexWrap: "wrap",
                      gap: "0.5rem",
                    }}
                  >
                    {noteBacklinks.map((bl) => (
                      <li key={bl.id}>
                        <Link
                          href={`/note?id=${bl.id}`}
                          style={{
                            fontSize: "0.875rem",
                            padding: "0.125rem 0.5rem",
                            borderRadius: "0.25rem",
                            background: "var(--card)",
                            border: "1px solid var(--border)",
                            color: "var(--foreground)",
                            textDecoration: "none",
                          }}
                        >
                          {bl.name}
                        </Link>
                      </li>
                    ))}
                  </ul>
                </div>
              )}
            </div>
          </div>
        </div>
      </div>

      <ConflictDialog
        isOpen={!!conflictRemote}
        onKeepLocal={handleKeepLocal}
        onKeepRemote={handleKeepRemote}
      />
    </div>
  );
}

export default function NotePage() {
  return (
    <Suspense fallback={<div>Loading note...</div>}>
      <NoteContent />
    </Suspense>
  );
}
