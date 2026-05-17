import React, { useEffect, useState, useRef } from "react";
import { useRouter } from "next/navigation";
import {
  Folder,
  ChevronRight,
  Star,
  Settings,
  FileText,
  Clock,
  Hash,
} from "lucide-react";
import {
  listStarred,
  starFile,
  FileItem,
  listRecent,
  listTags,
  TagCount,
} from "@/lib/api";
import SearchInput from "./SearchInput";
import { useLocalStorageBoolean } from "@/hooks/useLocalStorageBoolean";

interface SidebarProps {
  onNavigate: (folderId?: string) => void;
  isOpen?: boolean;
  onClose?: () => void;
  refreshTrigger?: number;
}

const RECENT_COLLAPSED_KEY = "sidebar:recent:collapsed";

export function Sidebar({
  onNavigate,
  isOpen = true,
  onClose,
  refreshTrigger = 0,
}: SidebarProps) {
  const handleNavigate = (folderId?: string) => {
    onNavigate(folderId);
    onClose?.();
  };
  const router = useRouter();
  const [starredItems, setStarredItems] = useState<FileItem[]>([]);
  const [recentFiles, setRecentFiles] = useState<FileItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [tags, setTags] = useState<TagCount[]>([]);
  const [recentCollapsed, setRecentCollapsed] = useLocalStorageBoolean(
    RECENT_COLLAPSED_KEY,
    false,
  );

  const loadRequestRef = useRef(0);

  const loadFolders = async () => {
    const requestId = ++loadRequestRef.current;
    setLoading(true);
    try {
      const starred = await listStarred();
      const recent = await listRecent(5);
      const tagList = await listTags(50);

      if (requestId !== loadRequestRef.current) return;

      setStarredItems(
        (starred || []).sort((a, b) => a.name.localeCompare(b.name)),
      );
      setRecentFiles(recent || []);
      setTags(tagList || []);
    } catch (err) {
      if (requestId !== loadRequestRef.current) return;
      console.error("Failed to load sidebar:", err);
    } finally {
      if (requestId === loadRequestRef.current) setLoading(false);
    }
  };

  useEffect(() => {
    loadFolders();
  }, [refreshTrigger]);

  return (
    <>
      <div
        className={`sidebar-overlay ${isOpen ? "open" : ""}`}
        onClick={onClose}
      />
      <div className={`sidebar ${isOpen ? "open" : ""}`}>
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
              GophDrive
            </h2>
          </div>
          <SearchInput />
        </div>

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
            ) : starredItems.length === 0 ? (
              <div
                style={{
                  padding: "0.5rem",
                  fontSize: "0.8rem",
                  opacity: 0.5,
                  fontStyle: "italic",
                }}
              >
                Nothing starred
              </div>
            ) : (
              <div className="animate-fade-in">
                {starredItems.map((item) => {
                  const isFolder =
                    item.mimeType === "application/vnd.google-apps.folder";
                  const navigate = () => {
                    if (isFolder) {
                      handleNavigate(item.id);
                    } else {
                      router.push(`/note?id=${item.id}`);
                      onClose?.();
                    }
                  };
                  return (
                    <div
                      key={`starred-${item.id}`}
                      role="button"
                      tabIndex={0}
                      onClick={navigate}
                      onKeyDown={(e) => {
                        if (e.key === "Enter" || e.key === " ") {
                          e.preventDefault();
                          navigate();
                        }
                      }}
                      style={{
                        display: "flex",
                        alignItems: "center",
                        gap: "0.5rem",
                        padding: "0.5rem",
                        borderRadius: "0.25rem",
                        cursor: "pointer",
                        background: "transparent",
                        color: "var(--foreground)",
                      }}
                      className="hover:bg-[var(--muted)]"
                    >
                      <button
                        onClick={(e) => {
                          e.stopPropagation();
                          starFile(item.id, !item.starred)
                            .then(() => loadFolders())
                            .catch((err: Error) => {
                              console.error("Failed to toggle star", err);
                              alert(err.message || "Failed to toggle star");
                            });
                        }}
                        style={{
                          background: "none",
                          border: "none",
                          cursor: "pointer",
                          padding: 0,
                          display: "flex",
                          flexShrink: 0,
                        }}
                        aria-label={`Unstar ${item.name}`}
                        title="Unstar"
                      >
                        <Star
                          size={16}
                          fill="var(--yellow)"
                          color="var(--yellow)"
                        />
                      </button>
                      {isFolder ? (
                        <Folder
                          size={14}
                          style={{ flexShrink: 0, opacity: 0.7 }}
                        />
                      ) : (
                        <FileText
                          size={14}
                          style={{ flexShrink: 0, opacity: 0.7 }}
                        />
                      )}
                      <span
                        style={{
                          overflow: "hidden",
                          textOverflow: "ellipsis",
                          whiteSpace: "nowrap",
                          fontSize: "0.9rem",
                          flex: 1,
                        }}
                      >
                        {item.name}
                      </span>
                    </div>
                  );
                })}
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
                    href={`/drive?tag=${encodeURIComponent(tag.name)}`}
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
