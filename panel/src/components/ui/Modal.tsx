import { useEffect, useId, useRef, type ReactNode } from 'react';
import { createPortal } from 'react-dom';
import { X } from 'lucide-react';
import { cn } from '../../lib/cn';
import { IconButton } from './Button';

const FOCUSABLE =
    'a[href], button:not([disabled]), textarea:not([disabled]), input:not([disabled]), select:not([disabled]), [tabindex]:not([tabindex="-1"])';

const widths = {
    sm: 'max-w-md',
    md: 'max-w-lg',
    lg: 'max-w-2xl',
    xl: 'max-w-4xl',
};

export interface ModalProps {
    open: boolean;
    onClose: () => void;
    title: ReactNode;
    description?: ReactNode;
    children?: ReactNode;
    /** Buttons shown in the footer, right-aligned. */
    footer?: ReactNode;
    size?: keyof typeof widths;
    /** Set false while an action is running so the dialog cannot be dismissed. */
    dismissible?: boolean;
}

/**
 * An accessible dialog: closes on Escape and backdrop click, keeps keyboard
 * focus inside while open, and returns focus to where it was on close.
 */
export function Modal({ open, onClose, title, description, children, footer, size = 'md', dismissible = true }: ModalProps) {
    const titleId = useId();
    const descriptionId = useId();
    const panel = useRef<HTMLDivElement>(null);
    const onCloseRef = useRef(onClose);
    useEffect(() => {
        onCloseRef.current = onClose;
    });

    useEffect(() => {
        if (!open) return;

        const previouslyFocused = document.activeElement as HTMLElement | null;
        const node = panel.current;

        // Focus the first field if there is one, otherwise the dialog itself.
        const first = node?.querySelector<HTMLElement>('[data-autofocus], input, textarea, select');
        (first ?? node)?.focus();

        const previousOverflow = document.body.style.overflow;
        document.body.style.overflow = 'hidden';

        const onKeyDown = (event: KeyboardEvent) => {
            if (event.key === 'Escape' && dismissible) {
                event.stopPropagation();
                onCloseRef.current();
                return;
            }
            if (event.key !== 'Tab' || !node) return;

            const focusable = Array.from(node.querySelectorAll<HTMLElement>(FOCUSABLE)).filter((el) => el.offsetParent !== null);
            if (focusable.length === 0) {
                event.preventDefault();
                return;
            }
            const firstEl = focusable[0];
            const lastEl = focusable[focusable.length - 1];
            if (event.shiftKey && document.activeElement === firstEl) {
                event.preventDefault();
                lastEl.focus();
            } else if (!event.shiftKey && document.activeElement === lastEl) {
                event.preventDefault();
                firstEl.focus();
            }
        };

        document.addEventListener('keydown', onKeyDown);
        return () => {
            document.removeEventListener('keydown', onKeyDown);
            document.body.style.overflow = previousOverflow;
            previouslyFocused?.focus?.();
        };
    }, [open, dismissible]);

    if (!open) return null;

    return createPortal(
        <div className="fixed inset-0 z-50 flex items-end justify-center p-0 sm:items-center sm:p-6">
            <div
                className="absolute inset-0 animate-fade-in bg-black/50 backdrop-blur-[2px]"
                onClick={dismissible ? onClose : undefined}
                aria-hidden
            />
            <div
                ref={panel}
                role="dialog"
                aria-modal="true"
                aria-labelledby={titleId}
                aria-describedby={description ? descriptionId : undefined}
                tabIndex={-1}
                className={cn(
                    'relative flex max-h-[92dvh] w-full animate-pop-in flex-col rounded-t-card border border-border bg-popover shadow-pop surface outline-none sm:rounded-card',
                    widths[size],
                )}
            >
                <div className="flex items-start justify-between gap-4 px-5 pb-1 pt-5 sm:px-6">
                    <div className="min-w-0">
                        <h2 id={titleId} className="text-base font-semibold tracking-tight text-foreground">
                            {title}
                        </h2>
                        {description && (
                            <p id={descriptionId} className="mt-1 text-sm text-muted">
                                {description}
                            </p>
                        )}
                    </div>
                    {dismissible && (
                        <IconButton label="Close" size="sm" onClick={onClose} className="-mr-1.5 -mt-1">
                            <X size={16} aria-hidden />
                        </IconButton>
                    )}
                </div>

                {children && <div className="overflow-y-auto px-5 py-4 sm:px-6">{children}</div>}

                {footer && (
                    <div className="flex flex-col-reverse gap-2 border-t border-border px-5 py-4 sm:flex-row sm:justify-end sm:px-6">
                        {footer}
                    </div>
                )}
            </div>
        </div>,
        document.body,
    );
}
