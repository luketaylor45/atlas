import { createContext, useContext } from 'react';

export interface User {
    id: number;
    username: string;
    is_admin: boolean;
}

export interface AuthContextValue {
    user: User | null;
    token: string | null;
    loading: boolean;
    login: (token: string, user: User, remember?: boolean) => void;
    logout: () => void;
}

export const AuthContext = createContext<AuthContextValue | undefined>(undefined);

export function useAuth() {
    const context = useContext(AuthContext);
    if (context === undefined) {
        throw new Error('useAuth must be used within an AuthProvider');
    }
    return context;
}
