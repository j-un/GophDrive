/* eslint-disable react-refresh/only-export-components */
import {
  createContext,
  useContext,
  useEffect,
  useRef,
  useState,
  ReactNode,
  useCallback,
} from "react";
import { useNavigate, useLocation } from "react-router";
import { getUser, User } from "@/lib/api";
import { syncLocalNotesToSubject } from "@/lib/idb";

function isAuthFailure(error: unknown): boolean {
  return (
    error instanceof Error &&
    (error.message.includes("401") || error.message.includes("Unauthorized"))
  );
}

async function dropCachedNotes(subject: string | null) {
  try {
    await syncLocalNotesToSubject(subject);
  } catch (error) {
    console.error("AuthContext: failed to drop cached notes", error);
  }
}

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
  const navigate = useNavigate();
  const navigateRef = useRef(navigate);
  useEffect(() => {
    navigateRef.current = navigate;
  });
  const location = useLocation();
  const pathname = location.pathname;

  const fetchUser = useCallback(async () => {
    try {
      const u = await getUser();
      await dropCachedNotes(u.id);
      setUser(u);
      const redirect = sessionStorage.getItem("authRedirect");
      if (redirect) {
        sessionStorage.removeItem("authRedirect");
        navigateRef.current(redirect);
      }
    } catch (error) {
      const e = error as Error;
      if (isAuthFailure(e)) {
        await dropCachedNotes(null);
      } else {
        console.error(
          "AuthContext: Failed to fetch user. Detailed error:",
          e.message || e,
        );
      }
      setUser(null);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    let cancelled = false;
    void getUser().then(
      async (u) => {
        if (cancelled) return;
        await dropCachedNotes(u.id);
        if (cancelled) return;
        setUser(u);
        const redirect = sessionStorage.getItem("authRedirect");
        if (redirect) {
          sessionStorage.removeItem("authRedirect");
          navigateRef.current(redirect);
        }
        setLoading(false);
      },
      async (error: unknown) => {
        if (cancelled) return;
        const e = error as Error;
        if (isAuthFailure(e)) {
          await dropCachedNotes(null);
        } else {
          console.error(
            "AuthContext: Failed to fetch user. Detailed error:",
            e.message || e,
          );
        }
        if (cancelled) return;
        setUser(null);
        setLoading(false);
      },
    );
    return () => {
      cancelled = true;
    };
  }, []);

  useEffect(() => {
    if (loading) return;

    const isPublic =
      pathname === "/" ||
      pathname.startsWith("/auth") ||
      pathname.startsWith("/privacy") ||
      pathname.startsWith("/terms");

    if (isPublic) return;

    if (!user) {
      sessionStorage.setItem("authRedirect", pathname + location.search);
      navigate("/");
      return;
    }
  }, [user, loading, pathname, location.search, navigate]);

  return (
    <AuthContext.Provider value={{ user, loading, refreshUser: fetchUser }}>
      {children}
    </AuthContext.Provider>
  );
}

export const useAuth = () => useContext(AuthContext);
