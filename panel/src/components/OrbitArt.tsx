import { useId } from 'react';
import { cn } from '../lib/cn';

/**
 * A line drawing of the Atlas mark at large scale: a banded planet with
 * tilted orbits and satellites. Decorative; takes its colour from `color`.
 */
export default function OrbitArt({ className }: { className?: string }) {
    const id = `orbit-${useId().replace(/[^a-zA-Z0-9_-]/g, '')}`;
    return (
        <svg viewBox="0 0 400 400" fill="none" className={cn('pointer-events-none', className)} aria-hidden focusable="false">
            <defs>
                <radialGradient id={id} cx="0.35" cy="0.3" r="0.8">
                    <stop offset="0" stopColor="currentColor" stopOpacity="0.28" />
                    <stop offset="1" stopColor="currentColor" stopOpacity="0.04" />
                </radialGradient>
                <clipPath id={`${id}-planet`}>
                    <circle cx="200" cy="200" r="86" />
                </clipPath>
            </defs>
            <g transform="rotate(-22 200 200)" stroke="currentColor">
                <ellipse cx="200" cy="200" rx="196" ry="72" strokeOpacity="0.18" />
                <ellipse cx="200" cy="200" rx="142" ry="46" strokeOpacity="0.3" strokeDasharray="3 7" />
            </g>
            <circle cx="200" cy="200" r="86" fill={`url(#${id})`} stroke="currentColor" strokeOpacity="0.5" />
            {/* Latitudes parallel to the orbit, as in the mark. */}
            <g clipPath={`url(#${id}-planet)`} transform="rotate(-22 200 200)" stroke="currentColor">
                <path d="M20 127A180 58 0 0 0 380 127" strokeOpacity="0.5" strokeWidth="1.5" />
                <path d="M20 92A180 58 0 0 0 380 92" strokeOpacity="0.2" />
                <ellipse cx="200" cy="200" rx="34" ry="86" strokeOpacity="0.18" />
            </g>
            <g transform="rotate(-22 200 200)">
                <ellipse cx="200" cy="200" rx="176" ry="56" stroke="currentColor" strokeOpacity="0.55" strokeWidth="1.5" />
                <circle cx="352.4" cy="228" r="7" fill="currentColor" />
                <circle cx="68.4" cy="185" r="4.5" fill="currentColor" fillOpacity="0.6" />
            </g>
        </svg>
    );
}
