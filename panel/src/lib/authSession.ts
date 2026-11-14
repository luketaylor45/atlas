export const AUTH_SESSION_EXPIRED_EVENT = 'atlas:auth-session-expired';

const TOKEN_KEY = 'token';
const USER_KEY = 'user';

export interface StoredAuth {
    token: string;
    user: string;
}

function readFromStorage(storage: Storage): StoredAuth | null {
    const token = storage.getItem(TOKEN_KEY);
    const user = storage.getItem(USER_KEY);
    return token && user ? { token, user } : null;
}

export function getStoredAuth(): StoredAuth | null {
    return readFromStorage(localStorage) ?? readFromStorage(sessionStorage);
}

export function getStoredToken(): string | null {
    return getStoredAuth()?.token ?? null;
}

export function storeAuth(token: string, user: string, remember: boolean) {
    clearStoredAuth();
    const storage = remember ? localStorage : sessionStorage;
    storage.setItem(TOKEN_KEY, token);
    storage.setItem(USER_KEY, user);
}

export function clearStoredAuth() {
    localStorage.removeItem(TOKEN_KEY);
    localStorage.removeItem(USER_KEY);
    sessionStorage.removeItem(TOKEN_KEY);
    sessionStorage.removeItem(USER_KEY);
}

export function expireAuthSession() {
    clearStoredAuth();
    window.dispatchEvent(new Event(AUTH_SESSION_EXPIRED_EVENT));
}

/** Replaces the saved user, keeping the token where it already lives. */
export function updateStoredUser(user: string) {
    for (const storage of [localStorage, sessionStorage]) {
        if (storage.getItem(TOKEN_KEY)) {
            storage.setItem(USER_KEY, user);
        }
    }
}
