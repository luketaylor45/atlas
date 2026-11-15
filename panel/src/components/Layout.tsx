import { useMemo, useState, type ComponentType } from 'react';
import { Outlet, useLocation } from 'react-router-dom';
import { useTheme, type LayoutTheme } from '../context/theme';
import { cn } from '../lib/cn';
import ErrorBoundary from './ErrorBoundary';
import ClassicNav from './shell/ClassicNav';
import CompactNav from './shell/CompactNav';
import GlassNav from './shell/GlassNav';
import HorizonNav from './shell/HorizonNav';
import { SkipLink } from './shell/parts';
import { ShellContext } from './shell/shellState';

interface Shell {
    /** The theme's navigation: sidebar, top bar or rail, plus its mobile menu. */
    nav: ComponentType;
    /** The page background behind everything. */
    root: string;
    /** Room left for the navigation beside or above the page. */
    main: string;
    /** The width and padding of the page itself. */
    content: string;
}

const shells: Record<LayoutTheme, Shell> = {
    classic: {
        nav: ClassicNav,
        root: 'bg-background',
        main: 'lg:pl-64',
        content: 'px-4 py-6 sm:px-6 sm:py-8 lg:px-10',
    },
    horizon: {
        nav: HorizonNav,
        root: 'bg-background',
        main: '',
        content: 'px-4 py-8 sm:px-6 sm:py-10 lg:px-8',
    },
    compact: {
        nav: CompactNav,
        root: 'bg-background',
        main: 'lg:pl-16',
        content: 'px-3 py-4 sm:px-5 sm:py-5 lg:px-6',
    },
    glass: {
        nav: GlassNav,
        // The mesh backdrop is painted behind the page by index.css.
        root: '',
        main: 'lg:pl-[16.5rem]',
        content: 'px-4 py-6 sm:px-6 sm:py-8 lg:px-8',
    },
};

/**
 * The signed-in application shell. The theme picks the navigation and the
 * page frame; the page itself always sits at the same place in the tree, so
 * switching theme never reloads it or loses what was typed.
 */
export default function Layout() {
    const { layout } = useTheme();
    const location = useLocation();
    const [drawerOpen, setDrawerOpen] = useState(false);
    const [accountOpen, setAccountOpen] = useState(false);
    const shellState = useMemo(() => ({ drawerOpen, setDrawerOpen, accountOpen, setAccountOpen }), [drawerOpen, accountOpen]);

    const shell = shells[layout];
    const Nav = shell.nav;

    return (
        <ShellContext.Provider value={shellState}>
            <div className={cn('min-h-dvh text-foreground', shell.root)}>
                <SkipLink />
                <Nav />

                <main id="main" tabIndex={-1} className={cn('relative isolate outline-none', shell.main)}>
                    {layout === 'horizon' && (
                        <div aria-hidden className="pointer-events-none absolute inset-x-0 top-0 -z-10 h-80 overflow-hidden">
                            <div className="horizon-glow absolute inset-0" />
                            <div className="horizon-grid absolute inset-0" />
                        </div>
                    )}
                    <div className={cn('mx-auto w-full max-w-[var(--shell-max)]', shell.content)}>
                        {/* Keyed by path so an error on one page clears when you leave it. */}
                        <ErrorBoundary key={location.pathname}>
                            <Outlet />
                        </ErrorBoundary>
                    </div>
                </main>
            </div>
        </ShellContext.Provider>
    );
}
