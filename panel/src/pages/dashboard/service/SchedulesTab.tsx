import { useRef, useState, type ReactNode } from 'react';
import { CalendarClock, Globe, Pencil, Play, Plus, Trash2 } from 'lucide-react';
import api from '../../../lib/api';
import { compareZone, describeCron, describeZone } from '../../../lib/cron';
import { errorMessage } from '../../../lib/errors';
import { formatDateTime, formatRelative } from '../../../lib/format';
import type { Schedule, ScheduleList, ScheduleRun, Service } from '../../../lib/types';
import { usePolling } from '../../../lib/usePolling';
import {
    Alert,
    Badge,
    Button,
    Card,
    CardHeader,
    EmptyState,
    IconButton,
    LoadingState,
    Switch,
    useConfirm,
    useToast,
    type Tone,
} from '../../../components/ui';
import { ScheduleDialog } from './ScheduleDialog';
import type { ServiceAccess } from './shared';

// Schedules run on their own, so the next and last runs are refreshed now and then.
const POLL_MS = 15000;
/** The most schedules the API allows on one server. */
const MAX_SCHEDULES = 25;

const runStatuses: Record<string, { label: string; tone: Tone }> = {
    success: { label: 'Success', tone: 'success' },
    failed: { label: 'Failed', tone: 'danger' },
    skipped: { label: 'Skipped', tone: 'neutral' },
};

type RowAction = 'toggle' | 'run' | 'delete';

const powerSummaries: Record<string, string> = {
    start: 'Starts the server',
    stop: 'Stops the server',
    restart: 'Restarts the server',
    kill: 'Kills the server',
};

/** What a schedule does, in a few words. */
function actionSummary(schedule: Schedule): ReactNode {
    switch (schedule.action) {
        case 'power':
            return powerSummaries[schedule.payload] ?? 'Changes the power state of the server';
        case 'command':
            return (
                <>
                    Sends the command <code className="break-all font-mono text-foreground">{schedule.payload}</code>
                </>
            );
        case 'backup':
            return schedule.payload ? `Makes a backup named "${schedule.payload}" with the date and time` : 'Makes a backup';
        default:
            return 'Does something this version of the panel does not know about';
    }
}

/** A time in the viewer's own zone, with how far away it is. */
function When({ date }: { date: string }) {
    return (
        <>
            <time dateTime={date}>{formatDateTime(date)}</time> <span className="text-muted">({formatRelative(date)})</span>
        </>
    );
}

export function SchedulesTab({ service, access }: { service: Service; access: ServiceAccess }) {
    const uuid = service.uuid;
    const toast = useToast();
    const confirm = useConfirm();

    const [schedules, setSchedules] = useState<Schedule[] | null>(null);
    const [zoneNote, setZoneNote] = useState('');
    const [loadError, setLoadError] = useState<string | null>(null);
    const [retrying, setRetrying] = useState(false);
    const [editing, setEditing] = useState<{ schedule: Schedule | null } | null>(null);
    // What is in flight for each schedule, by id. Only that schedule's controls wait for it.
    const [busy, setBusy] = useState<Record<number, RowAction>>({});
    // Counts the changes made here, so a list requested before one of them does not undo it when it arrives.
    const changes = useRef(0);

    const load = async () => {
        const seen = changes.current;
        try {
            const res = await api.get<ScheduleList>(`/services/${uuid}/schedules`);
            if (seen !== changes.current) return;
            setSchedules(Array.isArray(res.data.schedules) ? res.data.schedules : []);

            const offset = Number(res.data.timezone_offset) || 0;
            const zone = describeZone(res.data.timezone ?? '', res.data.timezone_abbr ?? '', offset);
            // The browser reports its own offset in minutes west of UTC.
            const difference = compareZone(offset, -new Date().getTimezoneOffset() * 60);
            setZoneNote(`Times are in the panel server's time zone (${zone}), which is ${difference}.`);
            setLoadError(null);
        } catch (err) {
            setLoadError(errorMessage(err, 'Could not load the schedules.'));
        }
    };

    usePolling(load, POLL_MS);

    /** Shows a change made here straight away; the next refresh confirms it. */
    const change = (update: (current: Schedule[]) => Schedule[]) => {
        changes.current += 1;
        setSchedules((current) => (current ? update(current) : current));
    };

    const replace = (next: Schedule) => change((current) => current.map((entry) => (entry.id === next.id ? next : entry)));

    const begin = (schedule: Schedule, action: RowAction) => setBusy((current) => ({ ...current, [schedule.id]: action }));

    const end = (schedule: Schedule) => {
        setBusy((current) => {
            const next = { ...current };
            delete next[schedule.id];
            return next;
        });
    };

    const tryAgain = async () => {
        setRetrying(true);
        await load();
        setRetrying(false);
    };

    const saved = (schedule: Schedule, created: boolean) => {
        setEditing(null);
        if (created) {
            change((current) => [...current.filter((entry) => entry.id !== schedule.id), schedule]);
        } else {
            replace(schedule);
        }
        toast.success(created ? `${schedule.name} was created.` : `${schedule.name} was saved.`);
        void load();
    };

    const toggle = async (schedule: Schedule, enabled: boolean) => {
        begin(schedule, 'toggle');
        // Show the change at once; put it back if the save fails.
        replace({ ...schedule, enabled });
        try {
            const res = await api.put<Schedule>(`/services/${uuid}/schedules/${schedule.id}`, { enabled });
            replace(res.data);
        } catch (err) {
            replace(schedule);
            toast.error(errorMessage(err, `Could not switch ${schedule.name} ${enabled ? 'on' : 'off'}.`));
        } finally {
            end(schedule);
        }
    };

    const runNow = async (schedule: Schedule) => {
        begin(schedule, 'run');
        try {
            const res = await api.post<ScheduleRun>(`/services/${uuid}/schedules/${schedule.id}/run`);
            if (res.data.schedule) replace(res.data.schedule);
            if (res.data.status === 'skipped') {
                toast.info(`${schedule.name} was skipped. ${res.data.message}`);
            } else {
                toast.success(res.data.message || `${schedule.name} ran.`);
            }
        } catch (err) {
            toast.error(errorMessage(err, `Could not run ${schedule.name}.`));
        } finally {
            end(schedule);
            // A run that failed is recorded on the schedule too.
            void load();
        }
    };

    const remove = async (schedule: Schedule) => {
        const confirmed = await confirm({
            title: `Delete "${schedule.name}"?`,
            description: 'The schedule is removed and stops running. Nothing it has already done is undone.',
            confirmLabel: 'Delete schedule',
            danger: true,
        });
        if (!confirmed) return;

        begin(schedule, 'delete');
        try {
            await api.delete(`/services/${uuid}/schedules/${schedule.id}`);
            change((current) => current.filter((entry) => entry.id !== schedule.id));
            toast.success(`${schedule.name} was deleted.`);
        } catch (err) {
            toast.error(errorMessage(err, `Could not delete ${schedule.name}.`));
        } finally {
            end(schedule);
            void load();
        }
    };

    const full = (schedules?.length ?? 0) >= MAX_SCHEDULES;

    return (
        <div className="space-y-4">
            <Card flush>
                <CardHeader
                    title="Schedules"
                    description={
                        <>
                            Things this server does by itself at set times: power actions, console commands and backups.
                            {full && (
                                <span className="mt-1 block">
                                    A server can have at most {MAX_SCHEDULES} schedules. Delete one to add another.
                                </span>
                            )}
                        </>
                    }
                    className="mb-0 px-5 pt-5 sm:px-6 sm:pt-6"
                    actions={
                        schedules && (
                            <Button
                                variant="primary"
                                icon={<Plus size={16} aria-hidden />}
                                disabled={full}
                                onClick={() => setEditing({ schedule: null })}
                            >
                                New schedule
                            </Button>
                        )
                    }
                />

                {loadError && (
                    <div className="px-5 pt-4 sm:px-6">
                        <Alert
                            tone={schedules ? 'warning' : 'danger'}
                            title={schedules ? 'This list may be out of date' : 'The schedules could not be loaded'}
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

                {schedules === null ? (
                    loadError ? (
                        <div className="pb-5" />
                    ) : (
                        <LoadingState label="Loading schedules…" />
                    )
                ) : schedules.length === 0 ? (
                    <EmptyState
                        icon={<CalendarClock size={18} aria-hidden />}
                        title="No schedules yet"
                        description="Add a schedule to restart the server, send a command or make a backup at the times you choose."
                        action={
                            <Button variant="outline" icon={<Plus size={16} aria-hidden />} onClick={() => setEditing({ schedule: null })}>
                                New schedule
                            </Button>
                        }
                    />
                ) : (
                    <>
                        <p className="flex items-start gap-2 px-5 pt-3 text-sm text-muted sm:px-6">
                            <Globe size={16} className="mt-0.5 shrink-0" aria-hidden />
                            <span>{zoneNote} The next and last runs below are shown in your own time.</span>
                        </p>
                        <ul className="mt-5 divide-y divide-border border-t border-border">
                            {schedules.map((schedule) => {
                                const rowBusy = busy[schedule.id] ?? null;
                                const run = runStatuses[schedule.last_status] ?? { label: 'Ran', tone: 'neutral' as const };
                                return (
                                    <li key={schedule.id} className="px-5 py-5 sm:px-6">
                                        <div className="flex flex-wrap items-start justify-between gap-x-4 gap-y-3">
                                            <div className="min-w-0 flex-1 basis-64">
                                                <p className="break-words text-sm font-medium text-foreground">{schedule.name}</p>
                                                <p className="mt-1 text-sm text-muted">
                                                    {describeCron(schedule.cron) ?? 'At the times of a custom expression'}{' '}
                                                    <code className="whitespace-nowrap font-mono text-xs text-subtle">{schedule.cron}</code>
                                                </p>
                                                <p className="mt-0.5 break-words text-sm text-muted">
                                                    {actionSummary(schedule)}
                                                    {schedule.only_when_online && schedule.action !== 'command' && ', only while the server is running'}
                                                </p>
                                            </div>
                                            <div className="flex flex-wrap items-center gap-x-3 gap-y-2">
                                                <Switch
                                                    label={
                                                        <>
                                                            Enabled<span className="sr-only">: {schedule.name}</span>
                                                        </>
                                                    }
                                                    checked={schedule.enabled}
                                                    disabled={rowBusy !== null}
                                                    onChange={(enabled) => void toggle(schedule, enabled)}
                                                    className="gap-2"
                                                />
                                                <div className="flex items-center gap-1">
                                                    <Button
                                                        size="sm"
                                                        variant="ghost"
                                                        icon={<Play size={16} aria-hidden />}
                                                        onClick={() => void runNow(schedule)}
                                                        loading={rowBusy === 'run'}
                                                        disabled={rowBusy !== null}
                                                        aria-label={`Run ${schedule.name} now`}
                                                    >
                                                        Run now
                                                    </Button>
                                                    <IconButton
                                                        size="sm"
                                                        label={`Edit ${schedule.name}`}
                                                        onClick={() => setEditing({ schedule })}
                                                        disabled={rowBusy !== null}
                                                    >
                                                        <Pencil size={16} aria-hidden />
                                                    </IconButton>
                                                    <IconButton
                                                        size="sm"
                                                        label={`Delete ${schedule.name}`}
                                                        className="hover:bg-danger/10 hover:text-danger"
                                                        onClick={() => void remove(schedule)}
                                                        loading={rowBusy === 'delete'}
                                                        disabled={rowBusy !== null}
                                                    >
                                                        <Trash2 size={16} aria-hidden />
                                                    </IconButton>
                                                </div>
                                            </div>
                                        </div>

                                        <dl className="mt-3 grid grid-cols-1 gap-x-8 gap-y-3 text-sm sm:grid-cols-2">
                                            <div className="min-w-0">
                                                <dt className="text-xs text-muted">Next run</dt>
                                                <dd className="mt-0.5 text-foreground">
                                                    {rowBusy === 'toggle' ? (
                                                        <span className="text-muted">Saving…</span>
                                                    ) : !schedule.enabled ? (
                                                        <span className="text-muted">Not planned while the schedule is switched off</span>
                                                    ) : schedule.next_run_at ? (
                                                        <When date={schedule.next_run_at} />
                                                    ) : (
                                                        <span className="text-muted">Never: no date matches this schedule</span>
                                                    )}
                                                </dd>
                                            </div>
                                            <div className="min-w-0">
                                                <dt className="text-xs text-muted">Last run</dt>
                                                <dd className="mt-0.5 text-foreground">
                                                    {schedule.last_run_at ? (
                                                        <>
                                                            <Badge tone={run.tone} className="mr-2">
                                                                {run.label}
                                                            </Badge>
                                                            <When date={schedule.last_run_at} />
                                                            {schedule.last_message && (
                                                                <p className="mt-1 break-words text-xs text-muted">{schedule.last_message}</p>
                                                            )}
                                                        </>
                                                    ) : (
                                                        <span className="text-muted">Has not run yet</span>
                                                    )}
                                                </dd>
                                            </div>
                                        </dl>
                                    </li>
                                );
                            })}
                        </ul>
                    </>
                )}
            </Card>

            {editing && (
                <ScheduleDialog
                    key={editing.schedule?.id ?? 'new'}
                    uuid={uuid}
                    schedule={editing.schedule}
                    access={access}
                    backupsEnabled={service.backup_limit !== 0}
                    zoneNote={zoneNote}
                    onSaved={saved}
                    onClose={() => setEditing(null)}
                />
            )}
        </div>
    );
}
