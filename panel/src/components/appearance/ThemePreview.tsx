import { useId } from 'react';
import type { LayoutTheme } from '../../context/theme';
import { cn } from '../../lib/cn';

// Small drawings of each theme's layout, in the current mode's colours.
// Everything is on a 160 x 100 grid and scales with the box it sits in.

const card = 'fill-card stroke-border';

function ClassicPreview() {
    return (
        <>
            <rect width="160" height="100" className="fill-background" />
            <rect width="38" height="100" className="fill-card" />
            <path d="M38.5 0v100" className="stroke-border" />
            <circle cx="9" cy="9" r="3.2" className="fill-primary" />
            <rect x="15" y="7.5" width="15" height="3" rx="1.5" className="fill-foreground/70" />
            <rect x="4" y="18" width="30" height="7" rx="2" className="fill-primary/15" />
            <rect x="4" y="18" width="1.6" height="7" rx="0.8" className="fill-primary" />
            <rect x="8" y="20.5" width="18" height="2" rx="1" className="fill-primary" />
            {[30, 38, 46].map((y) => (
                <rect key={y} x="8" y={y} width="20" height="2" rx="1" className="fill-subtle/50" />
            ))}
            <rect x="8" y="58" width="12" height="1.6" rx="0.8" className="fill-subtle/40" />
            {[64, 72].map((y) => (
                <rect key={y} x="8" y={y} width="20" height="2" rx="1" className="fill-subtle/50" />
            ))}
            <rect x="46" y="10" width="44" height="5" rx="2" className="fill-foreground/80" />
            {[46, 83, 120].map((x) => (
                <rect key={x} x={x} y="22" width="33" height="17" rx="3" className={card} strokeWidth="0.8" />
            ))}
            <rect x="46" y="45" width="107" height="47" rx="3" className={card} strokeWidth="0.8" />
            {[53, 63, 73, 83].map((y) => (
                <rect key={y} x="52" y={y} width={y === 53 ? 40 : 70} height="2.4" rx="1.2" className="fill-subtle/40" />
            ))}
        </>
    );
}

function HorizonPreview({ id }: { id: string }) {
    return (
        <>
            <defs>
                <radialGradient id={`${id}-glow`} cx="0.5" cy="0" r="0.75">
                    <stop offset="0" style={{ stopColor: 'var(--primary)', stopOpacity: 0.32 }} />
                    <stop offset="1" style={{ stopColor: 'var(--primary)', stopOpacity: 0 }} />
                </radialGradient>
            </defs>
            <rect width="160" height="100" className="fill-background" />
            <rect y="14" width="160" height="42" fill={`url(#${id}-glow)`} />
            <rect width="160" height="14" className="fill-card" />
            <path d="M0 14.5h160" className="stroke-border" />
            <circle cx="10" cy="7" r="3.2" className="fill-primary" />
            <rect x="21" y="3.5" width="18" height="7" rx="3.5" className="fill-primary/15" />
            <rect x="25" y="6" width="10" height="2" rx="1" className="fill-primary" />
            {[45, 61, 77].map((x) => (
                <rect key={x} x={x} y="6" width="11" height="2" rx="1" className="fill-subtle/60" />
            ))}
            <circle cx="150" cy="7" r="3.4" className="fill-primary/60" />
            <rect x="22" y="23" width="52" height="6" rx="3" className="fill-foreground/85" />
            <rect x="22" y="32" width="34" height="2.4" rx="1.2" className="fill-subtle/50" />
            <rect x="22" y="40" width="56" height="27" rx="6" className={card} strokeWidth="0.8" />
            <rect x="82" y="40" width="56" height="27" rx="6" className={card} strokeWidth="0.8" />
            <rect x="22" y="40" width="56" height="9" rx="6" className="fill-primary/25" />
            <rect x="82" y="40" width="56" height="9" rx="6" className="fill-primary/15" />
            <rect x="22" y="71" width="116" height="24" rx="6" className={card} strokeWidth="0.8" />
        </>
    );
}

function CompactPreview() {
    const rows = [14, 23, 32, 41, 50, 59, 68, 77, 86];
    return (
        <>
            <rect width="160" height="100" className="fill-background" />
            <rect width="15" height="100" className="fill-card" />
            <path d="M15.5 0v100" className="stroke-border" />
            <circle cx="7.5" cy="7" r="3" className="fill-primary" />
            <rect x="3.5" y="16" width="8" height="8" rx="1.5" className="fill-primary/20" />
            <rect x="5.5" y="18" width="4" height="4" rx="1" className="fill-primary" />
            {[29, 37, 45, 57, 65, 73].map((y) => (
                <rect key={y} x="5.5" y={y} width="4" height="4" rx="1" className="fill-subtle/60" />
            ))}
            <circle cx="7.5" cy="92" r="3" className="fill-primary/60" />
            <rect x="21" y="5" width="30" height="4" rx="1" className="fill-foreground/80" />
            {rows.map((y, index) => (
                <g key={y}>
                    <rect x="21" y={y} width="134" height="7" rx="1" className={card} strokeWidth="0.6" />
                    <circle
                        cx="25"
                        cy={y + 3.5}
                        r="1.3"
                        className={index % 4 === 2 ? 'fill-subtle' : index % 5 === 3 ? 'fill-warning' : 'fill-success'}
                    />
                    <rect x="29" y={y + 2.6} width="30" height="1.8" rx="0.9" className="fill-foreground/60" />
                    <rect x="92" y={y + 2.6} width="22" height="1.8" rx="0.9" className="fill-primary/40" />
                    <rect x="120" y={y + 2.6} width="28" height="1.8" rx="0.9" className="fill-subtle/40" />
                </g>
            ))}
        </>
    );
}

function GlassPreview({ id }: { id: string }) {
    const pane = 'fill-white/60 stroke-white/80 dark:fill-white/[0.08] dark:stroke-white/15';
    return (
        <>
            <defs>
                <radialGradient id={`${id}-a`} cx="0.1" cy="0" r="0.7">
                    <stop offset="0" style={{ stopColor: 'var(--primary)', stopOpacity: 0.55 }} />
                    <stop offset="1" style={{ stopColor: 'var(--primary)', stopOpacity: 0 }} />
                </radialGradient>
                <radialGradient id={`${id}-b`} cx="1" cy="0.15" r="0.6">
                    <stop offset="0" stopColor="#7c3aed" stopOpacity="0.4" />
                    <stop offset="1" stopColor="#7c3aed" stopOpacity="0" />
                </radialGradient>
                <radialGradient id={`${id}-c`} cx="0.65" cy="1.05" r="0.7">
                    <stop offset="0" stopColor="#2563eb" stopOpacity="0.35" />
                    <stop offset="1" stopColor="#2563eb" stopOpacity="0" />
                </radialGradient>
            </defs>
            <rect width="160" height="100" className="fill-background" />
            <rect width="160" height="100" fill={`url(#${id}-a)`} />
            <rect width="160" height="100" fill={`url(#${id}-b)`} />
            <rect width="160" height="100" fill={`url(#${id}-c)`} />
            <rect x="4" y="4" width="34" height="92" rx="7" className={pane} strokeWidth="0.8" />
            <circle cx="12" cy="13" r="3" className="fill-primary" />
            <rect x="8" y="22" width="26" height="7" rx="3.5" className="fill-primary" />
            {[34, 42, 50].map((y) => (
                <rect key={y} x="11" y={y} width="18" height="2" rx="1" className="fill-foreground/40" />
            ))}
            <rect x="46" y="9" width="40" height="5" rx="2.5" className="fill-foreground/80" />
            <rect x="46" y="20" width="53" height="32" rx="7" className={pane} strokeWidth="0.8" />
            <rect x="103" y="20" width="53" height="32" rx="7" className={pane} strokeWidth="0.8" />
            <rect x="46" y="56" width="110" height="40" rx="7" className={pane} strokeWidth="0.8" />
        </>
    );
}

/** A miniature of a theme's layout, drawn in the current light or dark colours. */
export default function ThemePreview({ theme, className }: { theme: LayoutTheme; className?: string }) {
    const id = `preview-${useId().replace(/[^a-zA-Z0-9_-]/g, '')}`;
    return (
        <svg
            viewBox="0 0 160 100"
            preserveAspectRatio="xMidYMid slice"
            className={cn('block h-auto w-full', className)}
            aria-hidden
            focusable="false"
        >
            {theme === 'classic' && <ClassicPreview />}
            {theme === 'horizon' && <HorizonPreview id={id} />}
            {theme === 'compact' && <CompactPreview />}
            {theme === 'glass' && <GlassPreview id={id} />}
        </svg>
    );
}
