import {
    Bell,
    Gauge,
    LayoutDashboard,
    Package,
    Server,
    ServerCog,
    Settings,
    Users,
    Waypoints,
    type LucideIcon,
} from 'lucide-react';

export interface NavItem {
    to: string;
    label: string;
    icon: LucideIcon;
    /** Match only the exact path (for index routes). */
    end?: boolean;
}

export const mainNav: NavItem[] = [
    { to: '/', label: 'Dashboard', icon: LayoutDashboard, end: true },
    { to: '/services', label: 'Servers', icon: Server },
    { to: '/settings', label: 'Settings', icon: Settings },
];

export const adminNav: NavItem[] = [
    { to: '/admin', label: 'Overview', icon: Gauge, end: true },
    { to: '/admin/nodes', label: 'Nodes', icon: Waypoints },
    { to: '/admin/services', label: 'All servers', icon: ServerCog },
    { to: '/admin/eggs', label: 'Eggs', icon: Package },
    { to: '/admin/users', label: 'Users', icon: Users },
    { to: '/admin/news', label: 'Announcements', icon: Bell },
];
