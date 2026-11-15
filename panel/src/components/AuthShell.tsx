import { useState, type ReactNode } from 'react';
import { Eye, EyeOff } from 'lucide-react';
import { cn } from '../lib/cn';
import Logo from './Logo';
import OrbitArt from './OrbitArt';
import ThemeToggle from './ThemeToggle';
import { Card, IconButton, Input, type InputProps } from './ui';

interface AuthShellProps {
    title: ReactNode;
    description?: ReactNode;
    children: ReactNode;
    /** Shown under the card, e.g. a link to the other sign-in page. */
    footer?: ReactNode;
}

/** The layout shared by the signed-out pages: sign in, register and setup. */
export default function AuthShell({ title, description, children, footer }: AuthShellProps) {
    return (
        <div className="relative isolate flex min-h-dvh bg-background text-foreground">
            <div className="absolute right-4 top-4 z-10">
                <ThemeToggle />
            </div>

            {/* The brand panel, on wide screens. Always dark, like the mark's night sky. */}
            <div className="auth-brand relative hidden w-[44%] max-w-[44rem] flex-col justify-between overflow-hidden p-12 text-white lg:flex">
                <OrbitArt className="absolute -right-32 -top-20 h-[38rem] w-[38rem] text-[#a5b0ff] opacity-80" />
                <Logo size="lg" tone="inverse" className="relative" />
                <div className="relative max-w-sm">
                    <p className="text-[2rem] font-bold leading-tight tracking-[-0.03em]">Your game servers, held steady.</p>
                    <p className="mt-4 text-base leading-relaxed text-[#c9cdfd]">
                        Start, stop and watch every server, manage files and backups, and share access with friends, all from one place.
                    </p>
                    <p className="mt-10 text-sm text-[#a5b0ff]">Self-hosted game server panel</p>
                </div>
            </div>

            <div className="relative flex min-w-0 flex-1 flex-col items-center justify-center px-4 py-20">
                <div aria-hidden className="pointer-events-none absolute inset-0 -z-10 overflow-hidden">
                    <div className="auth-glow absolute inset-0" />
                    <OrbitArt className="absolute left-1/2 top-[-9rem] h-[40rem] w-[40rem] -translate-x-1/2 text-primary-text opacity-30 lg:hidden" />
                </div>

                <main className="w-full max-w-[400px]">
                    <div className="mb-8 flex justify-center lg:hidden">
                        <Logo size="lg" />
                    </div>

                    <Card className="shadow-pop">
                        <h1 className="text-xl font-semibold tracking-tight text-foreground">{title}</h1>
                        {description && <p className="mt-1.5 text-sm text-muted">{description}</p>}
                        <div className="mt-6">{children}</div>
                    </Card>

                    {footer && <p className="mt-5 text-center text-sm text-muted">{footer}</p>}
                </main>
            </div>
        </div>
    );
}

/** A password input with a show/hide toggle. */
export function PasswordInput({ className, ...props }: Omit<InputProps, 'type'>) {
    const [visible, setVisible] = useState(false);

    return (
        <div className="relative">
            <Input {...props} type={visible ? 'text' : 'password'} className={cn('pr-10', className)} />
            <IconButton
                label={visible ? 'Hide password' : 'Show password'}
                aria-pressed={visible}
                size="sm"
                onClick={() => setVisible((current) => !current)}
                className="absolute right-0.5 top-0.5"
            >
                {visible ? <EyeOff size={16} aria-hidden /> : <Eye size={16} aria-hidden />}
            </IconButton>
        </div>
    );
}
