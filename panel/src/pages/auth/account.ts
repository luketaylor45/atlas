import { useEffect, useState } from 'react';
import api from '../../lib/api';

/** Response of GET /auth/setup-status. */
export interface SetupStatus {
    setup_required: boolean;
    registration_enabled: boolean;
}

/** Response of POST /auth/login, /auth/register and /auth/setup. */
export interface AuthResponse {
    token: string;
    user: { id: number; username: string; is_admin: boolean };
}

/**
 * Loads the panel's sign-in configuration once. `status` stays null when the
 * request fails, so pages can fall back to showing their form.
 */
export function useSetupStatus(): { status: SetupStatus | null; loading: boolean } {
    const [status, setStatus] = useState<SetupStatus | null>(null);
    const [loading, setLoading] = useState(true);

    useEffect(() => {
        let cancelled = false;
        api.get<SetupStatus>('/auth/setup-status')
            .then((res) => {
                if (!cancelled) setStatus(res.data);
            })
            .catch(() => {
                // The forms still work without this; the API enforces the rules.
            })
            .finally(() => {
                if (!cancelled) setLoading(false);
            });
        return () => {
            cancelled = true;
        };
    }, []);

    return { status, loading };
}

export const USERNAME_HINT = '3 to 32 characters: letters, numbers, dots, dashes and underscores.';
export const PASSWORD_HINT = '8 to 72 characters.';

/** Mirrors the API's username rule. Returns the problem, or undefined when valid. */
export function validateUsername(username: string): string | undefined {
    if (!username) return 'Enter a username.';
    if (username.length < 3 || username.length > 32) return 'Use between 3 and 32 characters.';
    if (!/^[A-Za-z0-9._-]+$/.test(username)) return 'Use only letters, numbers, dots, dashes and underscores.';
    return undefined;
}

/** Mirrors the API's password rule. Returns the problem, or undefined when valid. */
export function validatePassword(password: string): string | undefined {
    if (!password) return 'Enter a password.';
    if (password.length < 8) return 'Use at least 8 characters.';
    if (password.length > 72) return 'Use 72 characters or fewer.';
    return undefined;
}

export function validateConfirmation(password: string, confirmation: string): string | undefined {
    if (!confirmation) return 'Enter the password again.';
    if (password !== confirmation) return 'The passwords do not match.';
    return undefined;
}
