import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react';
import {
    LAYOUT_STORAGE_KEY,
    THEME_STORAGE_KEY,
    ThemeContext,
    isLayoutTheme,
    type LayoutTheme,
    type ResolvedTheme,
    type ThemePreference,
} from './theme';

const systemQuery = () => window.matchMedia('(prefers-color-scheme: dark)');

function readStored(key: string): string | null {
    try {
        return localStorage.getItem(key);
    } catch {
        // Storage can be unavailable (private mode); fall back to the defaults.
        return null;
    }
}

function store(key: string, value: string) {
    try {
        localStorage.setItem(key, value);
    } catch {
        // The choice still applies for this visit.
    }
}

function readPreference(): ThemePreference {
    const stored = readStored(THEME_STORAGE_KEY);
    if (stored === 'light' || stored === 'dark' || stored === 'system') return stored;
    return 'system';
}

function readLayout(): LayoutTheme {
    const stored = readStored(LAYOUT_STORAGE_KEY);
    return isLayoutTheme(stored) ? stored : 'classic';
}

function resolve(preference: ThemePreference, systemDark: boolean): ResolvedTheme {
    if (preference === 'system') return systemDark ? 'dark' : 'light';
    return preference;
}

export function ThemeProvider({ children }: { children: ReactNode }) {
    const [preference, setPreferenceState] = useState<ThemePreference>(readPreference);
    const [layout, setLayoutState] = useState<LayoutTheme>(readLayout);
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

    useEffect(() => {
        document.documentElement.setAttribute('data-layout', layout);
    }, [layout]);

    const setPreference = useCallback((next: ThemePreference) => {
        setPreferenceState(next);
        store(THEME_STORAGE_KEY, next);
    }, []);

    const setLayout = useCallback((next: LayoutTheme) => {
        setLayoutState(next);
        store(LAYOUT_STORAGE_KEY, next);
    }, []);

    const value = useMemo(
        () => ({ preference, theme, setPreference, layout, setLayout }),
        [preference, theme, setPreference, layout, setLayout],
    );

    return <ThemeContext.Provider value={value}>{children}</ThemeContext.Provider>;
}
