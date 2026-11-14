import { useEffect, useState } from 'react';
import { Link, NavLink, Outlet, useLocation } from 'react-router-dom';
import {
    Bell,
    Gauge,
    LayoutDashboard,
    LogOut,
    Menu,
    Package,
    Server,
    ServerCog,
    Settings,
    Users,
    Waypoints,
    X,
    type LucideIcon,
} from 'lucide-react';
import { useAuth } from '../context/auth';
import { cn } from '../lib/cn';
import ErrorBoundary from './ErrorBoundary';
import Logo from './Logo';
import ThemeToggle from './ThemeToggle';
import { IconButton } from './ui';

interface NavItem {
    to: string;
    label: string;
    icon: LucideIcon;
    /** Match only the exact path (for index routes). */
    end?: boolean;
}

const mainNav: NavItem[] = [
    { to: '/', label: 'Dashboard', icon: LayoutDashboard, end: true },
    { to: '/services', label: 'Servers', icon: Server },
    { to: '/settings', label: 'Settings', icon: Settings },
];

const adminNav: NavItem[] = [
    { to: '/admin', label: 'Overview', icon: Gauge, end: true },
    { to: '/admin/nodes', label: 'Nodes', icon: Waypoints },
    { to: '/admin/services', label: 'All servers', icon: ServerCog },
    { to: '/admin/eggs', label: 'Eggs', icon: Package },
    { to: '/admin/users', label: 'Users', icon: Users },
    { to: '/admin/news', label: 'Announcements', icon: Bell },
];

function NavSection({ title, items }: { title?: string; items: NavItem[] }) {
    return (
        <div>
            {title && <div className="mb-1.5 px-3 text-xs font-medium text-subtle">{title}</div>}
            <ul className="space-y-0.5">
                {items.map(({ to, label, icon: Icon, end }) => (
                    <li key={to}>
                        <NavLink
                            to={to}
                            end={end}
                            className={({ isActive }) =>
                                cn(
                                    'flex items-center gap-3 rounded-control px-3 py-2 text-sm transition-colors',
                                    isActive
                                        ? 'bg-primary/10 font-medium text-primary-text'
                                        : 'text-muted hover:bg-secondary hover:text-foreground',
                                )
                            }
                        >
                            <Icon size={17} aria-hidden />
                            {label}
                        </NavLink>
                    </li>
                ))}
            </ul>
        </div>
    );
}

function Sidebar({ onNavigate }: { onNavigate?: () => void }) {
    const { user, logout } = useAuth();

    return (
        <div className="flex h-full flex-col">
            <div className="flex h-14 shrink-0 items-center px-5">
                <Link to="/" onClick={onNavigate} aria-label="Atlas home">
                    <Logo />
                </Link>
            </div>

            <nav aria-label="Main" className="flex-1 space-y-6 overflow-y-auto px-3 py-3" onClick={onNavigate}>
                <NavSection items={mainNav} />
                {user?.is_admin && <NavSection title="Administration" items={adminNav} />}
            </nav>

            <div className="shrink-0 space-y-3 border-t border-border p-3">
                <div className="flex items-center justify-between gap-2 px-1">
                    <span className="text-xs text-muted">Theme</span>
                    <ThemeToggle />
                </div>
                <div className="flex items-center gap-3 rounded-control px-2 py-1.5">
                    <div
                        className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-primary/15 text-xs font-semibold uppercase text-primary-text"
                        aria-hidden
                    >
                        {user?.username?.charAt(0) || '?'}
                    </div>
                    <div className="min-w-0 flex-1">
                        <div className="truncate text-sm font-medium text-foreground" title={user?.username}>
                            {user?.username}
                        </div>
                        <div className="text-xs text-muted">{user?.is_admin ? 'Administrator' : 'Member'}</div>
                    </div>
                    <IconButton label="Sign out" size="sm" onClick={logout}>
                        <LogOut size={16} aria-hidden />
                    </IconButton>
                </div>
            </div>
        </div>
    );
}

/** The signed-in application shell: sidebar on desktop, drawer on small screens. */
export default function Layout() {
    const [drawerOpen, setDrawerOpen] = useState(false);
    const location = useLocation();

    // The drawer closes itself when a link inside it is followed.

    useEffect(() => {
        if (!drawerOpen) return;
        const onKeyDown = (event: KeyboardEvent) => {
            if (event.key === 'Escape') setDrawerOpen(false);
        };
        document.addEventListener('keydown', onKeyDown);
        return () => document.removeEventListener('keydown', onKeyDown);
    }, [drawerOpen]);

    return (
        <div className="min-h-dvh bg-background text-foreground">
            <a
                href="#main"
                className="sr-only focus:not-sr-only focus:fixed focus:left-4 focus:top-4 focus:z-[70] focus:rounded-control focus:bg-primary focus:px-4 focus:py-2 focus:text-sm focus:text-primary-foreground"
            >
                Skip to content
            </a>

            {/* Desktop sidebar */}
            <aside className="fixed inset-y-0 left-0 z-30 hidden w-60 border-r border-border bg-card lg:block">
                <Sidebar />
            </aside>

            {/* Mobile top bar */}
            <header className="sticky top-0 z-30 flex h-14 items-center justify-between border-b border-border bg-card/90 px-4 backdrop-blur lg:hidden">
                <Link to="/" aria-label="Atlas home">
                    <Logo size="sm" />
                </Link>
                <IconButton label="Open menu" onClick={() => setDrawerOpen(true)} aria-expanded={drawerOpen}>
                    <Menu size={20} aria-hidden />
                </IconButton>
            </header>

            {/* Mobile drawer */}
            {drawerOpen && (
                <div className="fixed inset-0 z-40 lg:hidden">
                    <div className="absolute inset-0 animate-fade-in bg-black/50" onClick={() => setDrawerOpen(false)} aria-hidden />
                    <aside
                        role="dialog"
                        aria-modal="true"
                        aria-label="Menu"
                        className="absolute inset-y-0 left-0 w-72 max-w-[85vw] border-r border-border bg-card shadow-pop"
                    >
                        <IconButton label="Close menu" className="absolute right-3 top-2.5" onClick={() => setDrawerOpen(false)}>
                            <X size={18} aria-hidden />
                        </IconButton>
                        <Sidebar onNavigate={() => setDrawerOpen(false)} />
                    </aside>
                </div>
            )}

            <main id="main" className="lg:pl-60">
                <div className="mx-auto w-full max-w-7xl px-4 py-6 sm:px-6 sm:py-8 lg:px-8">
                    {/* Keyed by path so an error on one page clears when you leave it. */}
                    <ErrorBoundary key={location.pathname}>
                        <Outlet />
                    </ErrorBoundary>
                </div>
            </main>
        </div>
    );
}
