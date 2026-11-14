import { cn } from '../lib/cn';

interface LogoProps {
    className?: string;
    size?: 'sm' | 'md' | 'lg';
}

const sizes = {
    sm: { mark: 'h-6 w-6', text: 'text-base' },
    md: { mark: 'h-7 w-7', text: 'text-lg' },
    lg: { mark: 'h-10 w-10', text: 'text-2xl' },
};

/** The Atlas mark and wordmark. Uses theme tokens, so it reads on any surface. */
export default function Logo({ className, size = 'md' }: LogoProps) {
    const style = sizes[size];
    return (
        <span className={cn('inline-flex select-none items-center gap-2.5', className)}>
            <svg viewBox="0 0 32 32" className={cn('shrink-0', style.mark)} aria-hidden>
                <rect width="32" height="32" rx="8" className="fill-primary" />
                <path d="M16 7 7.5 24h4.1l1.6-3.4h5.6l1.6 3.4h4.1L16 7Zm-1.3 10.4L16 14.6l1.3 2.8h-2.6Z" fill="#fff" />
            </svg>
            <span className={cn('font-semibold tracking-tight text-foreground', style.text)}>Atlas</span>
        </span>
    );
}
