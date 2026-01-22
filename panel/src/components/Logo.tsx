import clsx from 'clsx';

interface LogoProps {
    className?: string;
    size?: "sm" | "md" | "lg" | "xl";
}

export default function Logo({ className, size = "md" }: LogoProps) {
    const sizeClasses = {
        sm: "text-lg",
        md: "text-2xl",
        lg: "text-3xl",
        xl: "text-5xl"
    };

    return (
        <div className={clsx("font-black tracking-tighter flex items-baseline select-none opacity-90 hover:opacity-100 transition-opacity", className)}>
            <span className={clsx(
                "bg-gradient-to-br from-white via-zinc-200 to-zinc-500 bg-clip-text text-transparent",
                sizeClasses[size]
            )}>
                ATLAS
            </span>
            <div className={clsx(
                "rounded-full bg-primary shadow-[0_0_15px_var(--color-primary)] animate-pulse",
                size === "sm" ? "w-1.5 h-1.5 ml-0.5" :
                    size === "md" ? "w-2 h-2 ml-1" :
                        size === "lg" ? "w-2.5 h-2.5 ml-1.5" :
                            "w-4 h-4 ml-2"
            )} />
        </div>
    );
}
