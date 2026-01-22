import { Outlet } from 'react-router-dom';

export default function AdminLayout() {
    return (
        <div className="p-8 max-w-7xl mx-auto animation-enter">
            <div className="mb-8">
                <h1 className="text-3xl font-bold tracking-tight mb-2">Platform Administration</h1>
                <p className="text-muted">Manage nodes, services, and system-wide user accounts.</p>
            </div>

            {/* Navigation is now handled by the primary sidebar */}
            <div className="h-px bg-border mb-8" />

            <Outlet />
        </div>
    );
}
