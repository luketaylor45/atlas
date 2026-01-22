import React from 'react';
import { Navigate, useLocation } from 'react-router-dom';
import { useAuth } from '../context/AuthContext';

export default function RequireAuth({ children }: { children: React.ReactNode }) {
    const { user, loading } = useAuth();
    const location = useLocation();

    if (loading) {
        return <div className="flex h-screen items-center justify-center bg-background text-muted">Loading...</div>;
    }

    if (!user) {
        return <Navigate to="/login" state={{ from: location }} replace />;
    }

    return children;
}
