/* eslint-disable react-refresh/only-export-components */
import React, {
  createContext,
  useContext,
  useEffect,
  useState,
  ReactNode,
} from "react";

type Theme = "light" | "dark" | "system";

interface ThemeContextType {
  theme: Theme;
  resolvedTheme: "light" | "dark";
  setTheme: (theme: Theme) => void;
}

const ThemeContext = createContext<ThemeContextType>({
  theme: "system",
  resolvedTheme: "light",
  setTheme: () => {},
});

function getResolved(theme: Theme): "light" | "dark" {
  if (theme !== "system") return theme;
  return matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
}

export function ThemeProvider({ children }: { children: ReactNode }) {
  const [theme, setThemeState] = useState<Theme>(() => {
    try {
      return (localStorage.getItem("theme") as Theme) || "system";
    } catch {
      return "system";
    }
  });
  const [resolvedTheme, setResolvedTheme] = useState<"light" | "dark">(() =>
    getResolved(theme),
  );

  useEffect(() => {
    const mq = matchMedia("(prefers-color-scheme: dark)");
    const update = () => {
      const resolved = getResolved(theme);
      setResolvedTheme(resolved);
      document.documentElement.setAttribute("data-theme", resolved);
    };
    update();
    mq.addEventListener("change", update);
    return () => mq.removeEventListener("change", update);
  }, [theme]);

  const setTheme = (next: Theme) => {
    try {
      localStorage.setItem("theme", next);
    } catch {
      // localStorage not available (e.g. private browsing)
    }
    setThemeState(next);
    const resolved = getResolved(next);
    setResolvedTheme(resolved);
    document.documentElement.classList.add("no-transition");
    document.documentElement.setAttribute("data-theme", resolved);
    requestAnimationFrame(() =>
      document.documentElement.classList.remove("no-transition"),
    );
  };

  return (
    <ThemeContext.Provider value={{ theme, resolvedTheme, setTheme }}>
      {children}
    </ThemeContext.Provider>
  );
}

export const useTheme = () => useContext(ThemeContext);
