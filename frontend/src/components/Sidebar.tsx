import React, { useEffect, useState, useRef } from "react";
import { useRouter } from "next/navigation";
import {
  Folder,
  FolderPlus,
  ChevronRight,
  Star,
  Settings,
} from "lucide-react";
import {
  createFolder,
  deleteFile,
  listStarred,
  starFile,
  FileItem,
  listFiles,
} from "@/lib/api";
import { ConfirmDialog } from "./ConfirmDialog";
import SearchInput from "./SearchInput";
import { NoteMenu } from "./NoteMenu";

interface SidebarProps {
  currentFolderId?: string;
  onNavigate: (folderId?: string, folderName?: string) => void;
  breadcrumbs: { id?: string; name: string }[];
  isOpen?: boolean;
  onClose?: () => void;
}

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
  const [loading, setLoading] = useState(true);
  const [isCreating, setIsCreating] = useState(false);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [newFolderName, setNewFolderName] = useState("");
  const [activeMenuId, setActiveMenuId] = useState<string | null>(null);
  const [deleteConfirmation, setDeleteConfirmation] = useState<{
    isOpen: boolean;
    folder: FileItem | null;
  }>({ isOpen: false, folder: null });
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
            {starredFolders.length === 0 ? (
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
              starredFolders.map((folder) => (
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
                  <Star size={16} fill="var(--yellow)" color="var(--yellow)" />
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
              ))
            )}
            <div
              style={{
                borderBottom: "1px solid var(--border)",
                margin: "0.5rem 0",
              }}
            />
          </div>

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
            <div style={{ padding: "1rem", textAlign: "center", opacity: 0.5 }}>
              Loading...
            </div>
          ) : (
            folders.map((folder) => (
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
                  align="right"
                />
              </div>
            ))
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
