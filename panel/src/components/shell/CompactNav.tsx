import { useEffect, useState, type FocusEvent, type MouseEvent } from 'react';
import { Link, NavLink } from 'react-router-dom';
import { useAuth } from '../../context/auth';
import { cn } from '../../lib/cn';
import { LogoMark } from '../Logo';
import { MobileBar, MobileMenu } from './MobileMenu';
import { adminNav, mainNav, type NavItem } from './nav';
import UserMenu from './UserMenu';

interface Tip {
    label: string;
    top: number;
}

/**
 * Compact: a slim rail of icons. Each link is named for screen readers; a
 * label appears beside it on hover or keyboard focus and Escape hides it.
 */
export default function CompactNav() {
    const { user } = useAuth();
    const [tip, setTip] = useState<Tip | null>(null);

    useEffect(() => {
        if (!tip) return;
        const onKeyDown = (event: KeyboardEvent) => {
            if (event.key === 'Escape') setTip(null);
        };
        document.addEventListener('keydown', onKeyDown);
        return () => document.removeEventListener('keydown', onKeyDown);
    }, [tip]);

    const show = (label: string) => (event: MouseEvent<HTMLElement> | FocusEvent<HTMLElement>) => {
        const rect = event.currentTarget.getBoundingClientRect();
        setTip({ label, top: rect.top + rect.height / 2 });
    };
    const hide = () => setTip(null);

    const railLink = ({ to, label, icon: Icon, end }: NavItem) => (
        <li key={to}>
            <NavLink
                to={to}
                end={end}
                aria-label={label}
                onMouseEnter={show(label)}
                onMouseLeave={hide}
                onFocus={show(label)}
                onBlur={hide}
                onClick={hide}
                className={({ isActive }) =>
                    cn(
                        'relative flex h-10 w-10 items-center justify-center rounded-control transition-colors',
                        isActive
                            ? 'bg-primary/12 text-primary-text before:absolute before:-left-3 before:inset-y-2.5 before:w-[3px] before:rounded-r-full before:bg-primary'
                            : 'text-muted hover:bg-secondary hover:text-foreground',
                    )
                }
            >
                <Icon size={19} aria-hidden />
            </NavLink>
        </li>
    );

    return (
        <>
            <aside className="fixed inset-y-0 left-0 z-30 hidden w-16 flex-col items-center border-r border-border bg-card lg:flex">
                <Link
                    to="/"
                    aria-label="Atlas home"
                    className="flex h-14 w-full shrink-0 items-center justify-center"
                    onMouseEnter={show('Atlas home')}
                    onMouseLeave={hide}
                    onFocus={show('Atlas home')}
                    onBlur={hide}
                >
                    <LogoMark className="h-8 w-8" />
                </Link>

                <nav aria-label="Main" className="flex w-full flex-1 flex-col items-center gap-3 overflow-y-auto overflow-x-hidden py-2 [scrollbar-width:none]">
                    <ul className="flex flex-col items-center gap-1">{mainNav.map(railLink)}</ul>
                    {user?.is_admin && (
                        <>
                            <div className="h-px w-8 shrink-0 bg-border" aria-hidden />
                            <ul aria-label="Administration" className="flex flex-col items-center gap-1">
                                {adminNav.map(railLink)}
                            </ul>
                        </>
                    )}
                </nav>

                <div className="shrink-0 py-3">
                    <UserMenu placement="right" trigger="avatar" />
                </div>
            </aside>

            {tip && (
                <div
                    aria-hidden
                    className="pointer-events-none fixed left-[calc(var(--spacing)*16+0.5rem)] z-50 hidden -translate-y-1/2 animate-fade-in whitespace-nowrap rounded-control bg-foreground px-2.5 py-1 text-xs font-medium text-background shadow-pop lg:block"
                    style={{ top: tip.top }}
                >
                    {tip.label}
                </div>
            )}

            <MobileBar className="h-12" />
            <MobileMenu side="left" />
        </>
    );
}
