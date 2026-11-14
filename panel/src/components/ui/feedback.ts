import { createContext, useContext, type ReactNode } from 'react';

export type ToastTone = 'success' | 'error' | 'info';

export interface ToastApi {
    success: (message: string) => void;
    error: (message: string) => void;
    info: (message: string) => void;
}

export interface ConfirmOptions {
    title: string;
    /** What will happen, in plain words. Say what is lost if it is destructive. */
    description?: ReactNode;
    /** Label of the confirming button. Name the action: "Delete server", not "OK". */
    confirmLabel?: string;
    cancelLabel?: string;
    /** Styles the confirming button as destructive. */
    danger?: boolean;
    /**
     * For irreversible actions: the user must type this text to confirm.
     */
    requireText?: string;
}

export interface FeedbackContextValue {
    toast: ToastApi;
    /** Resolves true if the user confirmed, false if they cancelled. */
    confirm: (options: ConfirmOptions) => Promise<boolean>;
}

export const FeedbackContext = createContext<FeedbackContextValue | undefined>(undefined);

function useFeedback() {
    const context = useContext(FeedbackContext);
    if (!context) {
        throw new Error('useToast and useConfirm must be used within a FeedbackProvider');
    }
    return context;
}

/**
 * Transient messages, used instead of alert():
 *
 *   const toast = useToast();
 *   toast.success('Server created');
 *   toast.error(errorMessage(err, 'Could not create the server'));
 */
export function useToast(): ToastApi {
    return useFeedback().toast;
}

/**
 * A confirmation dialog, used instead of confirm():
 *
 *   const confirm = useConfirm();
 *   if (!(await confirm({ title: 'Delete server?', description: '…', confirmLabel: 'Delete server', danger: true }))) return;
 */
export function useConfirm() {
    return useFeedback().confirm;
}
