import { useEffect, useRef, useState, type HTMLAttributes, type ReactNode, type TdHTMLAttributes, type ThHTMLAttributes } from 'react';
import { AlertTriangle, Check, CheckCircle2, Copy, Info, Loader2, XCircle } from 'lucide-react';
import { cn } from '../../lib/cn';

// ----------------------------------------------------------------- Card

export interface CardProps extends HTMLAttributes<HTMLDivElement> {
    /** Removes the default padding, for cards that hold a table or list. */
    flush?: boolean;
}

export function Card({ className, flush, ...props }: CardProps) {
    return (
        <div
            className={cn('surface rounded-card border border-border bg-card shadow-card', !flush && 'p-5 sm:p-6', className)}
            {...props}
        />
    );
}

export interface CardHeaderProps {
    title: ReactNode;
    description?: ReactNode;
    actions?: ReactNode;
    icon?: ReactNode;
    className?: string;
}

/** Title row for a card. Inside a `flush` card it supplies its own padding. */
export function CardHeader({ title, description, actions, icon, className }: CardHeaderProps) {
    return (
        <div className={cn('mb-5 flex flex-wrap items-start justify-between gap-3', className)}>
            <div className="flex min-w-0 items-start gap-3">
                {icon && <div className="mt-0.5 shrink-0 text-muted">{icon}</div>}
                <div className="min-w-0">
                    <h2 className="text-base font-semibold tracking-tight text-foreground">{title}</h2>
                    {description && <p className="mt-1 text-sm text-muted">{description}</p>}
                </div>
            </div>
            {actions && <div className="flex shrink-0 flex-wrap items-center gap-2">{actions}</div>}
        </div>
    );
}

// ------------------------------------------------------------ PageHeader

export interface PageHeaderProps {
    title: ReactNode;
    description?: ReactNode;
    actions?: ReactNode;
    /** Shown above the title, e.g. a back link. */
    eyebrow?: ReactNode;
    className?: string;
}

/** The one heading at the top of every page. */
export function PageHeader({ title, description, actions, eyebrow, className }: PageHeaderProps) {
    return (
        <header className={cn('mb-6 flex flex-wrap items-end justify-between gap-4 sm:mb-8', className)}>
            <div className="min-w-0">
                {eyebrow && <div className="mb-2 text-sm text-muted">{eyebrow}</div>}
                <h1 className="page-title truncate text-foreground">{title}</h1>
                {description && <p className="mt-1.5 max-w-2xl text-sm text-muted">{description}</p>}
            </div>
            {actions && <div className="flex flex-wrap items-center gap-2">{actions}</div>}
        </header>
    );
}

// ----------------------------------------------------------------- Badge

export type Tone = 'neutral' | 'primary' | 'success' | 'warning' | 'danger' | 'info';

const badgeTones: Record<Tone, string> = {
    neutral: 'bg-secondary text-muted border-border',
    primary: 'bg-primary/10 text-primary-text border-primary/20',
    success: 'bg-success/10 text-success border-success/20',
    warning: 'bg-warning/10 text-warning border-warning/25',
    danger: 'bg-danger/10 text-danger border-danger/20',
    info: 'bg-info/10 text-info border-info/20',
};

const dotTones: Record<Tone, string> = {
    neutral: 'bg-subtle',
    primary: 'bg-primary',
    success: 'bg-success',
    warning: 'bg-warning',
    danger: 'bg-danger',
    info: 'bg-info',
};

export interface BadgeProps {
    tone?: Tone;
    /** Shows a status dot before the label. */
    dot?: boolean;
    /** Pulses the dot, for in-progress states. */
    pulse?: boolean;
    className?: string;
    children: ReactNode;
}

export function Badge({ tone = 'neutral', dot, pulse, className, children }: BadgeProps) {
    return (
        <span
            className={cn(
                'inline-flex items-center gap-1.5 whitespace-nowrap rounded-full border px-2 py-0.5 text-xs font-medium',
                badgeTones[tone],
                className,
            )}
        >
            {dot && <span className={cn('h-1.5 w-1.5 rounded-full', dotTones[tone], pulse && 'animate-pulse')} aria-hidden />}
            {children}
        </span>
    );
}

// ----------------------------------------------------------------- Alert

const alertTones: Record<Exclude<Tone, 'neutral' | 'primary'>, { box: string; icon: ReactNode }> = {
    info: { box: 'border-info/25 bg-info/5 text-foreground', icon: <Info size={16} className="text-info" aria-hidden /> },
    success: { box: 'border-success/25 bg-success/5 text-foreground', icon: <CheckCircle2 size={16} className="text-success" aria-hidden /> },
    warning: { box: 'border-warning/30 bg-warning/5 text-foreground', icon: <AlertTriangle size={16} className="text-warning" aria-hidden /> },
    danger: { box: 'border-danger/25 bg-danger/5 text-foreground', icon: <XCircle size={16} className="text-danger" aria-hidden /> },
};

export interface AlertProps {
    tone?: 'info' | 'success' | 'warning' | 'danger';
    title?: ReactNode;
    children?: ReactNode;
    actions?: ReactNode;
    className?: string;
}

/** An inline message that stays on the page (use toasts for transient ones). */
export function Alert({ tone = 'info', title, children, actions, className }: AlertProps) {
    const style = alertTones[tone];
    return (
        <div
            role={tone === 'danger' || tone === 'warning' ? 'alert' : 'status'}
            className={cn('flex items-start gap-3 rounded-card border p-4 text-sm', style.box, className)}
        >
            <span className="mt-0.5 shrink-0">{style.icon}</span>
            <div className="min-w-0 flex-1">
                {title && <p className="font-medium">{title}</p>}
                {children && <div className={cn('text-muted', title && 'mt-1')}>{children}</div>}
            </div>
            {actions && <div className="flex shrink-0 items-center gap-2">{actions}</div>}
        </div>
    );
}

// ------------------------------------------------------------ EmptyState

export interface EmptyStateProps {
    icon?: ReactNode;
    title: ReactNode;
    description?: ReactNode;
    action?: ReactNode;
    className?: string;
}

/** What a list shows when it has nothing in it. Always says what to do next. */
export function EmptyState({ icon, title, description, action, className }: EmptyStateProps) {
    return (
        <div className={cn('flex flex-col items-center justify-center px-6 py-14 text-center', className)}>
            {icon && (
                <div className="relative mb-4 flex h-12 w-12 items-center justify-center rounded-full bg-primary/10 text-primary-text ring-1 ring-primary/15">
                    {/* A faint orbit around the icon, echoing the logo. */}
                    <span className="absolute -inset-2 rounded-full border border-dashed border-primary/20" aria-hidden />
                    {icon}
                </div>
            )}
            <p className="text-sm font-medium text-foreground">{title}</p>
            {description && <p className="mt-1 max-w-sm text-sm text-muted">{description}</p>}
            {action && <div className="mt-5">{action}</div>}
        </div>
    );
}

// --------------------------------------------------------------- Loading

export function Spinner({ size = 18, className }: { size?: number; className?: string }) {
    return <Loader2 size={size} className={cn('animate-spin text-muted', className)} aria-hidden />;
}

/** A centred spinner with a label, for a page or panel that is loading. */
export function LoadingState({ label = 'Loading…', className }: { label?: string; className?: string }) {
    return (
        <div role="status" className={cn('flex items-center justify-center gap-3 py-16 text-sm text-muted', className)}>
            <Spinner />
            {label}
        </div>
    );
}

export function Skeleton({ className }: { className?: string }) {
    return <div className={cn('animate-pulse rounded-control bg-secondary', className)} aria-hidden />;
}

// ----------------------------------------------------------- ProgressBar

export interface ProgressBarProps {
    /** 0-100. */
    value: number;
    tone?: Tone;
    label?: string;
    className?: string;
}

export function ProgressBar({ value, tone = 'primary', label, className }: ProgressBarProps) {
    const clamped = Math.min(100, Math.max(0, Number(value) || 0));
    return (
        <div
            role="progressbar"
            aria-valuemin={0}
            aria-valuemax={100}
            aria-valuenow={Math.round(clamped)}
            aria-label={label}
            className={cn('h-1.5 w-full overflow-hidden rounded-full bg-secondary', className)}
        >
            <div className={cn('h-full rounded-full transition-[width] duration-500', dotTones[tone])} style={{ width: `${clamped}%` }} />
        </div>
    );
}

// ----------------------------------------------------------------- Table

/** Scrolls sideways on narrow screens instead of breaking the page. */
export function Table({ className, children }: { className?: string; children: ReactNode }) {
    return (
        <div className="w-full overflow-x-auto">
            <table className={cn('w-full border-collapse text-left text-sm', className)}>{children}</table>
        </div>
    );
}

export function Th({ className, ...props }: ThHTMLAttributes<HTMLTableCellElement>) {
    return (
        <th
            scope="col"
            className={cn('whitespace-nowrap border-b border-border px-4 py-2.5 text-xs font-medium text-muted first:pl-5 last:pr-5', className)}
            {...props}
        />
    );
}

export function Td({ className, ...props }: TdHTMLAttributes<HTMLTableCellElement>) {
    return <td className={cn('border-b border-border px-4 py-3 align-middle first:pl-5 last:pr-5', className)} {...props} />;
}

export function Tr({ className, ...props }: HTMLAttributes<HTMLTableRowElement>) {
    return <tr className={cn('transition-colors last:[&>td]:border-b-0 hover:bg-secondary/60', className)} {...props} />;
}

// ------------------------------------------------------------------ Tabs

export interface TabItem<T extends string> {
    id: T;
    label: ReactNode;
    icon?: ReactNode;
}

export interface TabsProps<T extends string> {
    tabs: TabItem<T>[];
    value: T;
    onChange: (id: T) => void;
    className?: string;
    /** Accessible name for the tab list. */
    label: string;
}

export function Tabs<T extends string>({ tabs, value, onChange, className, label }: TabsProps<T>) {
    const list = useRef<HTMLDivElement>(null);

    // On a narrow screen the strip scrolls sideways; keep the chosen tab in
    // view, including when the page opens on a tab near the end.
    useEffect(() => {
        const active = list.current?.querySelector<HTMLElement>('[aria-selected="true"]');
        active?.scrollIntoView({ block: 'nearest', inline: 'nearest' });
    }, [value]);

    return (
        <div
            ref={list}
            role="tablist"
            aria-label={label}
            className={cn('flex gap-1 overflow-x-auto border-b border-border [scrollbar-width:none]', className)}
        >
            {tabs.map((tab) => {
                const active = tab.id === value;
                return (
                    <button
                        key={tab.id}
                        type="button"
                        role="tab"
                        aria-selected={active}
                        onClick={() => onChange(tab.id)}
                        className={cn(
                            '-mb-px inline-flex shrink-0 items-center gap-2 border-b-2 px-3 py-2.5 text-sm font-medium transition-colors',
                            active ? 'border-primary text-foreground' : 'border-transparent text-muted hover:text-foreground',
                        )}
                    >
                        {tab.icon}
                        {tab.label}
                    </button>
                );
            })}
        </div>
    );
}

// ------------------------------------------------------------ CopyButton

/** Copies text and briefly confirms it. */
export function CopyButton({ value, label = 'Copy', className }: { value: string; label?: string; className?: string }) {
    const [copied, setCopied] = useState(false);

    const copy = async () => {
        try {
            await navigator.clipboard.writeText(value);
            setCopied(true);
            setTimeout(() => setCopied(false), 1500);
        } catch {
            // Clipboard access can be denied; the value stays selectable on screen.
        }
    };

    return (
        <button
            type="button"
            onClick={copy}
            aria-label={copied ? 'Copied' : label}
            title={copied ? 'Copied' : label}
            className={cn(
                'inline-flex h-7 w-7 shrink-0 items-center justify-center rounded-control text-muted transition-colors hover:bg-secondary hover:text-foreground',
                className,
            )}
        >
            {copied ? <Check size={14} className="text-success" aria-hidden /> : <Copy size={14} aria-hidden />}
        </button>
    );
}

// ------------------------------------------------------------- StatTile

export interface StatTileProps {
    label: ReactNode;
    value: ReactNode;
    /** Secondary line under the value. */
    detail?: ReactNode;
    icon?: ReactNode;
    /** Colour of the icon chip. */
    tone?: Tone;
    className?: string;
}

const statIconTones: Record<Tone, string> = {
    neutral: 'bg-secondary text-muted ring-border',
    primary: 'bg-primary/10 text-primary-text ring-primary/15',
    success: 'bg-success/10 text-success ring-success/20',
    warning: 'bg-warning/10 text-warning ring-warning/20',
    danger: 'bg-danger/10 text-danger ring-danger/20',
    info: 'bg-info/10 text-info ring-info/20',
};

/** One headline number with its label. */
export function StatTile({ label, value, detail, icon, tone = 'primary', className }: StatTileProps) {
    return (
        <Card className={cn('p-4 sm:p-5', className)}>
            <div className="flex items-center justify-between gap-3 text-sm text-muted">
                <span>{label}</span>
                {icon && (
                    <span
                        className={cn(
                            'flex h-8 w-8 shrink-0 items-center justify-center rounded-control ring-1 ring-inset',
                            statIconTones[tone],
                        )}
                    >
                        {icon}
                    </span>
                )}
            </div>
            <div className="tabular mt-1.5 text-2xl font-semibold tracking-tight text-foreground">{value}</div>
            {detail && <div className="mt-1 text-xs text-muted">{detail}</div>}
        </Card>
    );
}
