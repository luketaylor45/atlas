import { useCallback, useMemo, useRef, useState, type ReactNode } from 'react';
import { createPortal } from 'react-dom';
import { CheckCircle2, Info, X, XCircle } from 'lucide-react';
import { cn } from '../../lib/cn';
import { Button } from './Button';
import { Input } from './Form';
import { Modal } from './Modal';
import { FeedbackContext, type ConfirmOptions, type ToastApi, type ToastTone } from './feedback';

interface ToastItem {
    id: number;
    tone: ToastTone;
    message: string;
}

const toastIcons: Record<ToastTone, ReactNode> = {
    success: <CheckCircle2 size={16} className="text-success" aria-hidden />,
    error: <XCircle size={16} className="text-danger" aria-hidden />,
    info: <Info size={16} className="text-info" aria-hidden />,
};

interface PendingConfirm {
    options: ConfirmOptions;
    resolve: (confirmed: boolean) => void;
}

export function FeedbackProvider({ children }: { children: ReactNode }) {
    const [toasts, setToasts] = useState<ToastItem[]>([]);
    const [pending, setPending] = useState<PendingConfirm | null>(null);
    const [typed, setTyped] = useState('');
    const nextId = useRef(1);

    const dismiss = useCallback((id: number) => {
        setToasts((current) => current.filter((toast) => toast.id !== id));
    }, []);

    const push = useCallback(
        (tone: ToastTone, message: string) => {
            const id = nextId.current++;
            // Keep the stack short; the newest messages matter most.
            setToasts((current) => [...current.slice(-3), { id, tone, message }]);
            // Errors stay longer: they usually need reading twice.
            setTimeout(() => dismiss(id), tone === 'error' ? 8000 : 4000);
        },
        [dismiss],
    );

    const toast = useMemo<ToastApi>(
        () => ({
            success: (message) => push('success', message),
            error: (message) => push('error', message),
            info: (message) => push('info', message),
        }),
        [push],
    );

    const confirm = useCallback((options: ConfirmOptions) => {
        setTyped('');
        return new Promise<boolean>((resolve) => {
            setPending({ options, resolve });
        });
    }, []);

    const settle = (confirmed: boolean) => {
        pending?.resolve(confirmed);
        setPending(null);
    };

    const value = useMemo(() => ({ toast, confirm }), [toast, confirm]);
    const options = pending?.options;
    const textMatches = !options?.requireText || typed.trim() === options.requireText;

    return (
        <FeedbackContext.Provider value={value}>
            {children}

            <Modal
                open={Boolean(pending)}
                onClose={() => settle(false)}
                title={options?.title ?? ''}
                description={options?.description}
                size="sm"
                footer={
                    <>
                        <Button variant="ghost" onClick={() => settle(false)}>
                            {options?.cancelLabel ?? 'Cancel'}
                        </Button>
                        <Button
                            variant={options?.danger ? 'danger' : 'primary'}
                            disabled={!textMatches}
                            onClick={() => settle(true)}
                            data-autofocus={options?.requireText ? undefined : true}
                        >
                            {options?.confirmLabel ?? 'Confirm'}
                        </Button>
                    </>
                }
            >
                {options?.requireText && (
                    <form
                        onSubmit={(event) => {
                            event.preventDefault();
                            if (textMatches) settle(true);
                        }}
                        className="space-y-2"
                    >
                        <label htmlFor="confirm-text" className="block text-sm text-muted">
                            Type <span className="font-mono font-medium text-foreground">{options.requireText}</span> to confirm.
                        </label>
                        <Input
                            id="confirm-text"
                            value={typed}
                            onChange={(event) => setTyped(event.target.value)}
                            autoComplete="off"
                            spellCheck={false}
                        />
                    </form>
                )}
            </Modal>

            {createPortal(
                <div
                    aria-live="polite"
                    className="pointer-events-none fixed inset-x-0 bottom-0 z-[60] flex flex-col items-center gap-2 p-4 sm:inset-x-auto sm:right-0 sm:items-end sm:p-6"
                >
                    {toasts.map((item) => (
                        <div
                            key={item.id}
                            role={item.tone === 'error' ? 'alert' : 'status'}
                            className={cn(
                                'pointer-events-auto flex w-full max-w-sm animate-slide-in items-start gap-3 rounded-card border border-border bg-popover p-3.5 text-sm text-foreground shadow-pop surface',
                            )}
                        >
                            <span className="mt-0.5 shrink-0">{toastIcons[item.tone]}</span>
                            <p className="min-w-0 flex-1 break-words">{item.message}</p>
                            <button
                                type="button"
                                onClick={() => dismiss(item.id)}
                                aria-label="Dismiss"
                                className="-mr-1 -mt-0.5 shrink-0 rounded-control p-1 text-subtle transition-colors hover:bg-secondary hover:text-foreground"
                            >
                                <X size={14} aria-hidden />
                            </button>
                        </div>
                    ))}
                </div>,
                document.body,
            )}
        </FeedbackContext.Provider>
    );
}
