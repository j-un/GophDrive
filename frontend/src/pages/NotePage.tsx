import {
  Fragment,
  useCallback,
  useEffect,
  useMemo,
  useState,
  Suspense,
  useRef,
} from "react";
import type { KeyboardEvent as ReactKeyboardEvent } from "react";
import { useSearchParams, useNavigate, Link } from "react-router";
import { Check, Share2, ChevronLeft, Eye } from "lucide-react";
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
import { countChars } from "@/lib/countChars";
import {
  Editor,
  type EditorHandle,
  type SelectionInfo,
} from "@/components/Editor";
import { MarkdownToolbar } from "@/components/MarkdownToolbar";
import { Preview } from "@/components/Preview";
import { Sidebar } from "@/components/Sidebar";
import { LockBanner } from "@/components/LockBanner";
import { ConflictDialog } from "@/components/ConflictDialog";
import { NoteMenu } from "@/components/NoteMenu";
import { ConfirmDialog } from "@/components/ConfirmDialog";
import { buildShareLink } from "@/lib/share";
import styles from "./NotePage.module.css";

interface RemoteData {
  content: string;
  etag: string;
}

// Zen mode (see the "===== Zen mode =====" section in NoteContent below) is
// fully implemented but disabled at its single trigger point in
// handleEditorKeyDown — re-enable by flipping this back to true.
// Disabled 2026-08: caused flicker (any mousemove/blur reset the fade
// mid-typing) and made the note column look off-center once the sidebar
// faded to opacity:0 while still reserving its layout width.
const ZEN_MODE_ENABLED = false;

/** Formats an ISO timestamp as a 24h `HH:MM` string for the save indicator. */
function formatSavedTime(iso: string): string {
  return new Date(iso).toLocaleTimeString([], {
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
  });
}

function NoteContent() {
  const [searchParams] = useSearchParams();
  const navigate = useNavigate();
  const id = searchParams.get("id");

  const [content, setContent] = useState("");
  const [title, setTitle] = useState("");
  // Formatted HH:MM of the last successful save; initialized from the note's
  // modifiedTime on load and refreshed on every successful save (see
  // formatSavedTime below).
  const [savedAt, setSavedAt] = useState<string | null>(null);
  const [etag, setEtag] = useState("");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [parentId, setParentId] = useState<string | null>(null);

  const [lockedBy, setLockedBy] = useState<string | null>(null);
  const [lockExpires, setLockExpires] = useState<number | null>(null);
  const [conflictLocal, setConflictLocal] = useState<string | null>(null);
  const [conflictRemote, setConflictRemote] = useState<RemoteData | null>(null);
  const [conflictError, setConflictError] = useState<string | null>(null);
  const [breadcrumbs, setBreadcrumbs] = useState<BreadcrumbItem[]>([]);
  const [noteLinks, setNoteLinks] = useState<LinkRef[]>([]);
  const [noteBacklinks, setNoteBacklinks] = useState<BacklinkEntry[]>([]);

  const [isEditingTitle, setIsEditingTitle] = useState(false);
  const [previewOpen, setPreviewOpen] = useState(false);
  const [shareCopied, setShareCopied] = useState(false);
  const [selection, setSelection] = useState<SelectionInfo | null>(null);

  // Zen mode: fades the rail + top bar out ~1s after the user starts typing
  // in the editor; any mouse movement, Escape, or focus loss brings them
  // back immediately. See handleEditorKeyDown / exitZenMode below.
  const [isTyping, setIsTyping] = useState(false);
  const isTypingRef = useRef(isTyping);
  useEffect(() => {
    isTypingRef.current = isTyping;
  }, [isTyping]);
  const typingTimerRef = useRef<number | null>(null);

  const isOffline = useOffline();

  const copyTimerRef = useRef<number | null>(null);
  const editorRef = useRef<EditorHandle | null>(null);
  const noteLoadRef = useRef(0);
  const breadcrumbsRequestRef = useRef(0);

  useEffect(() => {
    if (!id) {
      // eslint-disable-next-line react-hooks/set-state-in-effect
      setError("No Note ID provided");
      setLoading(false);
      return;
    }

    setLoading(true);
    setError(null);
    const requestId = ++noteLoadRef.current;

    async function load() {
      const localNote = await getNoteLocal(id!);
      if (requestId !== noteLoadRef.current) return;

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
        if (requestId !== noteLoadRef.current) return;
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
        if (requestId !== noteLoadRef.current) return;
        console.error("Lock error", e);
      }

      try {
        const res = await apiFetch(`/notes/${id}`);
        if (requestId !== noteLoadRef.current) return;
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
        if (data.modifiedTime) {
          setSavedAt(formatSavedTime(data.modifiedTime));
        }
        setNoteLinks(data.links ?? []);
        setNoteBacklinks(data.backlinks ?? []);

        if (data.parents && data.parents.length > 0) {
          setParentId(data.parents[0]);
        } else {
          setParentId(null);
        }

        const headerEtag = res.headers.get("ETag");
        setEtag(headerEtag || data.etag || "");

        saveNoteLocal({
          id: id!,
          content: data.content || "",
          name: data.name || "Untitled Note",
          modifiedTime: new Date().toISOString(),
          dirty: false,
        });
      } catch (error) {
        if (requestId !== noteLoadRef.current) return;
        const e = error as Error;
        if (localNote) {
          setContent(localNote.content);
          setTitle(localNote.name);
          console.warn("Loaded from local cache due to API error");
        } else {
          setError(e.message || String(error));
        }
      } finally {
        if (requestId === noteLoadRef.current) setLoading(false);
      }
    }
    load();
  }, [id, isOffline]);

  useEffect(() => {
    if (!parentId) {
      // eslint-disable-next-line react-hooks/set-state-in-effect
      setBreadcrumbs([{ id: "", name: "Home" }]);
      return;
    }
    const requestId = ++breadcrumbsRequestRef.current;
    getBreadcrumbs(parentId)
      .then((crumbs) => {
        if (requestId !== breadcrumbsRequestRef.current) return;
        setBreadcrumbs(crumbs);
      })
      .catch((err) => {
        if (requestId !== breadcrumbsRequestRef.current) return;
        console.error("Failed to load breadcrumbs", err);
        // Leave the previous breadcrumbs in place on failure.
      });
  }, [parentId]);

  useEffect(() => {
    return () => {
      if (copyTimerRef.current) window.clearTimeout(copyTimerRef.current);
      if (typingTimerRef.current) window.clearTimeout(typingTimerRef.current);
    };
  }, []);

  const saveNote = useCallback(
    async (newContent: string) => {
      if (!id || lockedBy) return;

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

        const now = new Date().toISOString();
        setSavedAt(formatSavedTime(now));
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

  const {
    isSaving,
    hasUnsavedChanges,
    error: autoSaveError,
  } = useAutoSave(content, saveNote, 2000, !loading && !error && !!id);

  useHeartbeat(id ?? "", !lockedBy && !loading && !error && !!id);

  // Mirrors autoSaveError in a ref so handleEditorKeyDown (a stable callback
  // with an empty dep array) can read the latest value without needing
  // autoSaveError in its deps.
  const autoSaveErrorRef = useRef(autoSaveError);
  useEffect(() => {
    autoSaveErrorRef.current = autoSaveError;
  }, [autoSaveError]);

  // ===== Zen mode =====
  // First keystroke in the editor starts a 1s timer; when it fires, the rail
  // and top bar fade out. Mouse movement, Escape, or the editor losing focus
  // clear the timer and bring the UI back immediately. Disabled — see
  // ZEN_MODE_ENABLED above.
  const exitZenMode = useCallback(() => {
    if (typingTimerRef.current !== null) {
      window.clearTimeout(typingTimerRef.current);
      typingTimerRef.current = null;
    }
    if (isTypingRef.current) setIsTyping(false);
  }, []);

  const handleEditorKeyDown = useCallback((e: ReactKeyboardEvent) => {
    if (!ZEN_MODE_ENABLED) return;
    if (e.key === "Escape") return;
    // A save error must stay visible, so don't let the rail/top bar fade
    // out while one is showing.
    if (autoSaveErrorRef.current) return;
    if (isTypingRef.current || typingTimerRef.current !== null) return;
    typingTimerRef.current = window.setTimeout(() => {
      typingTimerRef.current = null;
      setIsTyping(true);
    }, 1000);
  }, []);

  useEffect(() => {
    const handleMouseMove = () => exitZenMode();
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Escape") exitZenMode();
    };
    window.addEventListener("mousemove", handleMouseMove);
    window.addEventListener("keydown", handleKeyDown);
    return () => {
      window.removeEventListener("mousemove", handleMouseMove);
      window.removeEventListener("keydown", handleKeyDown);
    };
  }, [exitZenMode]);

  // A save error appearing mid-typing must be visible immediately — exit
  // zen mode as soon as one shows up. handleEditorKeyDown's
  // autoSaveErrorRef check (above) then keeps zen mode from re-engaging
  // while the error is still present.
  useEffect(() => {
    if (autoSaveError) exitZenMode();
  }, [autoSaveError, exitZenMode]);

  const charCount = useMemo(() => countChars(content), [content]);

  const handleKeepLocal = () => {
    if (!conflictRemote || !id) return;

    const newContent = conflictLocal || content;
    setConflictError(null);

    apiFetch(`/notes/${id}`, {
      method: "PUT",
      body: JSON.stringify({ content: newContent }),
      headers: {
        "Content-Type": "application/json",
        "If-Match": conflictRemote.etag,
      },
    })
      .then(async (res) => {
        if (res.ok) {
          const data = await parseJson<{ etag?: string }>(res);
          setEtag(res.headers.get("ETag") || data.etag || "");
          setConflictRemote(null);
          setConflictLocal(null);
          setConflictError(null);
        } else {
          const text = await res.text();
          setConflictError(`Failed to overwrite: ${text}`);
        }
      })
      .catch((err: Error) => {
        console.error("handleKeepLocal network error", err);
        setConflictError(`Network error: ${err.message}`);
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
      navigate(`/note/?id=${newNote.id}`);
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
        await deleteNoteLocal(id);
      }
      navigate("/drive/");
    } catch (error) {
      const e = error as Error;
      console.error("Delete failed", e);
      alert(e.message || "Failed to delete note");
    }
  };

  const handleShare = async () => {
    if (!id) return;
    if (!navigator.clipboard?.writeText) {
      console.error("Clipboard API unavailable");
      return;
    }
    const url = `${window.location.origin}/note/?id=${id}`;
    try {
      await navigator.clipboard.writeText(buildShareLink(title, url));
    } catch (err) {
      console.error("Failed to copy share link", err);
      return;
    }
    setShareCopied(true);
    if (copyTimerRef.current) window.clearTimeout(copyTimerRef.current);
    copyTimerRef.current = window.setTimeout(() => setShareCopied(false), 1500);
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
        <Link to="/" className="btn btn-primary">
          Go Home
        </Link>
      </div>
    );

  let saveDotClass = styles.saveDot;
  let saveLabel = savedAt ? `saved ${savedAt}` : "saved";
  if (isSaving) {
    saveDotClass = `${styles.saveDot} ${styles.saveDotSaving}`;
  } else if (autoSaveError) {
    saveDotClass = `${styles.saveDot} ${styles.saveDotError}`;
    saveLabel = "error";
  } else if (hasUnsavedChanges) {
    saveDotClass = `${styles.saveDot} ${styles.saveDotUnsaved}`;
    saveLabel = "unsaved";
  }

  return (
    <div className={styles.root}>
      <div
        className={`${styles.sidebarWrap} ${isTyping ? styles.sidebarWrapZen : ""}`}
      >
        <Sidebar
          onNavigate={(folderId) =>
            navigate(folderId ? `/drive/?folderId=${folderId}` : "/drive/")
          }
        />
      </div>

      <div className={styles.main}>
        <header
          className={`${styles.topbar} ${isTyping ? styles.topbarZen : ""}`}
        >
          <div className={styles.crumbs}>
            <Link
              to="/drive/"
              className={styles.backBtn}
              aria-label="Back to Drive"
            >
              <ChevronLeft size={15} strokeWidth={1.8} />
            </Link>

            {breadcrumbs.map((bc, idx) => (
              <Fragment key={idx}>
                {idx > 0 && <span className={styles.crumbSep}>/</span>}
                <Link
                  to={bc.id ? `/drive/?folderId=${bc.id}` : "/drive/"}
                  className={styles.crumbLink}
                >
                  {bc.name}
                </Link>
              </Fragment>
            ))}

            <span className={styles.crumbSep}>/</span>

            {isEditingTitle ? (
              <input
                type="text"
                defaultValue={title}
                autoFocus
                className={styles.titleInput}
                onBlur={(e) => handleTitleRename(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === "Enter") {
                    handleTitleRename(e.currentTarget.value);
                  } else if (e.key === "Escape") {
                    setIsEditingTitle(false);
                  }
                }}
              />
            ) : (
              <span
                role="button"
                tabIndex={lockedBy ? -1 : 0}
                onClick={() => !lockedBy && setIsEditingTitle(true)}
                onKeyDown={(e) => {
                  if (lockedBy) return;
                  if (e.key === "Enter" || e.key === " ") {
                    e.preventDefault();
                    setIsEditingTitle(true);
                  }
                }}
                title="Click to rename"
                className={`${styles.noteTitle} ${!lockedBy ? styles.noteTitleEditable : ""}`}
              >
                {title}
              </span>
            )}
          </div>

          <div className={styles.actions}>
            {lockedBy && <span className={styles.readOnlyPill}>Read Only</span>}

            <div className={styles.saveStatus} title={autoSaveError?.message}>
              <span className={saveDotClass} />
              <span
                className={autoSaveError ? styles.saveLabelError : undefined}
              >
                {saveLabel}
              </span>
            </div>

            <button
              type="button"
              className={`${styles.actionBtn} ${previewOpen ? styles.actionBtnActive : ""}`}
              aria-label={previewOpen ? "Edit" : "Preview"}
              aria-pressed={previewOpen}
              title="Preview"
              onClick={() => setPreviewOpen((v) => !v)}
            >
              <Eye size={16} strokeWidth={1.8} />
            </button>

            <button
              type="button"
              className={styles.actionBtn}
              title={shareCopied ? "Copied!" : "Share (Copy Markdown link)"}
              onClick={handleShare}
            >
              {shareCopied ? (
                <Check size={16} strokeWidth={1.8} aria-hidden="true" />
              ) : (
                <Share2 size={16} strokeWidth={1.8} />
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

        {/* Lives outside .body so the mobile persistent bar (below 768px)
            renders as a full-width row above the editor rather than as a
            flex sibling of the editor/preview panes. The desktop floating
            variant is unaffected — position:fixed ignores DOM placement. */}
        {!previewOpen && (
          <MarkdownToolbar
            editorRef={editorRef}
            selection={selection}
            readOnly={!!lockedBy}
            zen={isTyping}
          />
        )}

        <div className={styles.body}>
          <div
            className={`${styles.editorPane} ${previewOpen ? styles.hidden : ""}`}
          >
            <div
              className={styles.editorScroll}
              onKeyDown={handleEditorKeyDown}
              onBlur={exitZenMode}
            >
              {/* Editor pins the mount-time value as its internal CodeMirror
                  instance's initial document, so a note switch needs a fresh
                  instance — key explicitly on note id rather than relying on
                  the loading spinner's incidental unmount/remount above. */}
              <Editor
                key={id ?? ""}
                ref={editorRef}
                value={content}
                onChange={setContent}
                readOnly={!!lockedBy}
                onSelectionChange={setSelection}
              />
            </div>
          </div>

          <div
            className={`${styles.previewPane} ${previewOpen ? "" : styles.hidden}`}
          >
            <div className={styles.previewScroll}>
              <div className={styles.previewColumn}>
                <Preview
                  markdown={content}
                  links={noteLinks}
                  className="h-full"
                  active={previewOpen}
                />
                {noteBacklinks.length > 0 && (
                  <div className={styles.backlinks}>
                    <p className={styles.backlinksHeading}>Backlinks</p>
                    <ul className={styles.backlinksList}>
                      {noteBacklinks.map((bl) => (
                        <li key={bl.id}>
                          <Link
                            to={`/note/?id=${bl.id}`}
                            className={styles.backlinkChip}
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

        <div className={styles.charCount}>{charCount} chars</div>
      </div>

      <ConflictDialog
        isOpen={!!conflictRemote}
        onKeepLocal={handleKeepLocal}
        onKeepRemote={handleKeepRemote}
        errorMessage={conflictError ?? undefined}
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
