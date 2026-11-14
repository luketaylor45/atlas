import { formatDistanceToNow } from 'date-fns';

/** 1536 -> "1.5 GB", 512 -> "512 MB". Input is megabytes. */
export function formatMB(megabytes: number | null | undefined): string {
    const value = Number(megabytes) || 0;
    if (value >= 1024) {
        const gigabytes = value / 1024;
        return `${Number.isInteger(gigabytes) ? gigabytes : gigabytes.toFixed(1)} GB`;
    }
    return `${Math.round(value)} MB`;
}

/** Bytes to a short human size. */
export function formatBytes(bytes: number | null | undefined): string {
    let value = Number(bytes) || 0;
    const units = ['B', 'KB', 'MB', 'GB', 'TB'];
    let unit = 0;
    while (value >= 1024 && unit < units.length - 1) {
        value /= 1024;
        unit += 1;
    }
    return `${unit === 0 || value >= 100 ? Math.round(value) : value.toFixed(1)} ${units[unit]}`;
}

/** CPU limit as cores: 100 -> "1 core", 250 -> "2.5 cores". */
export function formatCpu(percent: number | null | undefined): string {
    const cores = (Number(percent) || 0) / 100;
    const label = Number.isInteger(cores) ? String(cores) : cores.toFixed(1);
    return `${label} ${cores === 1 ? 'core' : 'cores'}`;
}

/** "3 minutes ago". Returns a dash for missing or invalid dates. */
export function formatRelative(date: string | Date | null | undefined): string {
    if (!date) return '—';
    const parsed = new Date(date);
    if (Number.isNaN(parsed.getTime()) || parsed.getFullYear() < 2000) return '—';
    return formatDistanceToNow(parsed, { addSuffix: true });
}

/** A full local date and time, for tooltips and logs. */
export function formatDateTime(date: string | Date | null | undefined): string {
    if (!date) return '—';
    const parsed = new Date(date);
    if (Number.isNaN(parsed.getTime()) || parsed.getFullYear() < 2000) return '—';
    return parsed.toLocaleString([], { dateStyle: 'medium', timeStyle: 'short' });
}

/** Parses a whole number from an input, or returns the fallback when empty or invalid. */
export function parseInteger(value: string, fallback = 0): number {
    const parsed = Number.parseInt(value, 10);
    return Number.isFinite(parsed) ? parsed : fallback;
}
