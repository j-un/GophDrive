"use client";

import React, { useState, useEffect, useMemo } from "react";
import NoteList from "@/components/NoteList";
import { Sidebar } from "@/components/Sidebar";
import { ChevronRight, Home, Menu } from "lucide-react";
import { useRouter, useSearchParams } from "next/navigation";
import { getBreadcrumbs, isValidNoteId, BreadcrumbItem } from "@/lib/api";

function NotesContent() {
  const router = useRouter();
  const searchParams = useSearchParams();
  // folderId from URL, or undefined if not present
  const folderIdParam = searchParams.get("folderId") || undefined;

  const [currentFolderId, setCurrentFolderId] = useState<string | undefined>(
    folderIdParam,
  );
  const [breadcrumbs, setBreadcrumbs] = useState<BreadcrumbItem[]>([
    { id: "", name: "Home" },
  ]);

  const [sidebarOpen, setSidebarOpen] = useState(false);
  const [sidebarRefreshKey, setSidebarRefreshKey] = useState(0);

  // Stable reference — only changes when actual tag values change
  const tagFilter = useMemo(
    () => searchParams.getAll("tag").filter(Boolean),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [searchParams.toString()],
  );

  // Sync state with URL param
  useEffect(() => {
    setCurrentFolderId(folderIdParam);
    if (folderIdParam) {
      fetchFolderInfo(folderIdParam);
    } else {
      setBreadcrumbs([{ id: "", name: "Home" }]);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [folderIdParam]);

  const fetchFolderInfo = async (id: string) => {
    if (!isValidNoteId(id)) {
      setCurrentFolderId(undefined);
      setBreadcrumbs([{ id: "", name: "Home" }]);
      router.replace("/drive");
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

  const handleNavigate = (folderId?: string) => {
    if (folderId) {
      router.push(`/drive?folderId=${folderId}`);
    } else {
      router.push("/drive");
    }
    setCurrentFolderId(folderId);
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
        {/* Header with Breadcrumbs */}
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

        {/* Main Content */}
        <NoteList
          folderId={currentFolderId}
          searchQuery={searchParams.get("q") || undefined}
          tagFilter={tagFilter}
          onAfterMutation={() => setSidebarRefreshKey((k) => k + 1)}
        />
      </div>
    </div>
  );
}

export default function NotesPage() {
  return (
    <React.Suspense
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
    </React.Suspense>
  );
}
