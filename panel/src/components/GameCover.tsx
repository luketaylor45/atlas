import { useId, type ReactNode } from 'react';
import { coverGradient, type GameIdentity } from '../lib/gameIdentity';
import { cn } from '../lib/cn';

/** One of four light line patterns laid over a cover. */
function CoverPattern({ pattern }: { pattern: number }) {
    const id = `cover-${useId().replace(/[^a-zA-Z0-9_-]/g, '')}`;
    return (
        <svg
            viewBox="0 0 320 100"
            preserveAspectRatio="xMaxYMid slice"
            className="absolute inset-0 h-full w-full text-white"
            aria-hidden
            focusable="false"
        >
            {pattern === 0 &&
                [26, 48, 70, 92, 114].map((r) => (
                    <circle key={r} cx="282" cy="50" r={r} fill="none" stroke="currentColor" strokeOpacity="0.16" />
                ))}
            {pattern === 1 &&
                Array.from({ length: 18 }, (_, index) => (
                    <path
                        key={index}
                        d={`M${index * 22 - 60} 110L${index * 22 + 10} -10`}
                        stroke="currentColor"
                        strokeOpacity="0.1"
                        strokeWidth="7"
                    />
                ))}
            {pattern === 2 && (
                <>
                    <defs>
                        <pattern id={id} width="14" height="14" patternUnits="userSpaceOnUse">
                            <circle cx="7" cy="7" r="1.4" fill="currentColor" fillOpacity="0.28" />
                        </pattern>
                    </defs>
                    <rect x="120" width="200" height="100" fill={`url(#${id})`} />
                </>
            )}
            {pattern === 3 &&
                [14, 30, 46, 62, 78, 94].map((y) => (
                    <path
                        key={y}
                        d={`M0 ${y}C80 ${y - 18} 170 ${y + 20} 320 ${y - 8}`}
                        fill="none"
                        stroke="currentColor"
                        strokeOpacity="0.17"
                    />
                ))}
        </svg>
    );
}

interface GameCoverProps {
    identity: GameIdentity;
    className?: string;
    children?: ReactNode;
}

/**
 * A generated cover for a server: a gradient in the game's colour, a line
 * pattern and its category icon as a watermark. Decorative.
 */
export default function GameCover({ identity, className, children }: GameCoverProps) {
    const { icon: Icon, hue, pattern } = identity;
    return (
        <div className={cn('relative overflow-hidden', className)} style={coverGradient(hue)}>
            <CoverPattern pattern={pattern} />
            <Icon
                className="absolute -bottom-5 right-3 h-24 w-24 -rotate-12 text-white/20"
                strokeWidth={1.25}
                aria-hidden
            />
            {children}
        </div>
    );
}

/** The game's icon on its gradient, as a small square badge. */
export function GameIcon({ identity, className, size = 20 }: { identity: GameIdentity; className?: string; size?: number }) {
    const { icon: Icon, hue } = identity;
    return (
        <span
            className={cn('flex shrink-0 items-center justify-center text-white shadow-[inset_0_1px_0_rgb(255_255_255/0.25)]', className)}
            style={coverGradient(hue)}
            aria-hidden
        >
            <Icon size={size} strokeWidth={2} />
        </span>
    );
}
