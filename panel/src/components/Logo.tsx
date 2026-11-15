import { useId } from 'react';
import { cn } from '../lib/cn';

// The mark: a planet held in a tilted orbit with one satellite on it. A
// latitude cut parallel to the orbit and the orbit itself split the planet
// into three slabs, like a stack of servers. The orbit passes behind the
// planet at the top and in front of it at the bottom; a mask cuts a gap so
// the crossing reads at small sizes.
//
// Drawn on a 32-unit grid. The orbit is drawn level and tilted as a group.
const TILT = 'rotate(-22 16 16)';
const ORBIT_FRONT = 'M1.6 16A14.4 4.7 0 0 0 30.4 16';
const LATITUDE = 'M1.6 9.6A14.4 4.7 0 0 0 30.4 9.6';

interface LogoMarkProps {
    className?: string;
}

/** The Atlas mark on its own. Uses theme tokens, so it reads on any surface. */
export function LogoMark({ className }: LogoMarkProps) {
    // Gradient and mask ids must be unique per instance and safe inside url().
    const id = `atlas-${useId().replace(/[^a-zA-Z0-9_-]/g, '')}`;

    return (
        <svg viewBox="0 0 32 32" className={cn('shrink-0 text-primary-text', className)} aria-hidden focusable="false">
            <defs>
                <linearGradient id={`${id}-planet`} x1="9" y1="7" x2="23" y2="25" gradientUnits="userSpaceOnUse">
                    <stop offset="0" stopColor="#8b85f7" />
                    <stop offset="0.5" stopColor="#4f46e5" />
                    <stop offset="1" stopColor="#3127b8" />
                </linearGradient>
                <mask id={`${id}-cut`} maskUnits="userSpaceOnUse" x="0" y="0" width="32" height="32">
                    <rect width="32" height="32" fill="#fff" />
                    <g transform={TILT} fill="none" stroke="#000">
                        <path d={LATITUDE} strokeWidth="1.6" />
                        <path d={ORBIT_FRONT} strokeWidth="4.6" />
                    </g>
                </mask>
            </defs>

            {/* Back of the orbit; the planet covers the part behind it. */}
            <ellipse cx="16" cy="16" rx="14.4" ry="4.7" transform={TILT} fill="none" stroke="currentColor" strokeWidth="1.7" />
            <circle cx="16" cy="16" r="9.6" fill={`url(#${id}-planet)`} mask={`url(#${id}-cut)`} />
            {/* Front of the orbit and its satellite. */}
            <g transform={TILT}>
                <path d={ORBIT_FRONT} fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" />
                <circle cx="27.8" cy="18.7" r="2.1" fill="currentColor" />
            </g>
        </svg>
    );
}

interface LogoProps {
    className?: string;
    size?: 'sm' | 'md' | 'lg';
    /** "mark" shows the symbol without the name, for narrow places. */
    variant?: 'full' | 'mark';
    /** "inverse" is for dark brand surfaces that do not follow the mode. */
    tone?: 'default' | 'inverse';
}

const sizes = {
    sm: { mark: 'h-7 w-7', text: 'text-[1.0625rem]', gap: 'gap-2' },
    md: { mark: 'h-8 w-8', text: 'text-lg', gap: 'gap-2.5' },
    lg: { mark: 'h-12 w-12', text: 'text-[1.75rem]', gap: 'gap-3' },
};

/** The Atlas mark and wordmark. */
export default function Logo({ className, size = 'md', variant = 'full', tone = 'default' }: LogoProps) {
    const style = sizes[size];
    const inverse = tone === 'inverse';
    return (
        <span className={cn('inline-flex select-none items-center', style.gap, className)}>
            <LogoMark className={cn(style.mark, inverse && 'text-[#a5b0ff]')} />
            {variant === 'full' && (
                <span className={cn('font-bold leading-none tracking-[-0.035em]', inverse ? 'text-white' : 'text-foreground', style.text)}>
                    Atlas
                </span>
            )}
        </span>
    );
}
