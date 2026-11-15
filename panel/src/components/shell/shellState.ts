import { createContext, useContext } from 'react';

/**
 * Open/closed state of the shell's menus. It lives above the theme-specific
 * navigation so that switching theme from inside a menu keeps it open in the
 * new layout instead of closing it and dropping keyboard focus.
 */
export interface ShellState {
    drawerOpen: boolean;
    setDrawerOpen: (open: boolean) => void;
    accountOpen: boolean;
    setAccountOpen: (open: boolean) => void;
}

export const ShellContext = createContext<ShellState | null>(null);

export function useShell() {
    const context = useContext(ShellContext);
    if (!context) {
        throw new Error('useShell must be used within the signed-in layout');
    }
    return context;
}
