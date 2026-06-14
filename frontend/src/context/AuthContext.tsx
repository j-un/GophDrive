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
      setUser(u);
      // Restore deep link saved before auth redirect (see redirect effect below).
      const redirect = sessionStorage.getItem("authRedirect");
      if (redirect) {
        sessionStorage.removeItem("authRedirect");
        navigateRef.current(redirect);
      }
    } catch (error) {
      const e = error as Error;
      if (e.message && e.message.includes("401")) {
        // Not an error, just means user is not logged in
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
    // eslint-disable-next-line react-hooks/set-state-in-effect
    fetchUser();
  }, [fetchUser]);

  useEffect(() => {
    // Redirect logic
    // console.log("AuthContext check:", { loading, user, pathname });
    if (loading) return;

    const isPublic =
      pathname === "/" ||
      pathname.startsWith("/auth") ||
      pathname.startsWith("/privacy") ||
      pathname.startsWith("/terms");

    // If we are on public page, just let it be.
    if (isPublic) return;

    if (!user) {
      // Save the intended destination so fetchUser can restore it after login.
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
