import React, { useState, useEffect, useMemo, useRef, Suspense } from "react";
import NoteList from "@/components/NoteList";
import { Sidebar } from "@/components/Sidebar";
import { Menu } from "lucide-react";
import { useNavigate, useSearchParams } from "react-router";
import { getBreadcrumbs, isValidNoteId, BreadcrumbItem } from "@/lib/api";
import styles from "./DrivePage.module.css";

function NotesContent() {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const folderIdParam = searchParams.get("folderId") || undefined;
  const searchQuery = searchParams.get("q") || undefined;

  const [breadcrumbs, setBreadcrumbs] = useState<BreadcrumbItem[]>([
    { id: "", name: "Home" },
  ]);

  const [sidebarOpen, setSidebarOpen] = useState(false);
  const [sidebarRefreshKey, setSidebarRefreshKey] = useState(0);
  const [createNoteSignal, setCreateNoteSignal] = useState(0);

  const searchParamsStr = searchParams.toString();
  const tagFilter = useMemo(
    () => searchParams.getAll("tag").filter(Boolean),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [searchParamsStr],
  );

  const canCreateNote = !searchQuery && tagFilter.length === 0;
  const canCreateNoteRef = useRef(canCreateNote);
  useEffect(() => {
    canCreateNoteRef.current = canCreateNote;
  });

  const fetchFolderInfo = async (id: string) => {
    if (!isValidNoteId(id)) {
      setBreadcrumbs([{ id: "", name: "Home" }]);
      navigate("/drive/", { replace: true });
      return;
    }
    try {
      const crumbs = await getBreadcrumbs(id);
      setBreadcrumbs(crumbs);
    } catch (e) {
      console.error("Failed to fetch folder info", e);
      setBreadcrumbs([{ id: "", name: "Home" }]);
    }
  };

  const fetchFolderInfoRef = useRef(fetchFolderInfo);
  useEffect(() => {
    fetchFolderInfoRef.current = fetchFolderInfo;
  });

  useEffect(() => {
    if (folderIdParam) {
      fetchFolderInfoRef.current(folderIdParam);
    } else {
      // eslint-disable-next-line react-hooks/set-state-in-effect
      setBreadcrumbs([{ id: "", name: "Home" }]);
    }
  }, [folderIdParam]);

  const handleNavigate = (folderId?: string) => {
    if (folderId) {
      navigate(`/drive/?folderId=${folderId}`);
    } else {
      navigate("/drive/");
    }
  };

  const triggerCreateNote = () => {
    if (!canCreateNoteRef.current) return;
    setCreateNoteSignal((v) => v + 1);
  };

  // ⌘N / Ctrl+N — start a new note, unless the user is typing somewhere else.
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (!(e.metaKey || e.ctrlKey) || e.key !== "n") return;
      const active = document.activeElement as HTMLElement | null;
      const tag = active?.tagName;
      if (tag === "INPUT" || tag === "TEXTAREA" || active?.isContentEditable) {
        return;
      }
      e.preventDefault();
      triggerCreateNote();
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, []);

  const pageTitle = searchQuery
    ? `Search results for "${searchQuery}"`
    : tagFilter.length > 0
      ? `Tag: ${tagFilter.join(", ")}`
      : breadcrumbs[breadcrumbs.length - 1]?.name || "Home";

  return (
    <div className={styles.wrapper}>
      <Sidebar
        onNavigate={handleNavigate}
        isOpen={sidebarOpen}
        onClose={() => setSidebarOpen(false)}
        refreshTrigger={sidebarRefreshKey}
      />

      <div className={styles.main}>
        <button
          type="button"
          className="mobile-menu-btn"
          onClick={() => setSidebarOpen(true)}
          aria-label="Open menu"
          style={{ margin: "12px 0 0 12px" }}
        >
          <Menu size={20} strokeWidth={1.8} />
        </button>

        <div className={styles.column}>
          <div className={styles.topRow}>
            <div className={styles.breadcrumb}>
              {breadcrumbs.map((bc, idx) => (
                <React.Fragment key={idx}>
                  {idx > 0 && <span className={styles.crumbSep}> / </span>}
                  <button
                    type="button"
                    className={styles.crumbButton}
                    onClick={() => handleNavigate(bc.id)}
                  >
                    {bc.name}
                  </button>
                </React.Fragment>
              ))}
            </div>

            {canCreateNote && (
              <button
                type="button"
                className={styles.newNoteBtn}
                onClick={triggerCreateNote}
              >
                + New note
                <span className={styles.keyChip}>⌘N</span>
              </button>
            )}
          </div>

          <h1 className={styles.title}>{pageTitle}</h1>

          <NoteList
            folderId={folderIdParam}
            searchQuery={searchQuery}
            tagFilter={tagFilter}
            onAfterMutation={() => setSidebarRefreshKey((k) => k + 1)}
            createNoteSignal={createNoteSignal}
          />
        </div>
      </div>
    </div>
  );
}

export default function DrivePage() {
  return (
    <Suspense
      fallback={
        <div
          style={{
            display: "flex",
            height: "100vh",
            alignItems: "center",
            justifyContent: "center",
          }}
        >
          Loading...
        </div>
      }
    >
      <NotesContent />
    </Suspense>
  );
}
