import { useState, useEffect, type ReactNode } from 'react';
import {
    AUTH_SESSION_EXPIRED_EVENT,
    clearStoredAuth,
    getStoredAuth,
    storeAuth,
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

function readStoredAuth(): AuthState {
    const storedAuth = getStoredAuth();
    if (!storedAuth) {
        clearStoredAuth();
        return { user: null, token: null };
    }

    try {
        const expiry = getTokenExpiry(storedAuth.token);
        if (expiry !== null && expiry <= Date.now()) {
            clearStoredAuth();
            return { user: null, token: null };
        }
        return { user: JSON.parse(storedAuth.user), token: storedAuth.token };
    } catch {
        clearStoredAuth();
        return { user: null, token: null };
    }
}

export function AuthProvider({ children }: { children: ReactNode }) {
    const [auth, setAuth] = useState<AuthState>(readStoredAuth);
    const { user, token } = auth;

    useEffect(() => {
        const clearAuthState = () => {
            setAuth({ user: null, token: null });
        };
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
            setAuth({ user: null, token: null });
        };
        const remaining = expiry - Date.now();
        const timer = window.setTimeout(expire, Math.max(remaining, 0));
        return () => window.clearTimeout(timer);
    }, [token]);

    const login = (newToken: string, newUser: User, remember = true) => {
        storeAuth(newToken, JSON.stringify(newUser), remember);
        setAuth({ token: newToken, user: newUser });
    };

    const logout = () => {
        clearStoredAuth();
        setAuth({ user: null, token: null });
    };

    return (
        <AuthContext.Provider value={{ user, token, loading: false, login, logout }}>
            {children}
        </AuthContext.Provider>
    );
}
