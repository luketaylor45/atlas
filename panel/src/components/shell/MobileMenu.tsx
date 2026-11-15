import { useEffect, useRef, type MouseEvent } from 'react';
import { Link } from 'react-router-dom';
import { LogOut, Menu, X } from 'lucide-react';
import { useAuth } from '../../context/auth';
import { cn } from '../../lib/cn';
import { ThemeQuickSwitch } from '../appearance/ThemePicker';
import Logo from '../Logo';
import ThemeToggle from '../ThemeToggle';
import { IconButton } from '../ui';
import { adminNav, mainNav } from './nav';
import { Avatar, SideNavSection, type SideNavVariant } from './parts';
import { useShell } from './shellState';

const FOCUSABLE = 'a[href], button:not([disabled]), [tabindex]:not([tabindex="-1"])';

const panels = {
    /** Slides in from the left edge. */
    left: 'inset-y-0 left-0 w-72 max-w-[85vw] animate-drawer-in border-r',
    /** Floats in from the left, inset from the edges. */
    floating: 'inset-y-2 left-2 w-72 max-w-[calc(100vw-1rem)] animate-drawer-in rounded-card border',
    /** Drops down from the top. */
    top: 'inset-x-0 top-0 max-h-[90dvh] animate-sheet-in rounded-b-card border-b',
};

interface MobileMenuProps {
    side?: keyof typeof panels;
    variant?: SideNavVariant;
}

/**
 * The navigation on small screens, opened from the menu button. A modal
 * dialog: focus stays inside, Escape or the backdrop closes it, and focus
 * returns to the menu button afterwards.
 */
export function MobileMenu({ side = 'left', variant = 'classic' }: MobileMenuProps) {
    const { user, logout } = useAuth();
    const { drawerOpen: open, setDrawerOpen } = useShell();
    const panel = useRef<HTMLDivElement>(null);

    useEffect(() => {
        if (!open) return;
        const node = panel.current;
        const opener = document.querySelector<HTMLElement>('[data-menu-button]');
        node?.querySelector<HTMLElement>('[data-close]')?.focus();

        const previousOverflow = document.body.style.overflow;
        document.body.style.overflow = 'hidden';

        const onKeyDown = (event: KeyboardEvent) => {
            if (event.key === 'Escape') {
                setDrawerOpen(false);
                return;
            }
            if (event.key !== 'Tab' || !node) return;
            const focusable = Array.from(node.querySelectorAll<HTMLElement>(FOCUSABLE)).filter((el) => el.offsetParent !== null);
            if (focusable.length === 0) return;
            const first = focusable[0];
            const last = focusable[focusable.length - 1];
            if (event.shiftKey && document.activeElement === first) {
                event.preventDefault();
                last.focus();
            } else if (!event.shiftKey && document.activeElement === last) {
                event.preventDefault();
                first.focus();
            }
        };
        document.addEventListener('keydown', onKeyDown);
        return () => {
            document.removeEventListener('keydown', onKeyDown);
            document.body.style.overflow = previousOverflow;
            // Back to the menu button, if it is still on the page.
            if (opener?.isConnected) opener.focus();
        };
    }, [open, setDrawerOpen]);

    if (!open) return null;

    const close = () => setDrawerOpen(false);
    // Following a link inside the menu closes it.
    const closeOnLink = (event: MouseEvent) => {
        if ((event.target as HTMLElement).closest('a')) close();
    };

    return (
        <div className="fixed inset-0 z-40 lg:hidden">
            <div className="absolute inset-0 animate-fade-in bg-black/50 backdrop-blur-[2px]" onClick={close} aria-hidden />
            <div
                ref={panel}
                role="dialog"
                aria-modal="true"
                aria-label="Menu"
                className={cn(
                    'surface absolute flex flex-col overflow-hidden border-border bg-popover shadow-pop',
                    panels[side],
                )}
            >
                <div className="flex h-14 shrink-0 items-center justify-between pl-5 pr-3">
                    <Link to="/" onClick={close} aria-label="Atlas home">
                        <Logo size="sm" />
                    </Link>
                    <IconButton label="Close menu" onClick={close} data-close>
                        <X size={18} aria-hidden />
                    </IconButton>
                </div>

                <nav
                    aria-label="Main"
                    className={cn(
                        'min-h-0 flex-1 overflow-y-auto px-3 py-2',
                        side === 'top' ? 'grid grid-cols-1 gap-6 min-[480px]:grid-cols-2' : 'space-y-6',
                    )}
                    onClick={closeOnLink}
                >
                    <SideNavSection items={mainNav} variant={variant} />
                    {user?.is_admin && <SideNavSection title="Administration" items={adminNav} variant={variant} />}
                </nav>

                <div className="shrink-0 space-y-3 border-t border-border p-3">
                    <div className="flex items-center justify-between gap-3 px-1">
                        <span className="text-xs font-medium text-muted">Mode</span>
                        <ThemeToggle />
                    </div>
                    <div className="px-1">
                        <div className="mb-1.5 text-xs font-medium text-muted">Theme</div>
                        <ThemeQuickSwitch />
                    </div>
                    <div className="flex items-center gap-3 rounded-control px-1 pt-1">
                        <Avatar name={user?.username} />
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
        </div>
    );
}

/** Opens the mobile menu. Hidden on large screens. */
export function MenuButton({ className }: { className?: string }) {
    const { drawerOpen, setDrawerOpen } = useShell();
    return (
        <IconButton
            label="Open menu"
            className={cn('lg:hidden', className)}
            onClick={() => setDrawerOpen(true)}
            aria-expanded={drawerOpen}
            aria-haspopup="dialog"
            data-menu-button
        >
            <Menu size={20} aria-hidden />
        </IconButton>
    );
}

/** The top bar on small screens for the sidebar-based themes. */
export function MobileBar({ className }: { className?: string }) {
    return (
        <header
            className={cn(
                'sticky top-0 z-30 flex h-14 items-center justify-between border-b border-border bg-card/85 px-4 backdrop-blur-lg lg:hidden',
                className,
            )}
        >
            <Link to="/" aria-label="Atlas home">
                <Logo size="sm" />
            </Link>
            <MenuButton />
        </header>
    );
}
