import { forwardRef, type ButtonHTMLAttributes, type ReactNode } from 'react';
import { Loader2 } from 'lucide-react';
import { cn } from '../../lib/cn';

export type ButtonVariant = 'primary' | 'secondary' | 'outline' | 'ghost' | 'danger' | 'danger-outline';
export type ButtonSize = 'sm' | 'md' | 'lg';

const base =
    'inline-flex items-center justify-center gap-2 whitespace-nowrap font-medium rounded-control select-none ' +
    'transition-[background-color,border-color,color,box-shadow,transform] duration-150 ' +
    'active:scale-[0.98] disabled:pointer-events-none disabled:opacity-50';

const variants: Record<ButtonVariant, string> = {
    primary: 'bg-primary text-primary-foreground hover:bg-primary-hover shadow-card',
    secondary: 'bg-secondary text-foreground hover:bg-border border border-border',
    outline: 'border border-border-strong text-foreground hover:bg-secondary',
    ghost: 'text-muted hover:text-foreground hover:bg-secondary',
    danger: 'bg-danger text-white hover:brightness-110 shadow-card dark:text-[#1a0606]',
    'danger-outline': 'border border-danger/40 text-danger hover:bg-danger/10',
};

const sizes: Record<ButtonSize, string> = {
    sm: 'h-8 px-3 text-xs',
    md: 'h-9 px-4 text-sm',
    lg: 'h-11 px-5 text-sm',
};

export interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
    variant?: ButtonVariant;
    size?: ButtonSize;
    /** Shows a spinner and disables the button. */
    loading?: boolean;
    /** Icon placed before the label. */
    icon?: ReactNode;
    fullWidth?: boolean;
}

export const Button = forwardRef<HTMLButtonElement, ButtonProps>(function Button(
    { variant = 'secondary', size = 'md', loading = false, icon, fullWidth, className, children, disabled, type = 'button', ...props },
    ref,
) {
    return (
        <button
            ref={ref}
            type={type}
            disabled={disabled || loading}
            aria-busy={loading || undefined}
            className={cn(base, variants[variant], sizes[size], fullWidth && 'w-full', className)}
            {...props}
        >
            {loading ? <Loader2 size={16} className="animate-spin" aria-hidden /> : icon}
            {children}
        </button>
    );
});

export interface IconButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
    /** Required: icon-only buttons need a name for screen readers and a tooltip. */
    label: string;
    variant?: ButtonVariant;
    size?: ButtonSize;
    loading?: boolean;
}

const iconSizes: Record<ButtonSize, string> = {
    sm: 'h-8 w-8',
    md: 'h-9 w-9',
    lg: 'h-11 w-11',
};

export const IconButton = forwardRef<HTMLButtonElement, IconButtonProps>(function IconButton(
    { label, variant = 'ghost', size = 'md', loading = false, className, children, disabled, type = 'button', ...props },
    ref,
) {
    return (
        <button
            ref={ref}
            type={type}
            aria-label={label}
            title={label}
            disabled={disabled || loading}
            className={cn(base, variants[variant], iconSizes[size], 'px-0', className)}
            {...props}
        >
            {loading ? <Loader2 size={16} className="animate-spin" aria-hidden /> : children}
        </button>
    );
});
