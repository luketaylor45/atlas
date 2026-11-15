import { useEffect, useId, useRef } from 'react';
import { Link } from 'react-router-dom';
import { ChevronsUpDown, LogOut, Settings } from 'lucide-react';
import { useAuth } from '../../context/auth';
import { cn } from '../../lib/cn';
import { ThemeQuickSwitch } from '../appearance/ThemePicker';
import ThemeToggle from '../ThemeToggle';
import { Avatar } from './parts';
import { useShell } from './shellState';

const placements = {
    /** Above the button, left-aligned: the bottom of a sidebar. */
    up: 'bottom-full left-0 mb-2 w-72 origin-bottom-left',
    /** Below the button, right-aligned: a top bar. */
    down: 'right-0 top-full mt-2 w-72 origin-top-right',
    /** To the right of the button, bottom-aligned: an icon rail. */
    right: 'bottom-0 left-full ml-3 w-72 origin-bottom-left',
};

interface UserMenuProps {
    placement: keyof typeof placements;
    /** "full" shows the name and role next to the avatar; "avatar" only the avatar. */
    trigger?: 'full' | 'avatar';
    className?: string;
}

const itemClass =
    'flex w-full items-center gap-2.5 rounded-control px-2.5 py-2 text-sm text-foreground transition-colors hover:bg-secondary';

/**
 * The signed-in person's menu: who they are, the light/dark mode, a quick
 * theme switch, settings and sign out. Closes on Escape, on a click outside
 * and when focus leaves it.
 */
export default function UserMenu({ placement, trigger = 'full', className }: UserMenuProps) {
    const { user, logout } = useAuth();
    const { accountOpen: open, setAccountOpen: setOpen } = useShell();
    const wrapper = useRef<HTMLDivElement>(null);
    const button = useRef<HTMLButtonElement>(null);
    const panel = useRef<HTMLDivElement>(null);
    // Mounting already open means the theme was just switched from this menu.
    const reopened = useRef(open);
    const panelId = useId();
    const role = user?.is_admin ? 'Administrator' : 'Member';

    useEffect(() => {
        if (!open) {
            reopened.current = false;
            return;
        }

        // Put keyboard focus on the current choice: the theme after a theme
        // switch, otherwise the mode.
        const selector = reopened.current
            ? '[role="radiogroup"][aria-label="Theme"] [aria-checked="true"]'
            : '[role="radio"][aria-checked="true"]';
        panel.current?.querySelector<HTMLElement>(selector)?.focus();

        const onKeyDown = (event: KeyboardEvent) => {
            if (event.key !== 'Escape') return;
            event.stopPropagation();
            setOpen(false);
            button.current?.focus();
        };
        const onPointerDown = (event: PointerEvent) => {
            if (!wrapper.current?.contains(event.target as Node)) setOpen(false);
        };
        document.addEventListener('keydown', onKeyDown);
        document.addEventListener('pointerdown', onPointerDown);
        return () => {
            document.removeEventListener('keydown', onKeyDown);
            document.removeEventListener('pointerdown', onPointerDown);
        };
    }, [open, setOpen]);

    return (
        <div
            ref={wrapper}
            className={cn('relative', className)}
            onBlur={(event) => {
                const next = event.relatedTarget as Node | null;
                // A null target is a theme switch re-rendering the layout, or
                // a click on empty space; the pointer handler covers the latter.
                if (open && next && !event.currentTarget.contains(next)) setOpen(false);
            }}
        >
            <button
                ref={button}
                type="button"
                aria-expanded={open}
                aria-controls={open ? panelId : undefined}
                aria-label={trigger === 'avatar' ? `Account: ${user?.username ?? ''}` : undefined}
                title={trigger === 'avatar' ? 'Account and appearance' : undefined}
                onClick={() => setOpen(!open)}
                className={cn(
                    'flex items-center text-left transition-[background-color,box-shadow]',
                    trigger === 'full'
                        ? 'w-full gap-3 rounded-control px-2 py-1.5 hover:bg-secondary'
                        : 'rounded-full p-0.5 hover:ring-4 hover:ring-primary/15',
                    open && (trigger === 'full' ? 'bg-secondary' : 'ring-4 ring-primary/20'),
                )}
            >
                <Avatar name={user?.username} />
                {trigger === 'full' && (
                    <>
                        <span className="min-w-0 flex-1">
                            <span className="block truncate text-sm font-medium text-foreground" title={user?.username}>
                                {user?.username}
                            </span>
                            <span className="block text-xs text-muted">{role}</span>
                        </span>
                        <ChevronsUpDown size={15} className="shrink-0 text-subtle" aria-hidden />
                    </>
                )}
            </button>

            {open && (
                <div
                    ref={panel}
                    id={panelId}
                    role="dialog"
                    aria-label="Account and appearance"
                    tabIndex={-1}
                    className={cn(
                        'surface absolute z-50 animate-menu-in rounded-card border border-border bg-popover p-1.5 shadow-pop outline-none',
                        placements[placement],
                    )}
                >
                    <div className="flex items-center gap-3 px-2.5 py-2">
                        <Avatar name={user?.username} size="lg" />
                        <div className="min-w-0">
                            <div className="truncate text-sm font-semibold text-foreground">{user?.username}</div>
                            <div className="text-xs text-muted">{role}</div>
                        </div>
                    </div>

                    <div className="my-1 h-px bg-border" />

                    <div className="space-y-3 px-2.5 py-2">
                        <div>
                            <div className="mb-1.5 text-xs font-medium text-muted">Mode</div>
                            <ThemeToggle showLabels className="flex w-full" />
                        </div>
                        <div>
                            <div className="mb-1.5 text-xs font-medium text-muted">Theme</div>
                            <ThemeQuickSwitch />
                        </div>
                    </div>

                    <div className="my-1 h-px bg-border" />

                    <Link to="/settings" className={itemClass} onClick={() => setOpen(false)}>
                        <Settings size={16} className="text-muted" aria-hidden />
                        Settings
                    </Link>
                    <button type="button" className={itemClass} onClick={logout}>
                        <LogOut size={16} className="text-muted" aria-hidden />
                        Sign out
                    </button>
                </div>
            )}
        </div>
    );
}
