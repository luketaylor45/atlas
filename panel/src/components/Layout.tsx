import { Outlet, Link, useLocation } from 'react-router-dom';
import { Home, Settings, LogOut, LayoutDashboard, Users, Layers, Activity, Package, Bell } from 'lucide-react';
import clsx from 'clsx';
import { useAuth } from '../context/AuthContext';
import Logo from './Logo';

const SidebarItem = ({ icon: Icon, label, to }: { icon: any, label: string, to: string }) => {
    const isActive = useLocation().pathname === to;

    return (
        <Link to={to} className={clsx(
            "flex items-center gap-3 p-3 rounded-lg transition-all duration-200",
            isActive
                ? "bg-primary/10 text-primary font-medium"
                : "text-muted hover:text-foreground hover:bg-white/5"
        )}>
            <Icon size={18} />
            <span className="text-sm">{label}</span>
        </Link>
    );
};

export default function Layout() {
    const { user, logout } = useAuth();

    return (
        <div className="flex min-h-screen bg-background text-foreground font-sans dark transition-colors duration-300">
            {/* Sidebar - Glassmorphism */}
            <aside className="w-64 p-6 flex flex-col gap-6 fixed h-full glass border-r border-border z-10">
                <div className="pl-2 pt-2">
                    <Logo />
                </div>

                <nav className="flex flex-col gap-2 flex-1 mt-4 scrollbar-thin overflow-y-auto pr-2">
                    <div className="text-xs font-semibold text-muted uppercase tracking-wider pl-3 mb-2">Platform</div>
                    <SidebarItem icon={Home} label="Dashboard" to="/" />
                    <SidebarItem icon={Layers} label="My Services" to="/services" />

                    <div className="text-xs font-semibold text-muted uppercase tracking-wider pl-3 mb-2 mt-6">Configuration</div>
                    <SidebarItem icon={Settings} label="Settings" to="/settings" />

                    {user?.is_admin && (
                        <>
                            <div className="text-xs font-semibold text-muted uppercase tracking-wider pl-3 mb-2 mt-6">Administration</div>
                            <SidebarItem icon={LayoutDashboard} label="Overview" to="/admin" />
                            <SidebarItem icon={Activity} label="Nodes" to="/admin/nodes" />
                            <SidebarItem icon={Layers} label="Services" to="/admin/services" />
                            <SidebarItem icon={Package} label="Eggs & Nests" to="/admin/eggs" />
                            <SidebarItem icon={Users} label="Users" to="/admin/users" />
                            <SidebarItem icon={Bell} label="Announcements" to="/admin/news" />
                        </>
                    )}
                </nav>

                <div className="mt-auto border-t border-border pt-4">
                    <div className="flex items-center gap-3 p-2 mb-4 rounded-lg bg-secondary/50">
                        <div className="w-8 h-8 rounded-full bg-gradient-to-tr from-primary to-purple-500 flex items-center justify-center text-xs font-bold text-white uppercase">
                            {user?.username?.charAt(0) || 'U'}
                        </div>
                        <div className="flex flex-col overflow-hidden">
                            <span className="text-sm font-medium truncate" title={user?.username}>{user?.username || 'User Account'}</span>
                            <span className="text-xs text-muted">{user?.is_admin ? 'Administrator' : 'User'}</span>
                        </div>
                    </div>

                    <button
                        onClick={logout}
                        className="flex items-center gap-3 p-3 w-full text-red-400 hover:text-red-500 hover:bg-red-500/10 rounded-lg transition-colors text-sm"
                    >
                        <LogOut size={18} />
                        <span>Sign Out</span>
                    </button>
                </div>
            </aside>

            {/* Main Content */}
            <main className="flex-1 ml-64 p-8 relative">
                {/* Background ambient glow */}
                <div className="absolute top-0 left-0 w-full h-[500px] bg-gradient-to-b from-primary/5 to-transparent pointer-events-none -z-10" />

                <div className="max-w-7xl mx-auto">
                    <Outlet />
                </div>
            </main>
        </div>
    );
}
