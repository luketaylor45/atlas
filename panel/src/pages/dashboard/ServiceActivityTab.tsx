import { useEffect, useRef, useState } from 'react';
import { ChevronLeft, ChevronRight, Download, History } from 'lucide-react';
import api from '../../lib/api';
import { cn } from '../../lib/cn';
import { errorMessage } from '../../lib/errors';
import { formatDateTime, formatRelative } from '../../lib/format';
import { parseJSON, type ActivityLog } from '../../lib/types';
import { usePolling } from '../../lib/usePolling';
import {
    Alert,
    Badge,
    Button,
    Card,
    CardHeader,
    EmptyState,
    Field,
    LoadingState,
    Select,
    Table,
    Td,
    Th,
    Tr,
    useToast,
} from '../../components/ui';

interface ActivityPage {
    logs: ActivityLog[] | null;
    total: number;
    limit: number;
    offset: number;
}

const PAGE_SIZE = 25;
const EXPORT_PAGE_SIZE = 200;
const EXPORT_MAX_ROWS = 10000;

const actionLabels: Record<string, string> = {
    power: 'Power',
    sftp_login: 'SFTP sign-in',
    file_extract: 'Archive extracted',
    reinstall: 'Reinstall',
    install_cancel: 'Install cancelled',
    rebuild: 'Container rebuilt',
    user_added: 'Sub-user added',
    user_updated: 'Sub-user changed',
    user_removed: 'Sub-user removed',
    create: 'Server created',
    update: 'Server changed',
};

function actionLabel(action: string): string {
    if (actionLabels[action]) return actionLabels[action];
    const words = action.replace(/_/g, ' ').trim();
    return words ? words.charAt(0).toUpperCase() + words.slice(1) : 'Unknown';
}

function actor(log: ActivityLog): string {
    return log.user?.username || 'System';
}

function metadataEntries(log: ActivityLog): [string, string][] {
    const parsed = parseJSON<unknown>(log.metadata, null);
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) return [];
    return Object.entries(parsed).map(([key, value]) => [
        key.replace(/_/g, ' '),
        value !== null && typeof value === 'object' ? JSON.stringify(value) : String(value),
    ]);
}

/**
 * One CSV cell: always quoted, quotes doubled, and anything a spreadsheet
 * would run as a formula is prefixed so it stays text.
 */
function csvCell(value: string | number | null | undefined): string {
    let text = value === null || value === undefined ? '' : String(value);
    if (/^[=+\-@\t\r]/.test(text)) text = `'${text}`;
    return `"${text.replace(/"/g, '""')}"`;
}

/** An unreadable date is exported as it was stored rather than aborting the export. */
function isoTime(value: string): string {
    const parsed = new Date(value);
    return Number.isNaN(parsed.getTime()) ? value : parsed.toISOString();
}

function toCsv(logs: ActivityLog[]): string {
    const rows: (string | number | null | undefined)[][] = [
        ['Time', 'User', 'Action', 'Resource', 'Description', 'IP address', 'Details'],
        ...logs.map((log) => [
            isoTime(log.created_at),
            actor(log),
            log.action,
            log.resource,
            log.description,
            log.ip_address,
            metadataEntries(log)
                .map(([key, value]) => `${key}: ${value}`)
                .join('; '),
        ]),
    ];
    return rows.map((row) => row.map(csvCell).join(',')).join('\r\n');
}

export default function ServiceActivityTab({ uuid }: { uuid: string }) {
    const toast = useToast();

    const [filter, setFilter] = useState('');
    const [offset, setOffset] = useState(0);
    const [refresh, setRefresh] = useState(0);
    const [exporting, setExporting] = useState(false);

    const query = `${filter}|${offset}`;
    const [page, setPage] = useState<{ query: string; logs: ActivityLog[]; total: number } | null>(null);
    const [failure, setFailure] = useState<{ query: string; message: string } | null>(null);

    useEffect(() => {
        let cancelled = false;
        void (async () => {
            try {
                const res = await api.get<ActivityPage>(`/services/${uuid}/logs`, {
                    params: { limit: PAGE_SIZE, offset, ...(filter ? { action: filter } : {}) },
                });
                if (cancelled) return;
                setPage({ query: `${filter}|${offset}`, logs: res.data.logs ?? [], total: Number(res.data.total) || 0 });
                setFailure(null);
            } catch (err) {
                if (!cancelled) setFailure({ query: `${filter}|${offset}`, message: errorMessage(err, 'Could not load the activity log.') });
            }
        })();
        return () => {
            cancelled = true;
        };
    }, [uuid, filter, offset, refresh]);

    // Keep the list fresh. The first tick is skipped: the effect above has already loaded it.
    const firstTick = useRef(true);
    usePolling(() => {
        if (firstTick.current) {
            firstTick.current = false;
            return;
        }
        setRefresh((current) => current + 1);
    }, 15000);

    const error = failure && failure.query === query ? failure.message : null;
    // While another page or filter is loading, the previous rows stay visible but dimmed.
    const stale = page !== null && page.query !== query;
    const logs = page?.logs ?? [];
    const total = page?.total ?? 0;
    const first = total === 0 ? 0 : offset + 1;
    const last = Math.min(offset + logs.length, total);

    const exportCsv = async () => {
        setExporting(true);
        try {
            const all: ActivityLog[] = [];
            let expected = Infinity;
            while (all.length < Math.min(expected, EXPORT_MAX_ROWS)) {
                const res = await api.get<ActivityPage>(`/services/${uuid}/logs`, {
                    params: { limit: EXPORT_PAGE_SIZE, offset: all.length, ...(filter ? { action: filter } : {}) },
                });
                const batch = res.data.logs ?? [];
                expected = Number(res.data.total) || 0;
                if (batch.length === 0) break;
                all.push(...batch);
            }
            if (all.length === 0) {
                toast.info('There is no activity to export.');
                return;
            }

            // The leading BOM makes spreadsheets read the file as UTF-8.
            const blob = new Blob(['﻿', toCsv(all)], { type: 'text/csv;charset=utf-8' });
            const url = URL.createObjectURL(blob);
            const link = document.createElement('a');
            link.href = url;
            link.download = `activity-${uuid.substring(0, 8)}-${new Date().toISOString().slice(0, 10)}.csv`;
            document.body.appendChild(link);
            link.click();
            link.remove();
            setTimeout(() => URL.revokeObjectURL(url), 0);

            if (expected > all.length) {
                toast.info(`Exported the newest ${all.length} entries.`);
            }
        } catch (err) {
            toast.error(errorMessage(err, 'Could not export the activity log.'));
        } finally {
            setExporting(false);
        }
    };

    return (
        <Card flush>
            <CardHeader
                title="Activity"
                description="What has been done to this server, and by whom."
                className="mb-0 px-5 pt-5 sm:px-6 sm:pt-6"
                actions={
                    <Button variant="outline" loading={exporting} onClick={exportCsv} icon={<Download size={16} aria-hidden />}>
                        Export CSV
                    </Button>
                }
            />

            <div className="px-5 pb-5 pt-4 sm:px-6">
                <Field label="Show" className="max-w-xs">
                    {(field) => (
                        <Select
                            {...field}
                            value={filter}
                            onChange={(event) => {
                                setFilter(event.target.value);
                                setOffset(0);
                            }}
                        >
                            <option value="">All activity</option>
                            {Object.entries(actionLabels).map(([action, label]) => (
                                <option key={action} value={action}>
                                    {label}
                                </option>
                            ))}
                        </Select>
                    )}
                </Field>
            </div>

            {error && (
                <div className="px-5 pb-5 sm:px-6">
                    <Alert
                        tone="danger"
                        title="The activity log could not be loaded"
                        actions={
                            <Button size="sm" variant="outline" onClick={() => setRefresh((current) => current + 1)}>
                                Try again
                            </Button>
                        }
                    >
                        {error}
                    </Alert>
                </div>
            )}

            {page === null ? (
                !error && <LoadingState label="Loading activity…" />
            ) : logs.length === 0 ? (
                <div className={cn('border-t border-border', stale && 'opacity-60')}>
                    <EmptyState
                        icon={<History size={18} aria-hidden />}
                        title={filter ? 'No activity of this kind' : 'No activity yet'}
                        description={
                            filter
                                ? 'Choose "All activity" to see everything recorded for this server.'
                                : 'Power actions, SFTP sign-ins and changes to this server will be listed here as they happen.'
                        }
                        action={
                            filter ? (
                                <Button
                                    variant="outline"
                                    onClick={() => {
                                        setFilter('');
                                        setOffset(0);
                                    }}
                                >
                                    Show all activity
                                </Button>
                            ) : undefined
                        }
                    />
                </div>
            ) : (
                <div className={cn('border-t border-border transition-opacity', stale && 'opacity-60')} aria-busy={stale || undefined}>
                    <Table>
                        <thead>
                            <tr>
                                <Th>When</Th>
                                <Th>Who</Th>
                                <Th>Action</Th>
                                <Th>What happened</Th>
                                <Th>IP address</Th>
                            </tr>
                        </thead>
                        <tbody>
                            {logs.map((log) => {
                                const details = metadataEntries(log);
                                return (
                                    <Tr key={log.id}>
                                        <Td className="whitespace-nowrap align-top text-muted">
                                            <time dateTime={log.created_at} title={formatDateTime(log.created_at)}>
                                                {formatRelative(log.created_at)}
                                            </time>
                                        </Td>
                                        <Td className="whitespace-nowrap align-top font-medium">{actor(log)}</Td>
                                        <Td className="align-top">
                                            <Badge>{actionLabel(log.action)}</Badge>
                                        </Td>
                                        <Td className="min-w-64 align-top">
                                            <p className="break-words">{log.description || '—'}</p>
                                            {log.resource && <p className="mt-0.5 break-all font-mono text-xs text-muted">{log.resource}</p>}
                                            {details.length > 0 && (
                                                <details className="mt-1 text-xs text-muted">
                                                    <summary className="w-fit hover:text-foreground">Details</summary>
                                                    <dl className="mt-1 space-y-0.5">
                                                        {details.map(([key, value]) => (
                                                            <div key={key} className="flex flex-wrap gap-x-2">
                                                                <dt>{key}:</dt>
                                                                <dd className="break-all font-mono text-foreground">{value}</dd>
                                                            </div>
                                                        ))}
                                                    </dl>
                                                </details>
                                            )}
                                        </Td>
                                        <Td className="whitespace-nowrap align-top font-mono text-xs text-muted">{log.ip_address || '—'}</Td>
                                    </Tr>
                                );
                            })}
                        </tbody>
                    </Table>
                </div>
            )}

            {page !== null && total > 0 && (
                <div className="flex flex-wrap items-center justify-between gap-3 border-t border-border px-5 py-3 sm:px-6">
                    <p className="tabular text-sm text-muted">
                        {first}–{last} of {total}
                    </p>
                    <div className="flex items-center gap-2">
                        <Button
                            size="sm"
                            variant="outline"
                            disabled={offset === 0 || stale}
                            onClick={() => setOffset(Math.max(0, offset - PAGE_SIZE))}
                            icon={<ChevronLeft size={16} aria-hidden />}
                        >
                            Previous
                        </Button>
                        <Button
                            size="sm"
                            variant="outline"
                            disabled={offset + PAGE_SIZE >= total || stale}
                            onClick={() => setOffset(offset + PAGE_SIZE)}
                        >
                            Next
                            <ChevronRight size={16} aria-hidden />
                        </Button>
                    </div>
                </div>
            )}
        </Card>
    );
}
