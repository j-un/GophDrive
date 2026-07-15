import React, { useEffect, useState, useRef } from "react";
import { useNavigate, Link } from "react-router";
import {
  Search,
  File,
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
import styles from "./Sidebar.module.css";

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
  const navigate = useNavigate();
  const [starredItems, setStarredItems] = useState<FileItem[]>([]);
  const [recentFiles, setRecentFiles] = useState<FileItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [tags, setTags] = useState<TagCount[]>([]);
  const [recentCollapsed, setRecentCollapsed] = useLocalStorageBoolean(
    RECENT_COLLAPSED_KEY,
    false,
  );
  // Desktop rail hover/tap expansion. Independent of `isOpen`, which drives
  // the mobile slide-in drawer only (see Sidebar.module.css breakpoints).
  const [expanded, setExpanded] = useState(false);

  const rootRef = useRef<HTMLDivElement>(null);
  const searchWrapRef = useRef<HTMLDivElement>(null);
  const starredSectionRef = useRef<HTMLDivElement>(null);
  const recentSectionRef = useRef<HTMLDivElement>(null);
  const tagsSectionRef = useRef<HTMLDivElement>(null);

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
    // eslint-disable-next-line react-hooks/set-state-in-effect
    loadFolders();
  }, [refreshTrigger]);

  // Keep RECENT/Starred fresh when the user returns to the tab (e.g. after
  // opening notes or editing on another device). In-app navigation back to
  // /drive already refetches via the mount effect above; this covers the
  // case where the sidebar stays mounted and the tab regains visibility.
  // (visibilitychange only — pairing it with `focus` double-fires on tab
  // switch. The ref is updated in an effect to stay React Compiler-safe.)
  const loadFoldersRef = useRef(loadFolders);
  useEffect(() => {
    loadFoldersRef.current = loadFolders;
  });
  useEffect(() => {
    const refresh = () => {
      if (document.visibilityState === "visible") loadFoldersRef.current();
    };
    document.addEventListener("visibilitychange", refresh);
    return () => document.removeEventListener("visibilitychange", refresh);
  }, []);

  const handleNavigate = (folderId?: string) => {
    onNavigate(folderId);
    setExpanded(false);
    onClose?.();
  };

  const openSearch = () => {
    setExpanded(true);
    searchWrapRef.current?.querySelector("input")?.focus();
  };

  // Cmd/Ctrl+K opens the panel and focuses search, regardless of hover
  // state. The ref mirrors the loadFoldersRef pattern above so the listener
  // is registered once but always calls the latest closure. Escape shares
  // this listener to close the desktop overlay panel when it was opened
  // without the pointer (e.g. via ⌘K or a rail button click while the
  // cursor stayed in the editor) and mouseleave never fires.
  const openSearchRef = useRef(openSearch);
  useEffect(() => {
    openSearchRef.current = openSearch;
  });
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key === "k") {
        e.preventDefault();
        openSearchRef.current();
      } else if (e.key === "Escape") {
        setExpanded(false);
      }
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, []);

  // Clicking outside the rail/panel also closes the panel for the same
  // pointer-never-left-the-panel reason Escape does. Scoped to `expanded`
  // so the listener only exists while there's something to close.
  useEffect(() => {
    if (!expanded) return;
    const handlePointerDown = (e: MouseEvent) => {
      if (rootRef.current && !rootRef.current.contains(e.target as Node)) {
        setExpanded(false);
      }
    };
    document.addEventListener("mousedown", handlePointerDown);
    return () => document.removeEventListener("mousedown", handlePointerDown);
  }, [expanded]);

  const openSection = (ref: React.RefObject<HTMLDivElement | null>) => {
    setExpanded(true);
    ref.current?.scrollIntoView?.({ block: "nearest" });
  };

  const handleNotesClick = () => {
    handleNavigate();
  };

  const handleSettingsClick = () => {
    setExpanded(false);
    navigate("/settings/");
    onClose?.();
  };

  // Touch devices have no hover: tapping the rail (outside a specific
  // button, which stops propagation) toggles the panel open/closed.
  const handleRailClick = () => {
    setExpanded((v) => !v);
  };

  return (
    <div
      ref={rootRef}
      className={styles.root}
      onMouseEnter={() => setExpanded(true)}
      onMouseLeave={() => setExpanded(false)}
    >
      <div
        className={`${styles.overlay} ${isOpen ? styles.overlayOpen : ""}`}
        onClick={onClose}
      />

      <nav
        className={styles.rail}
        data-testid="sidebar-rail"
        onClick={handleRailClick}
      >
        <Link
          to="/drive/"
          className={styles.logo}
          aria-label="GophDrive"
          onClick={(e) => e.stopPropagation()}
        >
          G
        </Link>
        <button
          type="button"
          className={styles.railBtn}
          aria-label="Search"
          title="Search (⌘K)"
          onClick={(e) => {
            e.stopPropagation();
            openSearch();
          }}
        >
          <Search size={17} strokeWidth={1.8} />
        </button>
        <button
          type="button"
          className={`${styles.railBtn} ${styles.railBtnActive}`}
          aria-label="Notes"
          title="Notes"
          onClick={(e) => {
            e.stopPropagation();
            handleNotesClick();
          }}
        >
          <File size={17} strokeWidth={1.8} />
        </button>
        <button
          type="button"
          className={styles.railBtn}
          aria-label="Starred"
          title="Starred"
          onClick={(e) => {
            e.stopPropagation();
            openSection(starredSectionRef);
          }}
        >
          <Star size={17} strokeWidth={1.8} />
        </button>
        <button
          type="button"
          className={styles.railBtn}
          aria-label="Recent"
          title="Recent"
          onClick={(e) => {
            e.stopPropagation();
            openSection(recentSectionRef);
          }}
        >
          <Clock size={17} strokeWidth={1.8} />
        </button>
        <button
          type="button"
          className={styles.railBtn}
          aria-label="Tags"
          title="Tags"
          onClick={(e) => {
            e.stopPropagation();
            openSection(tagsSectionRef);
          }}
        >
          <Hash size={17} strokeWidth={1.8} />
        </button>
        <div className={styles.spacer} />
        <button
          type="button"
          className={styles.settingsBtn}
          aria-label="Settings"
          title="Settings"
          onClick={(e) => {
            e.stopPropagation();
            handleSettingsClick();
          }}
        >
          <Settings size={17} strokeWidth={1.8} />
        </button>
      </nav>

      <div
        className={`${styles.panel} ${expanded ? styles.panelExpanded : ""} ${isOpen ? styles.panelMobileOpen : ""}`}
        data-testid="sidebar-panel"
      >
        <div className={styles.panelSearch} ref={searchWrapRef}>
          <SearchInput />
        </div>

        <div className={styles.panelBody}>
          {/* Starred Section */}
          <div ref={starredSectionRef} className={styles.section}>
            <div className={styles.sectionHeading}>Starred</div>
            {loading ? (
              Array.from({ length: 2 }).map((_, i) => (
                <div key={`skeleton-star-${i}`} className={styles.skeletonRow}>
                  <div className={styles.skeletonDot} />
                  <div
                    className={styles.skeletonBar}
                    style={{ width: "60%" }}
                  />
                </div>
              ))
            ) : starredItems.length === 0 ? (
              <div className={styles.emptyState}>Nothing starred</div>
            ) : (
              starredItems.map((item) => {
                const isFolder =
                  item.mimeType === "application/vnd.google-apps.folder";
                const handleClick = () => {
                  if (isFolder) {
                    handleNavigate(item.id);
                  } else {
                    navigate(`/note/?id=${item.id}`);
                    setExpanded(false);
                    onClose?.();
                  }
                };
                return (
                  <div
                    key={`starred-${item.id}`}
                    role="button"
                    tabIndex={0}
                    onClick={handleClick}
                    onKeyDown={(e) => {
                      if (e.key === "Enter" || e.key === " ") {
                        e.preventDefault();
                        handleClick();
                      }
                    }}
                    className={styles.row}
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
                      className={styles.starToggle}
                      aria-label={`Unstar ${item.name}`}
                      title="Unstar"
                    >
                      <Star size={16} fill="var(--star)" color="var(--star)" />
                    </button>
                    {isFolder ? (
                      <Folder size={14} className={styles.rowIcon} />
                    ) : (
                      <FileText size={14} className={styles.rowIcon} />
                    )}
                    <span className={styles.rowText}>{item.name}</span>
                  </div>
                );
              })
            )}
          </div>

          {/* Recent Section */}
          <div ref={recentSectionRef} className={styles.section}>
            <button
              type="button"
              onClick={() => setRecentCollapsed((v) => !v)}
              aria-expanded={!recentCollapsed}
              aria-controls="sidebar-recent-list"
              className={styles.sectionToggle}
            >
              <ChevronRight
                size={14}
                className={`${styles.chevron} ${!recentCollapsed ? styles.chevronExpanded : ""}`}
              />
              <Clock size={14} className={styles.headingIcon} /> Recent
            </button>
            <div id="sidebar-recent-list" hidden={recentCollapsed}>
              {loading ? (
                Array.from({ length: 3 }).map((_, i) => (
                  <div
                    key={`skeleton-recent-${i}`}
                    className={styles.skeletonRow}
                  >
                    <div className={styles.skeletonDot} />
                    <div
                      className={styles.skeletonBar}
                      style={{ width: "80%" }}
                    />
                  </div>
                ))
              ) : recentFiles.length === 0 ? (
                <div className={styles.emptyState}>No recent files</div>
              ) : (
                recentFiles.map((file) => {
                  const handleClick = () => {
                    navigate(`/note/?id=${file.id}`);
                    setExpanded(false);
                    onClose?.();
                  };
                  return (
                    <div
                      key={`recent-${file.id}`}
                      role="button"
                      tabIndex={0}
                      onClick={handleClick}
                      onKeyDown={(e) => {
                        if (e.key === "Enter" || e.key === " ") {
                          e.preventDefault();
                          handleClick();
                        }
                      }}
                      className={styles.row}
                    >
                      <FileText size={16} className={styles.rowIcon} />
                      <span className={styles.rowText}>{file.name}</span>
                    </div>
                  );
                })
              )}
            </div>
          </div>

          {/* Tags Section */}
          {tags.length > 0 && (
            <div ref={tagsSectionRef} className={styles.section}>
              <div className={styles.sectionHeading}>
                <Hash size={14} className={styles.headingIcon} /> Tags
              </div>
              <div className={styles.tagList}>
                {tags.map((tag) => (
                  <Link
                    key={tag.name}
                    to={`/drive/?tag=${encodeURIComponent(tag.name)}`}
                    className={styles.tagChip}
                    onClick={() => {
                      setExpanded(false);
                      onClose?.();
                    }}
                  >
                    #{tag.name}
                    <span className={styles.tagCount}>{tag.count}</span>
                  </Link>
                ))}
              </div>
            </div>
          )}
        </div>

        {/* Settings is reachable from the rail on desktop; the rail is
            hidden on mobile (≤768px), so the panel carries its own entry
            there (CSS-gated, see .panelMobileFooter). */}
        <div className={styles.panelMobileFooter}>
          <button
            type="button"
            onClick={handleSettingsClick}
            className={styles.mobileSettingsBtn}
          >
            <Settings size={18} strokeWidth={1.8} />
            <span>Settings</span>
          </button>
        </div>
      </div>
    </div>
  );
}
