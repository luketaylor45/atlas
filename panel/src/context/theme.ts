import { createContext, useContext } from 'react';

/** Light or dark colours. "system" follows the device. */
export type ThemePreference = 'system' | 'light' | 'dark';
export type ResolvedTheme = 'light' | 'dark';

/**
 * The theme: how the panel is laid out and styled (navigation, density,
 * surfaces, shape). Independent of the light/dark mode.
 */
export type LayoutTheme = 'classic' | 'horizon' | 'compact' | 'glass';

export const LAYOUT_THEMES: readonly LayoutTheme[] = ['classic', 'horizon', 'compact', 'glass'];

/** Storage keys; also read by the inline script in index.html. */
export const THEME_STORAGE_KEY = 'atlas-theme';
export const LAYOUT_STORAGE_KEY = 'atlas-layout';

export function isLayoutTheme(value: unknown): value is LayoutTheme {
    return typeof value === 'string' && (LAYOUT_THEMES as readonly string[]).includes(value);
}

export interface ThemeContextValue {
    /** What the user chose. */
    preference: ThemePreference;
    /** The mode actually shown, after resolving "system". */
    theme: ResolvedTheme;
    setPreference: (preference: ThemePreference) => void;
    /** The chosen theme (layout and style). */
    layout: LayoutTheme;
    setLayout: (layout: LayoutTheme) => void;
}

export const ThemeContext = createContext<ThemeContextValue | undefined>(undefined);

export function useTheme() {
    const context = useContext(ThemeContext);
    if (!context) {
        throw new Error('useTheme must be used within a ThemeProvider');
    }
    return context;
}
