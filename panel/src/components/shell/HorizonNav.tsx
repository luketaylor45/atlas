import { Link, NavLink, useLocation } from 'react-router-dom';
import { useAuth } from '../../context/auth';
import { cn } from '../../lib/cn';
import Logo from '../Logo';
import { MenuButton, MobileMenu } from './MobileMenu';
import { adminNav, mainNav, type NavItem } from './nav';
import UserMenu from './UserMenu';

const topItems = (isAdmin: boolean): NavItem[] =>
    isAdmin ? [...mainNav, { to: '/admin', label: 'Administration', icon: adminNav[0].icon }] : mainNav;

/**
 * Horizon: one bar across the top with the primary links and the account
 * menu. Inside Administration a second row lists its pages.
 */
export default function HorizonNav() {
    const { user } = useAuth();
    const location = useLocation();
    const isAdmin = Boolean(user?.is_admin);
    const inAdmin = isAdmin && (location.pathname === '/admin' || location.pathname.startsWith('/admin/'));

    return (
        <>
            <header className="sticky top-0 z-30 border-b border-border bg-card/80 backdrop-blur-xl">
                <div className="mx-auto flex h-16 w-full max-w-[var(--shell-max)] items-center gap-4 px-4 sm:px-6 lg:px-8">
                    <Link to="/" aria-label="Atlas home" className="shrink-0">
                        <Logo />
                    </Link>

                    <nav aria-label="Main" className="ml-6 hidden lg:block">
                        <ul className="flex items-center gap-1">
                            {topItems(isAdmin).map(({ to, label, end }) => (
                                <li key={to}>
                                    <NavLink
                                        to={to}
                                        end={end}
                                        className={({ isActive }) =>
                                            cn(
                                                'inline-flex h-9 items-center rounded-full px-4 text-sm font-medium transition-colors',
                                                isActive
                                                    ? 'bg-primary/10 text-primary-text'
                                                    : 'text-muted hover:bg-secondary hover:text-foreground',
                                            )
                                        }
                                    >
                                        {label}
                                    </NavLink>
                                </li>
                            ))}
                        </ul>
                    </nav>

                    <div className="ml-auto flex items-center gap-2">
                        <UserMenu placement="down" trigger="avatar" className="hidden lg:block" />
                        <MenuButton />
                    </div>
                </div>

                {inAdmin && (
                    <nav aria-label="Administration" className="border-t border-border">
                        <ul className="mx-auto flex h-11 w-full max-w-[var(--shell-max)] items-stretch gap-1 overflow-x-auto px-4 [scrollbar-width:none] sm:px-6 lg:px-8">
                            {adminNav.map(({ to, label, icon: Icon, end }) => (
                                <li key={to} className="flex">
                                    <NavLink
                                        to={to}
                                        end={end}
                                        className={({ isActive }) =>
                                            cn(
                                                'inline-flex shrink-0 items-center gap-2 whitespace-nowrap border-b-2 px-3 text-sm transition-colors',
                                                isActive
                                                    ? 'border-primary font-medium text-foreground'
                                                    : 'border-transparent text-muted hover:text-foreground',
                                            )
                                        }
                                    >
                                        <Icon size={15} aria-hidden />
                                        {label}
                                    </NavLink>
                                </li>
                            ))}
                        </ul>
                    </nav>
                )}
            </header>

            <MobileMenu side="top" />
        </>
    );
}
