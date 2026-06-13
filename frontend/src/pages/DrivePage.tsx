import React, { useState, useEffect, useMemo, useRef, Suspense } from "react";
import NoteList from "@/components/NoteList";
import { Sidebar } from "@/components/Sidebar";
import { ChevronRight, Home, Menu } from "lucide-react";
import { useNavigate, useSearchParams } from "react-router";
import { getBreadcrumbs, isValidNoteId, BreadcrumbItem } from "@/lib/api";

function NotesContent() {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const folderIdParam = searchParams.get("folderId") || undefined;

  const [breadcrumbs, setBreadcrumbs] = useState<BreadcrumbItem[]>([
    { id: "", name: "Home" },
  ]);

  const [sidebarOpen, setSidebarOpen] = useState(false);
  const [sidebarRefreshKey, setSidebarRefreshKey] = useState(0);

  const searchParamsStr = searchParams.toString();
  const tagFilter = useMemo(
    () => searchParams.getAll("tag").filter(Boolean),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [searchParamsStr],
  );

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

  return (
    <div
      style={{
        display: "flex",
        height: "100vh",
        width: "100vw",
        overflow: "hidden",
      }}
    >
      <Sidebar
        onNavigate={handleNavigate}
        isOpen={sidebarOpen}
        onClose={() => setSidebarOpen(false)}
        refreshTrigger={sidebarRefreshKey}
      />

      <div
        style={{
          flex: 1,
          display: "flex",
          flexDirection: "column",
          height: "100%",
        }}
      >
        <div
          style={{
            padding: "1rem",
            borderBottom: "1px solid var(--border)",
            display: "flex",
            alignItems: "center",
            gap: "0.5rem",
            background: "var(--header-background)",
          }}
        >
          <button
            className="mobile-menu-btn"
            onClick={() => setSidebarOpen(true)}
            aria-label="Open menu"
          >
            <Menu size={20} />
          </button>
          <div
            style={{
              display: "flex",
              alignItems: "center",
              gap: "0.5rem",
              fontWeight: "bold",
            }}
          >
            {breadcrumbs.map((bc, idx) => (
              <React.Fragment key={idx}>
                {idx > 0 && <ChevronRight size={16} style={{ opacity: 0.5 }} />}
                <button
                  onClick={() => handleNavigate(bc.id)}
                  className="hover:underline"
                  style={{
                    display: "flex",
                    alignItems: "center",
                    gap: "0.5rem",
                    background: "transparent",
                    border: "none",
                    color: "inherit",
                    cursor: "pointer",
                    opacity: idx === breadcrumbs.length - 1 ? 1 : 0.7,
                  }}
                >
                  {idx === 0 && <Home size={16} />}
                  {bc.name}
                </button>
              </React.Fragment>
            ))}
          </div>
        </div>

        <NoteList
          folderId={folderIdParam}
          searchQuery={searchParams.get("q") || undefined}
          tagFilter={tagFilter}
          onAfterMutation={() => setSidebarRefreshKey((k) => k + 1)}
        />
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
