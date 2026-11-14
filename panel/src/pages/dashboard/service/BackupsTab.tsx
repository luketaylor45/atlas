import { useId, useRef, useState, type FormEvent } from 'react';
import { Archive, ArchiveRestore, Download, Plus, Trash2 } from 'lucide-react';
import api from '../../../lib/api';
import { cn } from '../../../lib/cn';
import { errorMessage } from '../../../lib/errors';
import { formatBytes, formatDateTime, formatRelative } from '../../../lib/format';
import type { Backup, BackupList, Service } from '../../../lib/types';
import { usePolling } from '../../../lib/usePolling';
import {
    Alert,
    Badge,
    Button,
    Card,
    CardHeader,
    EmptyState,
    Field,
    IconButton,
    Input,
    LoadingState,
    Modal,
    Switch,
    useConfirm,
    useToast,
    type Tone,
} from '../../../components/ui';

// Nothing tells the panel when a backup finishes, so the list is asked for
// again: often while one is being made or restored, now and then otherwise
// (a schedule can add one at any time).
const BUSY_POLL_MS = 3000;
const IDLE_POLL_MS = 20000;

interface BackupStatusInfo {
    label: string;
    tone: Tone;
    /** True while the node is still working on the backup. */
    busy: boolean;
}

const statuses: Record<string, BackupStatusInfo> = {
    creating: { label: 'Creating', tone: 'primary', busy: true },
    restoring: { label: 'Restoring', tone: 'info', busy: true },
    completed: { label: 'Completed', tone: 'success', busy: false },
    failed: { label: 'Failed', tone: 'danger', busy: false },
};

function backupStatus(backup: Backup): BackupStatusInfo {
    return statuses[backup.status] ?? { label: backup.status || 'Unknown', tone: 'neutral', busy: false };
}

/**
 * Starts a browser download of a link the API issued. The link carries its own
 * short-lived ticket, so the browser opens it itself, without the session.
 */
function startDownload(path: string) {
    // The path already begins with /api/v1, so only the origin of the API's address is taken.
    const origin = new URL(String(api.defaults.baseURL || ''), window.location.href);
    const link = document.createElement('a');
    link.href = new URL(path, origin).toString();
    link.download = '';
    document.body.appendChild(link);
    link.click();
    link.remove();
}

export function BackupsTab({ service }: { service: Service }) {
    const uuid = service.uuid;
    const toast = useToast();
    const confirm = useConfirm();

    const [list, setList] = useState<BackupList | null>(null);
    const [loadError, setLoadError] = useState<string | null>(null);
    const [retrying, setRetrying] = useState(false);
    const [creating, setCreating] = useState(false);
    const [restoring, setRestoring] = useState<Backup | null>(null);
    const [busy, setBusy] = useState<{ uuid: string; action: 'download' | 'delete' } | null>(null);
    // Counts the changes made here, so a list requested before one of them does not undo it when it arrives.
    const changes = useRef(0);

    const load = async () => {
        const seen = changes.current;
        try {
            const res = await api.get<BackupList>(`/services/${uuid}/backups`);
            if (seen !== changes.current) return;
            setList({ backups: Array.isArray(res.data.backups) ? res.data.backups : [], limit: Number(res.data.limit) || 0 });
            setLoadError(null);
        } catch (err) {
            setLoadError(errorMessage(err, 'Could not load the backups.'));
        }
    };

    const backups = list?.backups ?? [];
    const limit = list?.limit ?? 0;
    // A failed backup holds no files and does not count towards the limit.
    const used = backups.filter((backup) => backup.status !== 'failed').length;
    const working = backups.some((backup) => backupStatus(backup).busy);

    usePolling(load, working ? BUSY_POLL_MS : IDLE_POLL_MS);

    const blocked =
        used >= limit
            ? `This server has its limit of ${limit} ${limit === 1 ? 'backup' : 'backups'}. Delete one to make room for another.`
            : working
              ? 'Another backup or restore is in progress. Wait for it to finish before making a backup.'
              : null;

    /** Shows a change made here straight away; the refresh that follows confirms it. */
    const change = (update: (current: Backup[]) => Backup[]) => {
        changes.current += 1;
        setList((current) => (current ? { ...current, backups: update(current.backups) } : current));
    };

    const tryAgain = async () => {
        setRetrying(true);
        await load();
        setRetrying(false);
    };

    const created = (backup: Backup) => {
        setCreating(false);
        change((current) => [backup, ...current.filter((entry) => entry.uuid !== backup.uuid)]);
        toast.success('Backup started. You can leave this page while it is made.');
        void load();
    };

    const restoreStarted = (backup: Backup) => {
        setRestoring(null);
        change((current) => current.map((entry) => (entry.uuid === backup.uuid ? { ...entry, status: 'restoring', error: '' } : entry)));
        toast.success('Restore started. The server can be started again once it has finished.');
        void load();
    };

    const download = async (backup: Backup) => {
        setBusy({ uuid: backup.uuid, action: 'download' });
        try {
            const res = await api.post<{ url: string }>(`/services/${uuid}/backups/${backup.uuid}/download`);
            startDownload(res.data.url);
        } catch (err) {
            toast.error(errorMessage(err, `Could not download ${backup.name}.`));
        } finally {
            setBusy(null);
        }
    };

    const remove = async (backup: Backup) => {
        const confirmed = await confirm({
            title: `Delete "${backup.name}"?`,
            description: 'The backup is permanently deleted. This cannot be undone. The server and its files are not affected.',
            confirmLabel: 'Delete backup',
            danger: true,
        });
        if (!confirmed) return;

        setBusy({ uuid: backup.uuid, action: 'delete' });
        try {
            await api.delete(`/services/${uuid}/backups/${backup.uuid}`);
            change((current) => current.filter((entry) => entry.uuid !== backup.uuid));
            toast.success(`Deleted ${backup.name}`);
        } catch (err) {
            toast.error(errorMessage(err, `Could not delete ${backup.name}.`));
        } finally {
            setBusy(null);
            void load();
        }
    };

    return (
        <div className="space-y-4">
            <Card flush>
                <CardHeader
                    title="Backups"
                    description={
                        list && limit > 0 ? (
                            <>
                                <span className="tabular">
                                    {used} of {limit}
                                </span>{' '}
                                {limit === 1 ? 'backup' : 'backups'} used
                                {blocked && <span className="mt-1 block">{blocked}</span>}
                            </>
                        ) : (
                            "Copies of this server's files that you can download or put back."
                        )
                    }
                    className="mb-0 px-5 pt-5 sm:px-6 sm:pt-6"
                    actions={
                        list &&
                        limit > 0 && (
                            <Button
                                variant="primary"
                                icon={<Plus size={16} aria-hidden />}
                                disabled={blocked !== null}
                                onClick={() => setCreating(true)}
                            >
                                Create backup
                            </Button>
                        )
                    }
                />

                {loadError && (
                    <div className="px-5 pt-4 sm:px-6">
                        <Alert
                            tone={list ? 'warning' : 'danger'}
                            title={list ? 'This list may be out of date' : 'The backups could not be loaded'}
                            actions={
                                <Button size="sm" variant="outline" loading={retrying} onClick={tryAgain}>
                                    Try again
                                </Button>
                            }
                        >
                            {loadError}
                        </Alert>
                    </div>
                )}

                {list === null ? (
                    loadError ? (
                        <div className="pb-5" />
                    ) : (
                        <LoadingState label="Loading backups…" />
                    )
                ) : backups.length === 0 ? (
                    limit === 0 ? (
                        <EmptyState
                            icon={<Archive size={18} aria-hidden />}
                            title="Backups are not enabled for this server"
                            description="An administrator can allow them."
                        />
                    ) : (
                        <EmptyState
                            icon={<Archive size={18} aria-hidden />}
                            title="No backups yet"
                            description="Create a backup to keep a copy of this server's files as they are now."
                            action={
                                <Button variant="outline" icon={<Plus size={16} aria-hidden />} onClick={() => setCreating(true)}>
                                    Create backup
                                </Button>
                            }
                        />
                    )
                ) : (
                    <>
                        {limit === 0 && (
                            <div className="px-5 pt-4 sm:px-6">
                                <Alert tone="info" title="Backups are not enabled for this server">
                                    An administrator can allow them. The backups already made can still be downloaded, restored and
                                    deleted.
                                </Alert>
                            </div>
                        )}
                        <ul className="mt-5 divide-y divide-border border-t border-border">
                            {backups.map((backup) => {
                                const status = backupStatus(backup);
                                const finished = backup.status === 'completed';
                                const rowBusy = busy?.uuid === backup.uuid ? busy.action : null;
                                return (
                                    <li key={backup.uuid} className="flex flex-wrap items-center justify-between gap-x-4 gap-y-3 px-5 py-4 sm:px-6">
                                        <div className="min-w-0 flex-1 basis-64">
                                            <div className="flex flex-wrap items-center gap-2">
                                                <p className="min-w-0 break-words text-sm font-medium text-foreground">{backup.name}</p>
                                                {backup.automated && <Badge>Automatic</Badge>}
                                            </div>
                                            <div className="mt-1.5 flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-muted">
                                                <Badge tone={status.tone} dot pulse={status.busy}>
                                                    {status.label}
                                                </Badge>
                                                {backup.size > 0 && <span className="tabular">{formatBytes(backup.size)}</span>}
                                                <time dateTime={backup.created_at} title={formatDateTime(backup.created_at)}>
                                                    {formatRelative(backup.created_at)}
                                                </time>
                                            </div>
                                            {backup.error && (
                                                <p className={cn('mt-2 break-words text-xs', finished ? 'text-warning' : 'text-danger')}>
                                                    {backup.error}
                                                </p>
                                            )}
                                        </div>
                                        <div className="flex items-center gap-1">
                                            <Button
                                                size="sm"
                                                variant="ghost"
                                                icon={<Download size={16} aria-hidden />}
                                                onClick={() => void download(backup)}
                                                loading={rowBusy === 'download'}
                                                disabled={busy !== null || !(finished || backup.status === 'restoring')}
                                                aria-label={`Download ${backup.name}`}
                                            >
                                                Download
                                            </Button>
                                            <Button
                                                size="sm"
                                                variant="ghost"
                                                icon={<ArchiveRestore size={16} aria-hidden />}
                                                onClick={() => setRestoring(backup)}
                                                disabled={busy !== null || !finished}
                                                aria-label={`Restore ${backup.name}`}
                                            >
                                                Restore
                                            </Button>
                                            <IconButton
                                                size="sm"
                                                label={`Delete ${backup.name}`}
                                                className="hover:bg-danger/10 hover:text-danger"
                                                onClick={() => void remove(backup)}
                                                loading={rowBusy === 'delete'}
                                                disabled={busy !== null || backup.status === 'restoring'}
                                            >
                                                <Trash2 size={16} aria-hidden />
                                            </IconButton>
                                        </div>
                                    </li>
                                );
                            })}
                        </ul>
                    </>
                )}
            </Card>

            <p className="text-xs text-muted">
                Backups are kept on the server&apos;s node and do not count towards its disk limit. A backup made by a schedule is marked
                Automatic: once the limit is reached, the oldest automatic backup is replaced by the new one. Backups you make by hand are
                never removed automatically.
            </p>

            {creating && <CreateBackupDialog uuid={uuid} onCreated={created} onClose={() => setCreating(false)} />}
            {restoring && (
                <RestoreBackupDialog
                    uuid={uuid}
                    backup={restoring}
                    stopped={service.status === 'offline'}
                    onStarted={restoreStarted}
                    onClose={() => setRestoring(null)}
                />
            )}
        </div>
    );
}

interface CreateBackupDialogProps {
    uuid: string;
    /** Receives the backup the API has started making. */
    onCreated: (backup: Backup) => void;
    onClose: () => void;
}

/** Asks for the name of a new backup and starts it. Mount it only while it is open. */
function CreateBackupDialog({ uuid, onCreated, onClose }: CreateBackupDialogProps) {
    const formId = useId();
    const [name, setName] = useState('');
    const [error, setError] = useState<string | null>(null);
    const [submitting, setSubmitting] = useState(false);

    const submit = async (event: FormEvent) => {
        event.preventDefault();
        if (submitting) return;

        setSubmitting(true);
        setError(null);
        try {
            const trimmed = name.trim();
            const res = await api.post<Backup>(`/services/${uuid}/backups`, trimmed ? { name: trimmed } : {});
            onCreated(res.data);
        } catch (err) {
            setError(errorMessage(err, 'Could not start the backup.'));
            setSubmitting(false);
        }
    };

    return (
        <Modal
            open
            onClose={onClose}
            size="sm"
            dismissible={!submitting}
            title="Create backup"
            description="Saves a copy of every file on this server as it is now. You can keep using the server while it is made."
            footer={
                <>
                    <Button variant="ghost" onClick={onClose} disabled={submitting}>
                        Cancel
                    </Button>
                    <Button type="submit" form={formId} variant="primary" loading={submitting}>
                        Create backup
                    </Button>
                </>
            }
        >
            <form id={formId} onSubmit={submit} className="space-y-4" noValidate>
                <Field label="Name" aside="Optional" hint="Leave it empty to name the backup after the date and time.">
                    {(field) => (
                        <Input
                            {...field}
                            value={name}
                            onChange={(event) => setName(event.target.value)}
                            placeholder="Before the update"
                            autoComplete="off"
                            maxLength={100}
                        />
                    )}
                </Field>
                {error && <Alert tone="danger">{error}</Alert>}
            </form>
        </Modal>
    );
}

interface RestoreBackupDialogProps {
    uuid: string;
    backup: Backup;
    /** A backup can only be restored while the server is stopped. */
    stopped: boolean;
    /** Called once the node has started restoring. */
    onStarted: (backup: Backup) => void;
    onClose: () => void;
}

/** Confirms restoring a backup and asks what should happen to the other files. Mount it only while it is open. */
function RestoreBackupDialog({ uuid, backup, stopped, onStarted, onClose }: RestoreBackupDialogProps) {
    const [truncate, setTruncate] = useState(false);
    const [error, setError] = useState<string | null>(null);
    const [submitting, setSubmitting] = useState(false);

    const restore = async () => {
        if (submitting || !stopped) return;

        setSubmitting(true);
        setError(null);
        try {
            await api.post(`/services/${uuid}/backups/${backup.uuid}/restore`, { truncate });
            onStarted(backup);
        } catch (err) {
            setError(errorMessage(err, 'Could not start the restore.'));
            setSubmitting(false);
        }
    };

    return (
        <Modal
            open
            onClose={onClose}
            dismissible={!submitting}
            title={`Restore "${backup.name}"?`}
            description="Puts the files from this backup back on the server."
            footer={
                <>
                    <Button variant="ghost" onClick={onClose} disabled={submitting}>
                        Cancel
                    </Button>
                    <Button variant={truncate ? 'danger' : 'primary'} loading={submitting} disabled={!stopped} onClick={restore}>
                        Restore backup
                    </Button>
                </>
            }
        >
            <div className="space-y-4">
                {!stopped && (
                    <Alert tone="warning" title="Stop the server first">
                        A backup can only be restored while the server is stopped. Stop the server, then restore the backup.
                    </Alert>
                )}

                <p className="text-sm text-muted">
                    Each file in the backup replaces the file with the same name on the server. Everything else on the server is kept,
                    unless you switch on the option below.
                </p>

                <div className="rounded-control border border-border px-3 py-2">
                    <Switch
                        label="Also delete files that are not in the backup"
                        description="The server's current files are deleted first, so only the backup's files remain."
                        checked={truncate}
                        disabled={submitting}
                        onChange={setTruncate}
                    />
                </div>

                {truncate && (
                    <Alert tone="warning" title="Every file now on the server will be deleted">
                        Anything added or changed since this backup was made is lost. This cannot be undone.
                    </Alert>
                )}

                {error && <Alert tone="danger">{error}</Alert>}
            </div>
        </Modal>
    );
}
