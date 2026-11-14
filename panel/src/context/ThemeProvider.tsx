import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react';
import { THEME_STORAGE_KEY, ThemeContext, type ResolvedTheme, type ThemePreference } from './theme';

const systemQuery = () => window.matchMedia('(prefers-color-scheme: dark)');

function readPreference(): ThemePreference {
    try {
        const stored = localStorage.getItem(THEME_STORAGE_KEY);
        if (stored === 'light' || stored === 'dark' || stored === 'system') return stored;
    } catch {
        // Storage can be unavailable (private mode); fall back to the system theme.
    }
    return 'system';
}

function resolve(preference: ThemePreference, systemDark: boolean): ResolvedTheme {
    if (preference === 'system') return systemDark ? 'dark' : 'light';
    return preference;
}

export function ThemeProvider({ children }: { children: ReactNode }) {
    const [preference, setPreferenceState] = useState<ThemePreference>(readPreference);
    const [systemDark, setSystemDark] = useState(() => systemQuery().matches);

    // Follow the operating system while the preference is "system".
    useEffect(() => {
        const query = systemQuery();
        const onChange = (event: MediaQueryListEvent) => setSystemDark(event.matches);
        query.addEventListener('change', onChange);
        return () => query.removeEventListener('change', onChange);
    }, []);

    const theme = resolve(preference, systemDark);

    useEffect(() => {
        document.documentElement.setAttribute('data-theme', theme);
    }, [theme]);

    const setPreference = useCallback((next: ThemePreference) => {
        setPreferenceState(next);
        try {
            localStorage.setItem(THEME_STORAGE_KEY, next);
        } catch {
            // The choice still applies for this visit.
        }
    }, []);

    const value = useMemo(() => ({ preference, theme, setPreference }), [preference, theme, setPreference]);

    return <ThemeContext.Provider value={value}>{children}</ThemeContext.Provider>;
}
