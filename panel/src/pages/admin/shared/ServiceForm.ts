import { parseInteger } from '../../../lib/format';

/** A whole, non-negative number typed into an input, or null when it is anything else. */
export function wholeNumber(value: string): number | null {
    const trimmed = value.trim();
    if (!/^\d+$/.test(trimmed)) return null;
    const parsed = parseInteger(trimmed, -1);
    return parsed >= 0 && Number.isSafeInteger(parsed) ? parsed : null;
}

/** A valid port number (1-65535), or null. */
export function portNumber(value: string): number | null {
    const port = wholeNumber(value);
    return port !== null && port >= 1 && port <= 65535 ? port : null;
}

/** The port number an API error such as "port 25565 is already assigned…" talks about. */
export function portInMessage(message: string): number | null {
    const match = /port (\d+)/i.exec(message);
    return match ? parseInteger(match[1], 0) || null : null;
}

/** Classes that make a `<Link>` look like the kit's primary button. */
export const primaryLinkClass =
    'inline-flex h-9 items-center justify-center gap-2 whitespace-nowrap rounded-control bg-primary px-4 text-sm font-medium ' +
    'text-primary-foreground shadow-card transition-colors hover:bg-primary-hover';

/** Classes that make a `<Link>` look like the kit's small ghost icon button. */
export const iconLinkClass =
    'inline-flex h-8 w-8 items-center justify-center rounded-control text-muted transition-colors hover:bg-secondary hover:text-foreground';
