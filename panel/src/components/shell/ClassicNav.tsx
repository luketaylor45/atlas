import { Link } from 'react-router-dom';
import { useAuth } from '../../context/auth';
import Logo from '../Logo';
import { MobileBar, MobileMenu } from './MobileMenu';
import { adminNav, mainNav } from './nav';
import { SideNavSection } from './parts';
import UserMenu from './UserMenu';

/** Classic: a labelled sidebar fixed to the left edge. */
export default function ClassicNav() {
    const { user } = useAuth();

    return (
        <>
            <aside className="fixed inset-y-0 left-0 z-30 hidden w-64 flex-col border-r border-border bg-card lg:flex">
                <div className="flex h-16 shrink-0 items-center px-6">
                    <Link to="/" aria-label="Atlas home">
                        <Logo />
                    </Link>
                </div>

                <nav aria-label="Main" className="flex-1 space-y-7 overflow-y-auto px-3 py-3">
                    <SideNavSection items={mainNav} />
                    {user?.is_admin && <SideNavSection title="Administration" items={adminNav} />}
                </nav>

                <div className="shrink-0 border-t border-border p-3">
                    <UserMenu placement="up" />
                </div>
            </aside>

            <MobileBar />
            <MobileMenu side="left" />
        </>
    );
}
