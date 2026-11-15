import { useId, useRef, type KeyboardEvent } from 'react';
import { Check } from 'lucide-react';
import { useTheme } from '../../context/theme';
import { cn } from '../../lib/cn';
import ThemePreview from './ThemePreview';
import { themeOptions } from './themeOptions';

/** The full theme chooser for the settings page: a preview card per theme. */
export default function ThemePicker() {
    const { layout, setLayout } = useTheme();
    const name = useId();

    return (
        <fieldset>
            <legend className="sr-only">Theme</legend>
            <div className="grid grid-cols-1 gap-3 min-[420px]:grid-cols-2 xl:grid-cols-4">
                {themeOptions.map((option) => {
                    const active = option.id === layout;
                    return (
                        <label
                            key={option.id}
                            className={cn(
                                'relative flex cursor-pointer flex-col overflow-hidden rounded-card border bg-card transition-[border-color,box-shadow] duration-150',
                                'has-[input:focus-visible]:outline-2 has-[input:focus-visible]:outline-offset-2 has-[input:focus-visible]:outline-primary',
                                active ? 'border-primary ring-2 ring-primary/25' : 'border-border hover:border-border-strong',
                            )}
                        >
                            <input
                                type="radio"
                                name={name}
                                value={option.id}
                                checked={active}
                                onChange={() => setLayout(option.id)}
                                className="sr-only"
                            />
                            <span className="block border-b border-border bg-secondary/50 p-2.5">
                                <span className="block overflow-hidden rounded-[calc(var(--card-radius)-6px)] ring-1 ring-border">
                                    <ThemePreview theme={option.id} />
                                </span>
                            </span>
                            <span className="flex items-start gap-3 p-3.5">
                                <span className="min-w-0 flex-1">
                                    <span className="block text-sm font-medium text-foreground">{option.name}</span>
                                    <span className="mt-0.5 block text-xs leading-relaxed text-muted">{option.description}</span>
                                </span>
                                <span
                                    className={cn(
                                        'mt-0.5 flex h-5 w-5 shrink-0 items-center justify-center rounded-full border transition-colors',
                                        active ? 'border-primary bg-primary text-primary-foreground' : 'border-border-strong',
                                    )}
                                    aria-hidden
                                >
                                    {active && <Check size={12} strokeWidth={3} />}
                                </span>
                            </span>
                        </label>
                    );
                })}
            </div>
        </fieldset>
    );
}

/** A small theme switch with thumbnails, for the account menu. Arrow keys move between themes. */
export function ThemeQuickSwitch({ className }: { className?: string }) {
    const { layout, setLayout } = useTheme();
    const group = useRef<HTMLDivElement>(null);

    const onKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
        const step = { ArrowRight: 1, ArrowDown: 1, ArrowLeft: -1, ArrowUp: -1 }[event.key];
        if (!step) return;
        event.preventDefault();
        const index = themeOptions.findIndex((option) => option.id === layout);
        const next = (index + step + themeOptions.length) % themeOptions.length;
        setLayout(themeOptions[next].id);
        group.current?.querySelectorAll<HTMLButtonElement>('[role="radio"]')[next]?.focus();
    };

    return (
        <div ref={group} role="radiogroup" aria-label="Theme" className={cn('grid grid-cols-4 gap-1', className)} onKeyDown={onKeyDown}>
            {themeOptions.map((option) => {
                const active = option.id === layout;
                return (
                    <button
                        key={option.id}
                        type="button"
                        role="radio"
                        aria-checked={active}
                        tabIndex={active ? 0 : -1}
                        title={option.description}
                        onClick={() => setLayout(option.id)}
                        className={cn(
                            'flex min-w-0 flex-col items-center gap-1 rounded-control p-1 text-[11px] font-medium transition-colors',
                            active ? 'bg-primary/10 text-primary-text' : 'text-muted hover:bg-secondary hover:text-foreground',
                        )}
                    >
                        <span
                            className={cn(
                                'block w-full overflow-hidden rounded-[4px] ring-1',
                                active ? 'ring-2 ring-primary' : 'ring-border',
                            )}
                        >
                            <ThemePreview theme={option.id} />
                        </span>
                        {option.name}
                    </button>
                );
            })}
        </div>
    );
}
