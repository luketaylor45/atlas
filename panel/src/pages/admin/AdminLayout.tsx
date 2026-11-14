import { Navigate, Outlet } from 'react-router-dom';
import { useAuth } from '../../context/auth';

/**
 * Guards every /admin route. The API enforces this too; the guard keeps
 * non-admins from landing on pages that would only show errors.
 */
export default function AdminLayout() {
    const { user } = useAuth();

    if (!user?.is_admin) {
        return <Navigate to="/" replace />;
    }

    return <Outlet />;
}
