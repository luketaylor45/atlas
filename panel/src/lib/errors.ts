import { isAxiosError } from 'axios';

/**
 * Turns anything thrown by an API call into a message fit to show a user.
 * The API always answers failures with `{ error: string }`.
 */
export function errorMessage(error: unknown, fallback = 'Something went wrong. Please try again.'): string {
    if (isAxiosError(error)) {
        const data = error.response?.data as { error?: unknown } | undefined;
        if (typeof data?.error === 'string' && data.error.trim()) {
            return data.error.trim();
        }
        if (!error.response) {
            return 'Could not reach the server. Check your connection and try again.';
        }
        if (error.response.status === 429) {
            return 'Too many attempts. Please wait a few minutes and try again.';
        }
    }
    return fallback;
}

/** The HTTP status of a failed API call, if there was a response. */
export function errorStatus(error: unknown): number | undefined {
    return isAxiosError(error) ? error.response?.status : undefined;
}
