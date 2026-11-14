import { createContext, useContext } from 'react';

export type ThemePreference = 'system' | 'light' | 'dark';
export type ResolvedTheme = 'light' | 'dark';

/** Storage key; also read by the inline script in index.html. */
export const THEME_STORAGE_KEY = 'atlas-theme';

export interface ThemeContextValue {
    /** What the user chose. */
    preference: ThemePreference;
    /** The theme actually shown, after resolving "system". */
    theme: ResolvedTheme;
    setPreference: (preference: ThemePreference) => void;
}

export const ThemeContext = createContext<ThemeContextValue | undefined>(undefined);

export function useTheme() {
    const context = useContext(ThemeContext);
    if (!context) {
        throw new Error('useTheme must be used within a ThemeProvider');
    }
    return context;
}
