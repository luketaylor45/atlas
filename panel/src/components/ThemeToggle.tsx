import { Monitor, Moon, Sun } from 'lucide-react';
import { useTheme, type ThemePreference } from '../context/theme';
import { cn } from '../lib/cn';

const options: { value: ThemePreference; label: string; icon: typeof Sun }[] = [
    { value: 'light', label: 'Light', icon: Sun },
    { value: 'system', label: 'System', icon: Monitor },
    { value: 'dark', label: 'Dark', icon: Moon },
];

/** Three-way theme switch: light, follow the system, dark. */
export default function ThemeToggle({ className, showLabels = false }: { className?: string; showLabels?: boolean }) {
    const { preference, setPreference } = useTheme();

    return (
        <div
            role="radiogroup"
            aria-label="Theme"
            className={cn('inline-flex items-center gap-0.5 rounded-control border border-border bg-secondary p-0.5', className)}
        >
            {options.map(({ value, label, icon: Icon }) => {
                const active = preference === value;
                return (
                    <button
                        key={value}
                        type="button"
                        role="radio"
                        aria-checked={active}
                        aria-label={showLabels ? undefined : `${label} theme`}
                        title={`${label} theme`}
                        onClick={() => setPreference(value)}
                        className={cn(
                            'inline-flex h-7 items-center justify-center gap-1.5 rounded-[6px] text-xs font-medium transition-colors',
                            showLabels ? 'px-3' : 'w-7',
                            active ? 'bg-card text-foreground shadow-card' : 'text-muted hover:text-foreground',
                        )}
                    >
                        <Icon size={14} aria-hidden />
                        {showLabels && label}
                    </button>
                );
            })}
        </div>
    );
}
