import React, { useEffect, useState, useRef } from "react";
import { useRouter } from "next/navigation";
import {
  Folder,
  FolderPlus,
  ChevronRight,
  Star,
  Settings,
  FileText,
  Clock,
  Hash,
} from "lucide-react";
import {
  createFolder,
  deleteFile,
  renameNote,
  listStarred,
  starFile,
  FileItem,
  listFiles,
  listRecent,
  listTags,
  TagCount,
} from "@/lib/api";
import { ConfirmDialog } from "./ConfirmDialog";
import { RenameDialog } from "./RenameDialog";
import SearchInput from "./SearchInput";
import { NoteMenu } from "./NoteMenu";
import { useLocalStorageBoolean } from "@/hooks/useLocalStorageBoolean";

interface SidebarProps {
  currentFolderId?: string;
  onNavigate: (folderId?: string, folderName?: string) => void;
  breadcrumbs: { id?: string; name: string }[];
  isOpen?: boolean;
  onClose?: () => void;
}

const RECENT_COLLAPSED_KEY = "sidebar:recent:collapsed";

export function Sidebar({
  currentFolderId,
  onNavigate,
  breadcrumbs,
  isOpen = true,
  onClose,
}: SidebarProps) {
  const handleNavigate = (folderId?: string, folderName?: string) => {
    onNavigate(folderId, folderName);
    onClose?.();
  };
  const router = useRouter();
  const [folders, setFolders] = useState<FileItem[]>([]);
  const [starredFolders, setStarredFolders] = useState<FileItem[]>([]);
  const [recentFiles, setRecentFiles] = useState<FileItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [isCreating, setIsCreating] = useState(false);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [newFolderName, setNewFolderName] = useState("");
  const [activeMenuId, setActiveMenuId] = useState<string | null>(null);
  const [tags, setTags] = useState<TagCount[]>([]);
  const [recentCollapsed, setRecentCollapsed] = useLocalStorageBoolean(
    RECENT_COLLAPSED_KEY,
    false,
  );
  const [deleteConfirmation, setDeleteConfirmation] = useState<{
    isOpen: boolean;
    folder: FileItem | null;
  }>({ isOpen: false, folder: null });

  // Rename State
  const [renameFolderId, setRenameFolderId] = useState<string | null>(null);
  const [renameFolderName, setRenameFolderName] = useState<string>("");

  const menuRef = useRef<HTMLDivElement>(null);

  const loadFolders = async () => {
    setLoading(true);
    try {
      const items = await listFiles(currentFolderId);
      const folderItems = (items || []).filter(
        (item) => item.mimeType === "application/vnd.google-apps.folder",
      );
      setFolders(folderItems.sort((a, b) => a.name.localeCompare(b.name)));

      const starred = await listStarred();
      const starredItems = (starred || []).filter(
        (item) => item.mimeType === "application/vnd.google-apps.folder",
      );
      setStarredFolders(
        starredItems.sort((a, b) => a.name.localeCompare(b.name)),
      );

      // Fetch Recent Files (top 5)
      const recent = await listRecent(5);
      setRecentFiles(recent || []);

      const tagList = await listTags(50);
      setTags(tagList || []);
    } catch (err) {
      console.error("Failed to load folders:", err);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadFolders();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [currentFolderId]);

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

  const requestDeleteFolder = (e: React.MouseEvent, folder: FileItem) => {
    e.stopPropagation();
    setActiveMenuId(null);
    setDeleteConfirmation({ isOpen: true, folder });
  };

  const confirmDeleteFolder = async () => {
    const folder = deleteConfirmation.folder;
    if (!folder) return;

    try {
      await deleteFile(folder.id);
      loadFolders();
      if (currentFolderId === folder.id) {
        onNavigate(undefined, "Home");
      }
    } catch (error) {
      const err = error as Error;
      alert(err.message || "Failed to delete folder");
    } finally {
      setDeleteConfirmation({ isOpen: false, folder: null });
    }
  };

  const requestRenameFolder = (e: React.MouseEvent, folder: FileItem) => {
    e.preventDefault();
    e.stopPropagation();
    setActiveMenuId(null);
    setRenameFolderId(folder.id);
    setRenameFolderName(folder.name || "Untitled Folder");
  };

  const executeRename = async (newName: string) => {
    if (!renameFolderId) return;
    try {
      await renameNote(renameFolderId, newName);
      setRenameFolderId(null);
      loadFolders();
    } catch (error) {
      const err = error as Error;
      console.error(err);
      alert(err.message || "Failed to rename folder");
    }
  };

  const handleCreateFolder = async (name: string) => {
    if (!name.trim()) return;
    if (isSubmitting) return;
    setIsSubmitting(true);
    try {
      await createFolder(name, currentFolderId);
      setIsCreating(false);
      setNewFolderName("");
      loadFolders();
    } catch (error) {
      const err = error as Error;
      alert(err.message || "Failed to create folder");
    } finally {
      setIsSubmitting(false);
    }
  };

  const handleToggleStar = async (e: React.MouseEvent, folder: FileItem) => {
    e.stopPropagation();
    setActiveMenuId(null);
    try {
      await starFile(folder.id, !folder.starred);
      loadFolders();
    } catch (error) {
      const err = error as Error;
      console.error("Failed to toggle star", err);
      alert(err.message || "Failed to toggle star");
    }
  };

  return (
    <>
      <div
        className={`sidebar-overlay ${isOpen ? "open" : ""}`}
        onClick={onClose}
      />
      <div className={`sidebar ${isOpen ? "open" : ""}`}>
        <ConfirmDialog
          isOpen={deleteConfirmation.isOpen}
          title="Delete Folder"
          message={`Are you sure you want to delete folder "${deleteConfirmation.folder?.name}"? taking all its contents with it? This cannot be undone.`}
          onConfirm={confirmDeleteFolder}
          onCancel={() =>
            setDeleteConfirmation({ isOpen: false, folder: null })
          }
        />
        <RenameDialog
          isOpen={!!renameFolderId}
          initialName={renameFolderName}
          onRename={executeRename}
          onCancel={() => setRenameFolderId(null)}
          title="Rename Folder"
        />
        {/* Header */}
        <div
          style={{
            padding: "1rem",
            borderBottom: "1px solid var(--border)",
            display: "flex",
            flexDirection: "column",
            gap: "1rem",
          }}
        >
          <div
            style={{
              display: "flex",
              alignItems: "center",
              justifyContent: "space-between",
            }}
          >
            <h2
              style={{
                fontSize: "1.25rem",
                fontWeight: "800",
                letterSpacing: "-0.025em",
                display: "flex",
                alignItems: "center",
                gap: "0.5rem",
              }}
            >
              {/* You could add a Logo icon here if you want, e.g. <Cloud size={24} className="text-primary" /> */}
              GophDrive
            </h2>
            <button
              onClick={() => setIsCreating(true)}
              className="btn"
              style={{
                padding: "0.25rem",
                background: "transparent",
                color: "var(--muted-foreground)",
              }}
              title="New Folder"
            >
              <FolderPlus size={18} />
            </button>
          </div>
          <SearchInput />
        </div>

        {/* Breadcrumbs Navigation (Simplified for Sidebar context, showing Back/Up) */}
        {breadcrumbs.length > 1 && (
          <div
            style={{
              padding: "0.5rem 1rem",
              borderBottom: "1px solid var(--border)",
              fontSize: "0.875rem",
            }}
          >
            <button
              onClick={() => {
                // Go up one level
                const parent = breadcrumbs[breadcrumbs.length - 2];
                handleNavigate(parent.id, parent.name);
              }}
              style={{
                display: "flex",
                alignItems: "center",
                gap: "0.5rem",
                background: "transparent",
                border: "none",
                cursor: "pointer",
                color: "var(--muted-foreground)",
              }}
            >
              <ChevronRight size={14} style={{ transform: "rotate(180deg)" }} />{" "}
              Back
            </button>
          </div>
        )}

        <div style={{ flex: 1, overflowY: "auto", padding: "0.5rem" }}>
          {/* Starred Section */}
          <div style={{ marginBottom: "1rem" }}>
            <div
              style={{
                padding: "0.5rem",
                fontSize: "0.75rem",
                fontWeight: "bold",
                color: "var(--muted-foreground)",
                textTransform: "uppercase",
              }}
            >
              Starred
            </div>
            {loading ? (
              Array.from({ length: 2 }).map((_, i) => (
                <div
                  key={`skeleton-star-${i}`}
                  className="animate-pulse"
                  style={{
                    display: "flex",
                    alignItems: "center",
                    gap: "0.5rem",
                    padding: "0.5rem",
                    margin: "0.25rem 0",
                    borderRadius: "0.25rem",
                    background: "var(--muted)",
                  }}
                >
                  <div
                    style={{
                      width: "16px",
                      height: "16px",
                      borderRadius: "50%",
                      background: "var(--border)",
                      flexShrink: 0,
                    }}
                  />
                  <div
                    style={{
                      width: "60%",
                      height: "12px",
                      borderRadius: "4px",
                      background: "var(--border)",
                    }}
                  />
                </div>
              ))
            ) : starredFolders.length === 0 ? (
              <div
                style={{
                  padding: "0.5rem",
                  fontSize: "0.8rem",
                  opacity: 0.5,
                  fontStyle: "italic",
                }}
              >
                No starred folders
              </div>
            ) : (
              <div className="animate-fade-in">
                {starredFolders.map((folder) => (
                  <div
                    key={`starred-${folder.id}`}
                    onClick={() => handleNavigate(folder.id, folder.name)}
                    style={{
                      display: "flex",
                      alignItems: "center",
                      gap: "0.5rem",
                      padding: "0.5rem",
                      borderRadius: "0.25rem",
                      cursor: "pointer",
                      background:
                        currentFolderId === folder.id
                          ? "var(--muted)"
                          : "transparent",
                      color: "var(--foreground)",
                    }}
                    className="hover:bg-[var(--muted)]"
                  >
                    <Star
                      size={16}
                      fill="var(--yellow)"
                      color="var(--yellow)"
                    />
                    <span
                      style={{
                        overflow: "hidden",
                        textOverflow: "ellipsis",
                        whiteSpace: "nowrap",
                        fontSize: "0.9rem",
                      }}
                    >
                      {folder.name}
                    </span>
                  </div>
                ))}
              </div>
            )}
            <div
              style={{
                borderBottom: "1px solid var(--border)",
                margin: "0.5rem 0",
              }}
            />
          </div>

          {/* Recent Section */}
          <div style={{ marginBottom: "1rem" }}>
            <button
              type="button"
              onClick={() => setRecentCollapsed((v) => !v)}
              aria-expanded={!recentCollapsed}
              aria-controls="sidebar-recent-list"
              style={{
                width: "100%",
                background: "none",
                border: "none",
                padding: "0.5rem",
                fontSize: "0.75rem",
                fontWeight: "bold",
                color: "var(--muted-foreground)",
                textTransform: "uppercase",
                display: "flex",
                alignItems: "center",
                gap: "0.5rem",
                cursor: "pointer",
                textAlign: "left",
              }}
            >
              <ChevronRight
                size={14}
                style={{
                  transform: recentCollapsed ? "rotate(0deg)" : "rotate(90deg)",
                  transition: "transform 0.15s ease",
                }}
              />
              <Clock size={14} /> Recent
            </button>
            <div id="sidebar-recent-list" hidden={recentCollapsed}>
              {loading ? (
                Array.from({ length: 3 }).map((_, i) => (
                  <div
                    key={`skeleton-recent-${i}`}
                    className="animate-pulse"
                    style={{
                      display: "flex",
                      alignItems: "center",
                      gap: "0.5rem",
                      padding: "0.5rem",
                      margin: "0.25rem 0",
                      borderRadius: "0.25rem",
                      background: "var(--muted)",
                    }}
                  >
                    <div
                      style={{
                        width: "16px",
                        height: "16px",
                        borderRadius: "4px",
                        background: "var(--border)",
                        flexShrink: 0,
                      }}
                    />
                    <div
                      style={{
                        width: "80%",
                        height: "12px",
                        borderRadius: "4px",
                        background: "var(--border)",
                      }}
                    />
                  </div>
                ))
              ) : recentFiles.length === 0 ? (
                <div
                  style={{
                    padding: "0.5rem",
                    fontSize: "0.8rem",
                    opacity: 0.5,
                    fontStyle: "italic",
                  }}
                >
                  No recent files
                </div>
              ) : (
                <div className="animate-fade-in">
                  {recentFiles.map((file) => (
                    <div
                      key={`recent-${file.id}`}
                      onClick={() => {
                        router.push(`/note?id=${file.id}`);
                        onClose?.();
                      }}
                      style={{
                        display: "flex",
                        alignItems: "center",
                        gap: "0.5rem",
                        padding: "0.5rem",
                        borderRadius: "0.25rem",
                        cursor: "pointer",
                        color: "var(--foreground)",
                      }}
                      className="hover:bg-[var(--muted)]"
                    >
                      <FileText
                        size={16}
                        color="var(--primary)"
                        style={{ flexShrink: 0 }}
                      />
                      <span
                        style={{
                          overflow: "hidden",
                          textOverflow: "ellipsis",
                          whiteSpace: "nowrap",
                          fontSize: "0.9rem",
                        }}
                      >
                        {file.name}
                      </span>
                    </div>
                  ))}
                </div>
              )}
            </div>
            <div
              style={{
                borderBottom: "1px solid var(--border)",
                margin: "0.5rem 0",
              }}
            />
          </div>

          {/* Tags Section */}
          {tags.length > 0 && (
            <div style={{ marginBottom: "1rem" }}>
              <div
                style={{
                  padding: "0.5rem",
                  fontSize: "0.75rem",
                  fontWeight: "bold",
                  color: "var(--muted-foreground)",
                  textTransform: "uppercase",
                  display: "flex",
                  alignItems: "center",
                  gap: "0.5rem",
                }}
              >
                <Hash size={14} /> Tags
              </div>
              <div
                style={{
                  display: "flex",
                  flexWrap: "wrap",
                  gap: "0.375rem",
                  padding: "0 0.5rem 0.5rem",
                }}
              >
                {tags.map((tag) => (
                  <a
                    key={tag.name}
                    href={`/notes?tag=${encodeURIComponent(tag.name)}`}
                    style={{
                      display: "inline-flex",
                      alignItems: "center",
                      gap: "0.25rem",
                      padding: "0.125rem 0.5rem",
                      borderRadius: "9999px",
                      fontSize: "0.75rem",
                      background: "var(--muted)",
                      color: "var(--foreground)",
                      textDecoration: "none",
                      whiteSpace: "nowrap",
                    }}
                    onClick={() => onClose?.()}
                  >
                    #{tag.name}
                    <span style={{ opacity: 0.6, fontSize: "0.65rem" }}>
                      {tag.count}
                    </span>
                  </a>
                ))}
              </div>
              <div
                style={{
                  borderBottom: "1px solid var(--border)",
                  margin: "0.5rem 0",
                }}
              />
            </div>
          )}

          {/* Create Input */}
          {isCreating && (
            <div style={{ padding: "0.5rem" }}>
              <input
                autoFocus
                type="text"
                placeholder="Folder Name"
                value={newFolderName}
                onChange={(e) => setNewFolderName(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === "Enter") {
                    e.preventDefault();
                    if (e.nativeEvent.isComposing) return;
                    handleCreateFolder(newFolderName);
                  }
                  if (e.key === "Escape") setIsCreating(false);
                }}
                onBlur={() => {
                  if (!newFolderName) setIsCreating(false);
                }}
                style={{
                  width: "100%",
                  padding: "0.25rem",
                  borderRadius: "0.25rem",
                  border: "1px solid var(--primary)",
                  background: "var(--background)",
                  color: "var(--foreground)",
                }}
              />
            </div>
          )}

          {/* Folder List */}
          {loading ? (
            <div className="animate-fade-in">
              {Array.from({ length: 5 }).map((_, i) => (
                <div
                  key={`skeleton-folder-${i}`}
                  className="animate-pulse"
                  style={{
                    display: "flex",
                    alignItems: "center",
                    gap: "0.5rem",
                    padding: "0.5rem",
                    margin: "0.25rem 0",
                    borderRadius: "0.25rem",
                    background: "var(--muted)",
                    justifyContent: "space-between",
                  }}
                >
                  <div
                    style={{
                      display: "flex",
                      alignItems: "center",
                      gap: "0.5rem",
                      width: "80%",
                    }}
                  >
                    <div
                      style={{
                        width: "18px",
                        height: "18px",
                        borderRadius: "4px",
                        background: "var(--border)",
                        flexShrink: 0,
                      }}
                    />
                    <div
                      style={{
                        width: "70%",
                        height: "14px",
                        borderRadius: "4px",
                        background: "var(--border)",
                      }}
                    />
                  </div>
                  <div
                    style={{
                      width: "16px",
                      height: "16px",
                      borderRadius: "50%",
                      background: "var(--border)",
                      flexShrink: 0,
                    }}
                  />
                </div>
              ))}
            </div>
          ) : (
            <div className="animate-fade-in">
              {folders.map((folder) => (
                <div
                  key={folder.id}
                  style={{
                    position: "relative",
                    display: "flex",
                    alignItems: "center",
                    justifyContent: "space-between",
                    padding: "0.5rem",
                    borderRadius: "0.25rem",
                    cursor: "pointer",
                    marginTop: "0.25rem",
                    color: "var(--foreground)",
                  }}
                  className="hover:bg-[var(--muted)] group"
                  onClick={() => handleNavigate(folder.id, folder.name)}
                >
                  <div
                    style={{
                      display: "flex",
                      alignItems: "center",
                      gap: "0.5rem",
                      overflow: "hidden",
                    }}
                  >
                    <Folder
                      size={18}
                      style={{ color: "var(--foreground)", flexShrink: 0 }}
                    />
                    <span
                      style={{
                        overflow: "hidden",
                        textOverflow: "ellipsis",
                        whiteSpace: "nowrap",
                      }}
                    >
                      {folder.name}
                    </span>
                  </div>

                  {/* Unified Menu Component */}
                  <NoteMenu
                    isOpen={activeMenuId === folder.id}
                    onToggle={(e) => {
                      e.stopPropagation();
                      setActiveMenuId(
                        activeMenuId === folder.id ? null : folder.id,
                      );
                    }}
                    onClose={() => setActiveMenuId(null)}
                    onStar={(e) => handleToggleStar(e, folder)}
                    isStarred={folder.starred}
                    onDelete={(e) => requestDeleteFolder(e, folder)}
                    onRename={(e) => requestRenameFolder(e, folder)}
                    align="right"
                  />
                </div>
              ))}
            </div>
          )}
        </div>

        <div
          style={{ padding: "0.75rem", borderTop: "1px solid var(--border)" }}
        >
          <button
            onClick={() => router.push("/settings")}
            style={{
              display: "flex",
              alignItems: "center",
              gap: "0.75rem",
              width: "100%",
              padding: "0.5rem",
              borderRadius: "0.375rem",
              background: "transparent",
              border: "none",
              cursor: "pointer",
              color: "var(--muted-foreground)",
              transition: "background-color 0.2s",
            }}
            onMouseEnter={(e) => {
              e.currentTarget.style.backgroundColor = "var(--muted)";
              e.currentTarget.style.color = "var(--foreground)";
            }}
            onMouseLeave={(e) => {
              e.currentTarget.style.backgroundColor = "transparent";
              e.currentTarget.style.color = "var(--muted-foreground)";
            }}
          >
            <Settings size={18} />
            <span>Settings</span>
          </button>
        </div>
      </div>
    </>
  );
}
