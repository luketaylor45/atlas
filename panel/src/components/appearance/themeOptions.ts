import type { LayoutTheme } from '../../context/theme';

export interface ThemeOption {
    id: LayoutTheme;
    name: string;
    description: string;
}

/** The themes people can pick, in the order they are offered. */
export const themeOptions: ThemeOption[] = [
    {
        id: 'classic',
        name: 'Classic',
        description: 'A labelled sidebar on the left. Calm and familiar.',
    },
    {
        id: 'horizon',
        name: 'Horizon',
        description: 'Navigation across the top, wide pages and softer, rounder cards.',
    },
    {
        id: 'compact',
        name: 'Compact',
        description: 'A slim icon rail and denser pages, for keeping an eye on many servers.',
    },
    {
        id: 'glass',
        name: 'Glass',
        description: 'Frosted, see-through panels floating over a soft colour wash.',
    },
];
