import {
  createContext,
  useContext,
  useState,
  useEffect,
  useCallback,
  useMemo,
  ReactNode,
} from "react";

type Theme = "light" | "dark" | "midnight";
export type ThemePreference = Theme | "system";

interface ThemeContextType {
  // The theme actually applied; "system" resolves to light or dark.
  theme: Theme;
  preference: ThemePreference;
  toggleTheme: () => void;
  setTheme: (theme: ThemePreference) => void;
}

const ThemeContext = createContext<ThemeContextType | undefined>(undefined);

const THEME_KEY = "voxflow-theme";
const darkQuery = window.matchMedia("(prefers-color-scheme: dark)");

export function ThemeProvider({ children }: { children: ReactNode }) {
  const [preference, setPreference] = useState<ThemePreference>(() => {
    const stored = localStorage.getItem(THEME_KEY);
    if (stored === "light" || stored === "dark" || stored === "midnight") {
      return stored;
    }
    return "system";
  });
  const [systemDark, setSystemDark] = useState(darkQuery.matches);

  useEffect(() => {
    const onChange = (e: MediaQueryListEvent) => setSystemDark(e.matches);
    darkQuery.addEventListener("change", onChange);
    return () => darkQuery.removeEventListener("change", onChange);
  }, []);

  const theme: Theme =
    preference === "system" ? (systemDark ? "dark" : "light") : preference;

  useEffect(() => {
    localStorage.setItem(THEME_KEY, preference);
  }, [preference]);

  useEffect(() => {
    document.documentElement.classList.remove("light", "dark", "midnight");
    document.documentElement.classList.add(theme);
  }, [theme]);

  const toggleTheme = useCallback(() => {
    setPreference(theme === "light" ? "dark" : "light");
  }, [theme]);

  const value = useMemo(
    () => ({ theme, preference, toggleTheme, setTheme: setPreference }),
    [theme, preference, toggleTheme],
  );

  return (
    <ThemeContext.Provider value={value}>
      {children}
    </ThemeContext.Provider>
  );
}

export function useTheme() {
  const context = useContext(ThemeContext);
  if (context === undefined) {
    throw new Error("useTheme must be used within a ThemeProvider");
  }
  return context;
}
