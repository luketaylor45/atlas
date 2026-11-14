import { useState, type ReactNode } from 'react';
import { Eye, EyeOff } from 'lucide-react';
import { cn } from '../lib/cn';
import Logo from './Logo';
import ThemeToggle from './ThemeToggle';
import { Card, IconButton, Input, type InputProps } from './ui';

interface AuthShellProps {
    title: ReactNode;
    description?: ReactNode;
    children: ReactNode;
    /** Shown under the card, e.g. a link to the other sign-in page. */
    footer?: ReactNode;
}

/** The centred layout shared by the signed-out pages: sign in, register and setup. */
export default function AuthShell({ title, description, children, footer }: AuthShellProps) {
    return (
        <div className="relative flex min-h-dvh flex-col items-center justify-center bg-background px-4 py-20 text-foreground">
            <div className="absolute right-4 top-4">
                <ThemeToggle />
            </div>

            <main className="w-full max-w-[400px]">
                <div className="mb-6 flex justify-center">
                    <Logo size="lg" />
                </div>

                <Card>
                    <h1 className="text-lg font-semibold tracking-tight text-foreground">{title}</h1>
                    {description && <p className="mt-1 text-sm text-muted">{description}</p>}
                    <div className="mt-6">{children}</div>
                </Card>

                {footer && <p className="mt-5 text-center text-sm text-muted">{footer}</p>}
            </main>
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
