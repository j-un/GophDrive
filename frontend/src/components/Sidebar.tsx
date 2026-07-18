import { useEffect, useState, useRef } from "react";
import { useNavigate, Link } from "react-router";
import {
  File,
  Folder,
  ChevronRight,
  ChevronLeft,
  PanelLeftOpen,
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
const SIDEBAR_COLLAPSED_KEY = "sidebar:collapsed";

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
  // Desktop rail/panel collapse. Persisted so it survives page navigation
  // (the sidebar remounts on every route change). Independent of `isOpen`,
  // which drives the mobile slide-in drawer only (see Sidebar.module.css
  // breakpoints).
  const [collapsed, setCollapsed] = useLocalStorageBoolean(
    SIDEBAR_COLLAPSED_KEY,
    false,
  );
  // Bumped to trigger a focus effect on the search input. A plain `.focus()`
  // call from `openSearch` doesn't work here because the panel can be
  // `display: none` (collapsed) at the moment ⌘K fires — an element can't
  // receive focus while it isn't rendered, so focusing has to wait for the
  // post-expand render via an effect instead.
  const [searchFocusSignal, setSearchFocusSignal] = useState(0);

  const searchWrapRef = useRef<HTMLDivElement>(null);

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
    // Desktop: the sidebar stays open after navigating (it's a persistent
    // rail now, not a hover overlay). Mobile: still closes the drawer.
    onClose?.();
  };

  const openSearch = () => {
    setCollapsed(false);
    setSearchFocusSignal((s) => s + 1);
  };

  useEffect(() => {
    if (searchFocusSignal === 0) return;
    searchWrapRef.current?.querySelector("input")?.focus();
  }, [searchFocusSignal]);

  // Cmd/Ctrl+K expands the panel and focuses search regardless of current
  // collapsed state. The ref mirrors the loadFoldersRef pattern above so
  // the listener is registered once but always calls the latest closure.
  const openSearchRef = useRef(openSearch);
  useEffect(() => {
    openSearchRef.current = openSearch;
  });
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key === "k") {
        e.preventDefault();
        openSearchRef.current();
      }
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, []);

  const handleNotesClick = () => {
    handleNavigate();
  };

  const handleSettingsClick = () => {
    navigate("/settings/");
    onClose?.();
  };

  return (
    <div className={`${styles.root} ${collapsed ? styles.rootCollapsed : ""}`}>
      <div
        className={`${styles.overlay} ${isOpen ? styles.overlayOpen : ""}`}
        onClick={onClose}
      />

      {/* Rendered at all times (not just when collapsed) so its layout is
          established before the CSS display toggle; only visible via
          `.rootCollapsed .rail { display: flex }` at desktop widths. */}
      <nav className={styles.rail} data-testid="sidebar-rail">
        <button
          type="button"
          className={styles.railBtn}
          aria-label="Expand sidebar"
          title="Expand sidebar"
          onClick={() => setCollapsed(false)}
        >
          <PanelLeftOpen size={17} strokeWidth={1.8} />
        </button>
        <button
          type="button"
          className={`${styles.railBtn} ${styles.railBtnActive}`}
          aria-label="Notes"
          title="Notes"
          onClick={handleNotesClick}
        >
          <File size={17} strokeWidth={1.8} />
        </button>
        <div className={styles.spacer} />
        <button
          type="button"
          className={styles.settingsBtn}
          aria-label="Settings"
          title="Settings"
          onClick={handleSettingsClick}
        >
          <Settings size={17} strokeWidth={1.8} />
        </button>
      </nav>

      {/* Rendered at all times (not just when expanded) because the mobile
          drawer depends on this element already being in the DOM to
          translateX it into view; only hidden via
          `.rootCollapsed .panel { display: none }` at desktop widths. */}
      <div
        className={`${styles.panel} ${isOpen ? styles.panelMobileOpen : ""}`}
        data-testid="sidebar-panel"
      >
        <div className={styles.panelHeader}>
          <Link
            to="/drive/"
            className={styles.panelLogo}
            aria-label="GophDrive"
          >
            G
          </Link>
          <button
            type="button"
            className={styles.collapseBtn}
            aria-label="Collapse sidebar"
            title="Collapse sidebar"
            onClick={() => setCollapsed(true)}
          >
            <ChevronLeft size={17} strokeWidth={1.8} />
          </button>
        </div>
        <div className={styles.panelSearch} ref={searchWrapRef}>
          <SearchInput />
        </div>

        <div className={styles.panelBody}>
          {/* Starred Section */}
          <div className={styles.section}>
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
          <div className={styles.section}>
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
            <div className={styles.section}>
              <div className={styles.sectionHeading}>
                <Hash size={14} className={styles.headingIcon} /> Tags
              </div>
              <div className={styles.tagList}>
                {tags.map((tag) => (
                  <Link
                    key={tag.name}
                    to={`/drive/?tag=${encodeURIComponent(tag.name)}`}
                    className={styles.tagChip}
                    onClick={() => onClose?.()}
                  >
                    #{tag.name}
                    <span className={styles.tagCount}>{tag.count}</span>
                  </Link>
                ))}
              </div>
            </div>
          )}
        </div>

        {/* Always visible now that the panel is a persistent column rather
            than a hover overlay; previously mobile-only because the rail
            (with its own Settings entry) was hidden ≤768px. */}
        <div className={styles.panelFooter}>
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
