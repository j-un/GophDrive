"use client";

import React, { useState, useEffect } from "react";
import NoteList from "@/components/NoteList";
import { Sidebar } from "@/components/Sidebar";
import { ChevronRight, Home, Menu } from "lucide-react";
import { useRouter, useSearchParams } from "next/navigation";
import { getBreadcrumbs, BreadcrumbItem } from "@/lib/api";

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

  // Sync state with URL param
  useEffect(() => {
    setCurrentFolderId(folderIdParam);

    // If we have a folderId but breadcrumbs are just Home, assume we need to fetch info
    if (folderIdParam && breadcrumbs.length === 1) {
      fetchFolderInfo(folderIdParam);
    } else if (!folderIdParam) {
      setBreadcrumbs([{ id: "", name: "Home" }]);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [folderIdParam]);

  const fetchFolderInfo = async (id: string) => {
    try {
      const crumbs = await getBreadcrumbs(id);
      setBreadcrumbs(crumbs);
    } catch (e) {
      console.error("Failed to fetch folder info", e);
    }
  };

  const handleNavigate = (folderId?: string, folderName?: string) => {
    // Update URL
    if (folderId) {
      router.push(`/notes?folderId=${folderId}`);
    } else {
      router.push("/notes");
    }

    // Update local state immediately for responsiveness
    setCurrentFolderId(folderId);

    if (folderId === undefined) {
      setBreadcrumbs([{ id: "", name: "Home" }]);
      return;
    }

    const existingIndex = breadcrumbs.findIndex((b) => b.id === folderId);
    if (existingIndex !== -1) {
      setBreadcrumbs((prev) => prev.slice(0, existingIndex + 1));
    } else {
      setBreadcrumbs((prev) => [
        ...prev,
        { id: folderId, name: folderName || "Folder" },
      ]);
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
        currentFolderId={currentFolderId}
        onNavigate={handleNavigate}
        breadcrumbs={breadcrumbs}
        isOpen={sidebarOpen}
        onClose={() => setSidebarOpen(false)}
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
                  onClick={() => handleNavigate(bc.id, bc.name)}
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
