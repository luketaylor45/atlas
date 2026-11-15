import { Link } from 'react-router-dom';
import { useAuth } from '../../context/auth';
import Logo from '../Logo';
import { MobileBar, MobileMenu } from './MobileMenu';
import { adminNav, mainNav } from './nav';
import { SideNavSection } from './parts';
import UserMenu from './UserMenu';

/** Glass: a frosted sidebar floating over the colour wash, with pill links. */
export default function GlassNav() {
    const { user } = useAuth();

    return (
        <>
            <aside className="surface fixed bottom-3 left-3 top-3 z-30 hidden w-60 flex-col rounded-[22px] border border-border bg-card shadow-card lg:flex">
                <div className="flex h-16 shrink-0 items-center px-5">
                    <Link to="/" aria-label="Atlas home">
                        <Logo />
                    </Link>
                </div>

                <nav aria-label="Main" className="flex-1 space-y-7 overflow-y-auto px-3 py-2">
                    <SideNavSection items={mainNav} variant="glass" />
                    {user?.is_admin && <SideNavSection title="Administration" items={adminNav} variant="glass" />}
                </nav>

                <div className="shrink-0 p-3">
                    <div className="rounded-[14px] bg-foreground/[0.04] p-1">
                        <UserMenu placement="up" />
                    </div>
                </div>
            </aside>

            <MobileBar />
            <MobileMenu side="floating" variant="glass" />
        </>
    );
}
