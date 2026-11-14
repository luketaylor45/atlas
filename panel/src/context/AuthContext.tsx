import { useCallback, useEffect, useState, type ReactNode } from 'react';
import api from '../lib/api';
import {
    AUTH_SESSION_EXPIRED_EVENT,
    clearStoredAuth,
    getStoredAuth,
    storeAuth,
    updateStoredUser,
} from '../lib/authSession';
import { AuthContext, type User } from './auth';

function getTokenExpiry(token: string): number | null {
    try {
        const encodedPayload = token.split('.')[1];
        if (!encodedPayload) return null;

        const normalized = encodedPayload
            .replace(/-/g, '+')
            .replace(/_/g, '/')
            .padEnd(Math.ceil(encodedPayload.length / 4) * 4, '=');
        const payload = JSON.parse(atob(normalized));
        return typeof payload.exp === 'number' ? payload.exp * 1000 : null;
    } catch {
        return null;
    }
}

interface AuthState {
    user: User | null;
    token: string | null;
}

const signedOut: AuthState = { user: null, token: null };

function readStoredAuth(): AuthState {
    const storedAuth = getStoredAuth();
    if (!storedAuth) {
        clearStoredAuth();
        return signedOut;
    }

    try {
        const expiry = getTokenExpiry(storedAuth.token);
        if (expiry !== null && expiry <= Date.now()) {
            clearStoredAuth();
            return signedOut;
        }
        return { user: JSON.parse(storedAuth.user), token: storedAuth.token };
    } catch {
        clearStoredAuth();
        return signedOut;
    }
}

export function AuthProvider({ children }: { children: ReactNode }) {
    const [auth, setAuth] = useState<AuthState>(readStoredAuth);
    // A stored session is only trusted once the server has confirmed it. A
    // token handed over by a fresh login is already confirmed.
    const [verifiedToken, setVerifiedToken] = useState<string | null>(null);
    const { user, token } = auth;
    const verifying = token !== null && verifiedToken !== token;

    // Confirm the stored session with the server. What was saved at login can
    // be out of date: the account may have been deleted, renamed, or had its
    // admin role changed since.
    useEffect(() => {
        if (!token || verifiedToken === token) return;

        let cancelled = false;
        api.get<{ user: User }>('/auth/me')
            .then((res) => {
                if (cancelled) return;
                setAuth((current) => (current.token === token ? { token, user: res.data.user } : current));
                updateStoredUser(JSON.stringify(res.data.user));
            })
            .catch(() => {
                // A 401 already cleared the session through the API layer.
                // Any other failure (server briefly unreachable) keeps the
                // stored session so a blip does not sign everyone out.
            })
            .finally(() => {
                if (!cancelled) setVerifiedToken(token);
            });

        return () => {
            cancelled = true;
        };
    }, [token, verifiedToken]);

    useEffect(() => {
        const clearAuthState = () => setAuth(signedOut);
        const handleStorage = (event: StorageEvent) => {
            if ((event.key === 'token' || event.key === 'user') && event.newValue === null) {
                if (!getStoredAuth()) {
                    clearAuthState();
                }
            }
        };

        window.addEventListener(AUTH_SESSION_EXPIRED_EVENT, clearAuthState);
        window.addEventListener('storage', handleStorage);
        return () => {
            window.removeEventListener(AUTH_SESSION_EXPIRED_EVENT, clearAuthState);
            window.removeEventListener('storage', handleStorage);
        };
    }, []);

    useEffect(() => {
        if (!token) return;

        const expiry = getTokenExpiry(token);
        if (expiry === null) return;

        const expire = () => {
            clearStoredAuth();
            setAuth(signedOut);
        };
        const remaining = expiry - Date.now();
        // setTimeout overflows above ~24.8 days; sessions are far shorter.
        const timer = window.setTimeout(expire, Math.max(remaining, 0));
        return () => window.clearTimeout(timer);
    }, [token]);

    const login = useCallback((newToken: string, newUser: User, remember = true) => {
        storeAuth(newToken, JSON.stringify(newUser), remember);
        setVerifiedToken(newToken);
        setAuth({ token: newToken, user: newUser });
    }, []);

    const logout = useCallback(() => {
        clearStoredAuth();
        setAuth(signedOut);
    }, []);

    return (
        <AuthContext.Provider value={{ user, token, loading: verifying, login, logout }}>
            {children}
        </AuthContext.Provider>
    );
}
