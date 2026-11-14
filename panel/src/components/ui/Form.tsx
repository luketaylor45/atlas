import {
    forwardRef,
    useId,
    type InputHTMLAttributes,
    type ReactNode,
    type SelectHTMLAttributes,
    type TextareaHTMLAttributes,
} from 'react';
import { ChevronDown } from 'lucide-react';
import { cn } from '../../lib/cn';

const control =
    'w-full rounded-control border border-border-strong bg-background px-3 text-sm text-foreground ' +
    'placeholder:text-subtle transition-[border-color,box-shadow] duration-150 ' +
    'hover:border-subtle focus:border-primary focus:outline-none focus:ring-2 focus:ring-primary/25 ' +
    'disabled:cursor-not-allowed disabled:bg-secondary disabled:text-muted disabled:shadow-none ' +
    'aria-[invalid=true]:border-danger aria-[invalid=true]:focus:ring-danger/25';

export interface InputProps extends InputHTMLAttributes<HTMLInputElement> {
    invalid?: boolean;
}

export const Input = forwardRef<HTMLInputElement, InputProps>(function Input({ className, invalid, ...props }, ref) {
    return <input ref={ref} aria-invalid={invalid || undefined} className={cn(control, 'h-9', className)} {...props} />;
});

export interface TextareaProps extends TextareaHTMLAttributes<HTMLTextAreaElement> {
    invalid?: boolean;
}

export const Textarea = forwardRef<HTMLTextAreaElement, TextareaProps>(function Textarea({ className, invalid, ...props }, ref) {
    return (
        <textarea
            ref={ref}
            aria-invalid={invalid || undefined}
            className={cn(control, 'min-h-24 py-2 leading-relaxed', className)}
            {...props}
        />
    );
});

export interface SelectProps extends SelectHTMLAttributes<HTMLSelectElement> {
    invalid?: boolean;
}

export const Select = forwardRef<HTMLSelectElement, SelectProps>(function Select({ className, invalid, children, ...props }, ref) {
    return (
        <div className="relative">
            <select
                ref={ref}
                aria-invalid={invalid || undefined}
                className={cn(control, 'h-9 appearance-none pr-9', className)}
                {...props}
            >
                {children}
            </select>
            <ChevronDown size={16} className="pointer-events-none absolute right-3 top-1/2 -translate-y-1/2 text-muted" aria-hidden />
        </div>
    );
});

export interface FieldProps {
    label: ReactNode;
    /** Helper text shown under the control. */
    hint?: ReactNode;
    /** Error text; replaces the hint and marks the field invalid. */
    error?: ReactNode;
    /** Shown beside the label, e.g. "Optional" or a lock note. */
    aside?: ReactNode;
    className?: string;
    /** Receives the generated id and a11y props for the control. */
    children: (props: { id: string; 'aria-describedby'?: string; invalid?: boolean }) => ReactNode;
}

/**
 * A labelled form field. The label is always visible and tied to its control:
 *
 *   <Field label="Name" hint="Shown to players">
 *     {(field) => <Input {...field} value={name} onChange={...} />}
 *   </Field>
 */
export function Field({ label, hint, error, aside, className, children }: FieldProps) {
    const id = useId();
    const describedBy = error || hint ? `${id}-description` : undefined;

    return (
        <div className={cn('space-y-1.5', className)}>
            <div className="flex items-baseline justify-between gap-3">
                <label htmlFor={id} className="text-sm font-medium text-foreground">
                    {label}
                </label>
                {aside && <span className="text-xs text-subtle">{aside}</span>}
            </div>
            {children({ id, 'aria-describedby': describedBy, invalid: Boolean(error) || undefined })}
            {(error || hint) && (
                <p id={describedBy} role={error ? 'alert' : undefined} className={cn('text-xs', error ? 'text-danger' : 'text-muted')}>
                    {error || hint}
                </p>
            )}
        </div>
    );
}

export interface SwitchProps {
    checked: boolean;
    onChange: (checked: boolean) => void;
    label: ReactNode;
    description?: ReactNode;
    disabled?: boolean;
    className?: string;
    /** From a surrounding `Field`, so its label and help text belong to the switch as well. */
    id?: string;
    'aria-describedby'?: string;
}

/** An on/off setting with its label. The whole row is the click target. */
export function Switch({ checked, onChange, label, description, disabled, className, id: fieldId, 'aria-describedby': describedBy }: SwitchProps) {
    const ownId = useId();
    const id = fieldId ?? ownId;
    return (
        <label
            htmlFor={id}
            className={cn(
                'flex items-start justify-between gap-4 rounded-control py-1',
                disabled ? 'cursor-not-allowed opacity-60' : 'cursor-pointer',
                className,
            )}
        >
            <span className="min-w-0">
                <span className="block text-sm font-medium text-foreground">{label}</span>
                {description && <span className="mt-0.5 block text-xs text-muted">{description}</span>}
            </span>
            <span className="relative mt-0.5 inline-flex shrink-0">
                <input
                    id={id}
                    type="checkbox"
                    role="switch"
                    aria-describedby={describedBy}
                    className="peer sr-only"
                    checked={checked}
                    disabled={disabled}
                    onChange={(event) => onChange(event.target.checked)}
                />
                <span className="h-5 w-9 rounded-full bg-border-strong transition-colors peer-checked:bg-primary peer-focus-visible:outline peer-focus-visible:outline-2 peer-focus-visible:outline-offset-2 peer-focus-visible:outline-primary" />
                <span className="absolute left-0.5 top-0.5 h-4 w-4 rounded-full bg-white shadow-sm transition-transform peer-checked:translate-x-4" />
            </span>
        </label>
    );
}
