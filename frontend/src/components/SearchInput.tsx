"use client";

import { useState, useEffect } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { Search } from "lucide-react";

export default function SearchInput() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const [query, setQuery] = useState("");

  // Sync with URL q param so the input reflects current search
  useEffect(() => {
    const q = searchParams.get("q");
    if (q) {
      // eslint-disable-next-line react-hooks/set-state-in-effect
      setQuery(q);
    } else {
      setQuery("");
    }
  }, [searchParams]);

  const handleSearch = () => {
    if (query.trim()) {
      router.push(`/notes?q=${encodeURIComponent(query)}`);
    } else {
      // If empty, maybe go back to notes list/home?
      // router.push('/notes');
    }
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "Enter") {
      handleSearch();
    }
  };

  return (
    <div
      className="flex items-center px-2 py-2 transition-colors"
      style={{
        display: "flex",
        alignItems: "center",
        width: "100%",
        height: "40px",
        gap: "0.5rem",
      }}
    >
      <Search
        size={18}
        className="text-[var(--muted-foreground)] flex-shrink-0"
        strokeWidth={2.5}
      />
      <input
        type="text"
        value={query}
        onChange={(e) => setQuery(e.target.value)}
        onKeyDown={handleKeyDown}
        placeholder="Search..."
        className="flex-1 bg-transparent text-base text-[var(--foreground)] placeholder-[var(--muted-foreground)] min-w-0"
        style={{
          border: "none",
          borderBottom: "1px solid var(--border)",
          outline: "none",
          background: "transparent",
          height: "24px", // Reduced height to move underline up
          alignSelf: "center", // Center vertically
          padding: 0, // Reset padding
        }}
      />
    </div>
  );
}
