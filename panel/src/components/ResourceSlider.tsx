import { useId, type ReactNode } from 'react';
import { cn } from '../lib/cn';

export interface ResourceCapacity {
    /** Already given to other servers on the node. */
    allocated: number;
    /** The most the node may hand out in total. */
    total: number;
}

export interface ResourceSliderProps {
    label: string;
    /** The value as typed, so a half-typed number is not thrown away. */
    value: string;
    onChange: (value: string) => void;
    min: number;
    max: number;
    step: number;
    /** Renders an amount for people, e.g. "1.5 GB" or "2 cores". */
    format: (amount: number) => string;
    /** The unit shown beside the exact-value box, e.g. "MB" or "%". */
    unit: string;
    /** What the node has; draws the bar of used, this server and free. */
    capacity?: ResourceCapacity | null;
    /** A line under the control; replaced by the error when there is one. */
    hint?: ReactNode;
    error?: string;
    disabled?: boolean;
    className?: string;
}

function parse(value: string): number | null {
    const trimmed = value.trim();
    if (!/^\d+$/.test(trimmed)) return null;
    return Number(trimmed);
}

/**
 * A resource limit chosen with a slider, with a box beside it for an exact
 * value. The slider covers what is sensible for the node; the box accepts
 * anything, and the form decides whether it fits.
 */
export default function ResourceSlider({
    label,
    value,
    onChange,
    min,
    max,
    step,
    format,
    unit,
    capacity,
    hint,
    error,
    disabled,
    className,
}: ResourceSliderProps) {
    const id = useId();
    const amount = parse(value);
    const top = Math.max(max, min);
    const sliderValue = Math.min(Math.max(amount ?? min, min), top);
    const describedBy = error || hint ? `${id}-description` : undefined;

    // The bar is drawn against the node's total: what others have, what this
    // server would take, and what would be left.
    let bar: { used: number; mine: number; over: boolean } | null = null;
    if (capacity && capacity.total > 0) {
        const used = Math.min(capacity.allocated, capacity.total);
        const mine = Math.max(0, amount ?? 0);
        bar = {
            used: (used / capacity.total) * 100,
            mine: (Math.min(mine, capacity.total - used) / capacity.total) * 100,
            over: used + mine > capacity.total,
        };
    }

    return (
        <div className={cn('space-y-2', className)}>
            <div className="flex items-baseline justify-between gap-3">
                <label htmlFor={`${id}-range`} className="text-sm font-medium text-foreground">
                    {label}
                </label>
                <span className="text-sm font-semibold tabular-nums text-foreground">{amount === null ? '—' : format(amount)}</span>
            </div>

            <div className="flex items-center gap-3">
                <input
                    id={`${id}-range`}
                    type="range"
                    min={min}
                    max={top}
                    step={step}
                    value={sliderValue}
                    disabled={disabled}
                    aria-describedby={describedBy}
                    aria-valuetext={format(sliderValue)}
                    onChange={(event) => onChange(event.target.value)}
                    className="h-2 min-w-0 flex-1 cursor-pointer accent-primary disabled:cursor-not-allowed disabled:opacity-50"
                />
                <div className="relative w-28 shrink-0">
                    <input
                        type="text"
                        inputMode="numeric"
                        aria-label={`${label} in ${unit}`}
                        aria-invalid={error ? true : undefined}
                        aria-describedby={describedBy}
                        value={value}
                        disabled={disabled}
                        onChange={(event) => onChange(event.target.value)}
                        className={cn(
                            'h-9 w-full rounded-control border border-border-strong bg-background pl-3 pr-10 text-right text-sm tabular-nums text-foreground',
                            'focus:border-primary focus:outline-none focus:ring-2 focus:ring-primary/25 disabled:cursor-not-allowed disabled:bg-secondary',
                            'aria-[invalid=true]:border-danger',
                        )}
                    />
                    <span className="pointer-events-none absolute right-3 top-1/2 -translate-y-1/2 text-xs text-muted">{unit}</span>
                </div>
            </div>

            {bar && (
                <div
                    className="flex h-1.5 overflow-hidden rounded-full bg-secondary"
                    role="img"
                    aria-label={`${format(capacity!.allocated)} already given to other servers, ${amount === null ? 'nothing' : format(amount)} for this one, out of ${format(capacity!.total)}`}
                >
                    <div className="h-full bg-subtle/60" style={{ width: `${bar.used}%` }} />
                    <div className={cn('h-full', bar.over ? 'bg-danger' : 'bg-primary')} style={{ width: `${bar.mine}%` }} />
                </div>
            )}

            {(error || hint) && (
                <p id={describedBy} role={error ? 'alert' : undefined} className={cn('text-xs', error ? 'text-danger' : 'text-muted')}>
                    {error || hint}
                </p>
            )}
        </div>
    );
}
