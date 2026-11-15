import { NavLink } from 'react-router-dom';
import { cn } from '../../lib/cn';
import type { NavItem } from './nav';

/** The first thing a keyboard user reaches: jumps past the navigation. */
export function SkipLink() {
    return (
        <a
            href="#main"
            className="sr-only focus:not-sr-only focus:fixed focus:left-4 focus:top-4 focus:z-[70] focus:rounded-control focus:bg-primary focus:px-4 focus:py-2 focus:text-sm focus:text-primary-foreground"
        >
            Skip to content
        </a>
    );
}

const avatarSizes = {
    sm: 'h-7 w-7 text-[11px]',
    md: 'h-8 w-8 text-xs',
    lg: 'h-10 w-10 text-sm',
};

/** The signed-in person's initial on an accent disc. */
export function Avatar({ name, size = 'md', className }: { name?: string; size?: keyof typeof avatarSizes; className?: string }) {
    return (
        <span
            className={cn(
                'flex shrink-0 items-center justify-center rounded-full bg-linear-to-br from-[#8b85f7] via-primary to-[#3127b8] font-semibold uppercase text-white shadow-[inset_0_1px_0_rgb(255_255_255/0.25)]',
                avatarSizes[size],
                className,
            )}
            aria-hidden
        >
            {name?.charAt(0) || '?'}
        </span>
    );
}

export type SideNavVariant = 'classic' | 'glass';

const sideLink: Record<SideNavVariant, { base: string; active: string; idle: string }> = {
    classic: {
        base: 'relative flex items-center gap-3 rounded-control px-3 py-2 text-sm transition-colors',
        // A short accent bar at the sidebar's edge marks the current page.
        active:
            'bg-primary/10 font-medium text-primary-text before:absolute before:-left-3 before:inset-y-1.5 before:w-[3px] before:rounded-r-full before:bg-primary',
        idle: 'text-muted hover:bg-secondary hover:text-foreground',
    },
    glass: {
        base: 'flex items-center gap-3 rounded-full px-3.5 py-2 text-sm transition-colors',
        active: 'bg-primary font-medium text-primary-foreground shadow-[0_8px_20px_-8px_rgb(79_70_229/0.7)]',
        idle: 'text-muted hover:bg-foreground/[0.06] hover:text-foreground',
    },
};

/** A titled list of links in a vertical sidebar or drawer. */
export function SideNavSection({
    title,
    items,
    variant = 'classic',
}: {
    title?: string;
    items: NavItem[];
    variant?: SideNavVariant;
}) {
    const style = sideLink[variant];
    return (
        <div>
            {title && <div className="mb-2 px-3 text-[11px] font-semibold uppercase tracking-[0.08em] text-muted">{title}</div>}
            <ul className="space-y-0.5">
                {items.map(({ to, label, icon: Icon, end }) => (
                    <li key={to}>
                        <NavLink
                            to={to}
                            end={end}
                            className={({ isActive }) => cn(style.base, isActive ? style.active : style.idle)}
                        >
                            <Icon size={18} aria-hidden />
                            {label}
                        </NavLink>
                    </li>
                ))}
            </ul>
        </div>
    );
}
