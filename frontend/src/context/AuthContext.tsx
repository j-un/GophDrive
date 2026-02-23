"use client";
import {
  createContext,
  useContext,
  useEffect,
  useState,
  ReactNode,
  useCallback,
} from "react";
import { useRouter, usePathname } from "next/navigation";
import { getUser, User } from "@/lib/api";

interface AuthContextType {
  user: User | null;
  loading: boolean;
  refreshUser: () => Promise<void>;
}

const AuthContext = createContext<AuthContextType>({
  user: null,
  loading: true,
  refreshUser: async () => {},
});

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);
  const router = useRouter();
  const pathname = usePathname();

  const fetchUser = useCallback(async () => {
    console.log("AuthContext: fetchUser started");

    // Extract token from URL if present (e.g. after demo-login)
    if (typeof window !== "undefined") {
      const urlParams = new URLSearchParams(window.location.search);
      const token = urlParams.get("token");
      if (token) {
        // We must import setToken from api or use localStorage directly.
        // Since this runs before page.tsx's effect, it ensures token is set.
        localStorage.setItem("session_token", token);
        // Also remove it from URL so it doesn't linger
        window.history.replaceState(
          {},
          document.title,
          window.location.pathname,
        );
      }
    }

    try {
      const u = await getUser();
      console.log("AuthContext: getUser success:", u);
      setUser(u);
    } catch (error) {
      const e = error as Error;
      if (e.message && e.message.includes("401")) {
        // Not an error, just means user is not logged in
        console.log("AuthContext: User is not logged in (401).");
      } else {
        console.error(
          "AuthContext: Failed to fetch user. Detailed error:",
          e.message || e,
        );
      }
      setUser(null);
    } finally {
      console.log("AuthContext: setLoading(false)");
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchUser();
  }, [fetchUser]);

  useEffect(() => {
    // Redirect logic
    // console.log("AuthContext check:", { loading, user, pathname });
    if (loading) return;

    const isPublic = pathname === "/" || pathname.startsWith("/auth");

    // If we are on public page, just let it be.
    if (isPublic) return;

    if (!user) {
      console.warn(
        "AuthContext: Redirecting to / because user is null on protected route:",
        pathname,
      );
      router.push("/");
      return;
    }

    if (!user.base_folder_id && pathname !== "/setup") {
      console.warn(
        "AuthContext: Redirecting to /setup because base_folder_id is missing",
      );
      router.push("/setup");
    }
  }, [user, loading, pathname, router]);

  return (
    <AuthContext.Provider value={{ user, loading, refreshUser: fetchUser }}>
      {children}
    </AuthContext.Provider>
  );
}

export const useAuth = () => useContext(AuthContext);
