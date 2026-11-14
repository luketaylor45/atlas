import { useEffect, useId, useRef, useState, type FormEvent } from 'react';
import api from '../../../lib/api';
import {
    DAY_NAMES,
    DEFAULT_BUILDER,
    HOUR_GAPS,
    MINUTE_GAPS,
    WEEK,
    buildCron,
    builderFromCron,
    describeCron,
    type BuilderField,
    type CronBuilder,
    type RepeatMode,
} from '../../../lib/cron';
import { errorMessage } from '../../../lib/errors';
import type { Schedule, ScheduleAction } from '../../../lib/types';
import { Alert, Button, Field, Input, Modal, Select, Switch } from '../../../components/ui';
import type { PowerAction, ServiceAccess } from './shared';

export interface ScheduleDialogProps {
    uuid: string;
    /** The schedule being changed, or null to make a new one. */
    schedule: Schedule | null;
    /** What the signed-in user may do by hand. Only those things can be scheduled. */
    access: ServiceAccess;
    /** False when backups are turned off for this server. */
    backupsEnabled: boolean;
    /** The sentence saying which time zone the times are in. */
    zoneNote: string;
    onSaved: (schedule: Schedule, created: boolean) => void;
    onClose: () => void;
}

const actionOptions: { value: ScheduleAction; label: string }[] = [
    { value: 'power', label: 'Start, stop or restart the server' },
    { value: 'command', label: 'Send a console command' },
    { value: 'backup', label: 'Make a backup' },
];

const powerOptions: { value: PowerAction; label: string }[] = [
    { value: 'start', label: 'Start the server' },
    { value: 'stop', label: 'Stop the server' },
    { value: 'restart', label: 'Restart the server' },
    { value: 'kill', label: 'Kill the server' },
];

const repeatOptions: { value: RepeatMode; label: string }[] = [
    { value: 'daily', label: 'Every day' },
    { value: 'weekly', label: 'Certain days of the week' },
    { value: 'hours', label: 'Every few hours' },
    { value: 'minutes', label: 'Every few minutes' },
    { value: 'custom', label: 'Custom' },
];

const dayChip =
    'inline-flex h-9 min-w-12 cursor-pointer select-none items-center justify-center rounded-control border border-border-strong px-3 ' +
    'text-sm font-medium text-muted transition-colors hover:bg-secondary ' +
    'peer-checked:border-primary peer-checked:bg-primary/10 peer-checked:text-primary-text peer-checked:ring-1 peer-checked:ring-primary ' +
    'peer-focus-visible:outline peer-focus-visible:outline-2 peer-focus-visible:outline-offset-2 peer-focus-visible:outline-primary';

function initialAction(schedule: Schedule | null, access: ServiceAccess): ScheduleAction {
    if (schedule) return schedule.action === 'command' || schedule.action === 'backup' ? schedule.action : 'power';
    // A new schedule starts on something this person is allowed to schedule.
    return access.canPower ? 'power' : access.canCommands ? 'command' : access.canBackups ? 'backup' : 'power';
}

function initialPower(schedule: Schedule | null): PowerAction {
    const payload = schedule?.action === 'power' ? schedule.payload : '';
    return payload === 'start' || payload === 'stop' || payload === 'kill' ? payload : 'restart';
}

/** Why this person may not schedule an action, or an empty string when they may. */
function deniedReason(action: ScheduleAction, access: ServiceAccess): string {
    if (action === 'power' && !access.canPower) {
        return "You do not have permission to control this server's power, so you cannot schedule it.";
    }
    if (action === 'command' && !access.canCommands) {
        return 'You do not have permission to send commands to this server, so you cannot schedule one.';
    }
    if (action === 'backup' && !access.canBackups) {
        return "You do not have permission to manage this server's backups, so you cannot schedule one.";
    }
    return '';
}

/**
 * Creates or changes a schedule. Render it only while it should be open,
 * keyed by the schedule, so the form starts from its values.
 */
export function ScheduleDialog({ uuid, schedule, access, backupsEnabled, zoneNote, onSaved, onClose }: ScheduleDialogProps) {
    const formId = useId();
    const [name, setName] = useState(schedule?.name ?? '');
    const [action, setAction] = useState<ScheduleAction>(() => initialAction(schedule, access));
    const [power, setPower] = useState<PowerAction>(() => initialPower(schedule));
    const [command, setCommand] = useState(schedule?.action === 'command' ? schedule.payload : '');
    const [backupName, setBackupName] = useState(schedule?.action === 'backup' ? schedule.payload : '');
    const [onlyWhenOnline, setOnlyWhenOnline] = useState(Boolean(schedule?.only_when_online));
    const [builder, setBuilder] = useState<CronBuilder>(() => (schedule ? builderFromCron(schedule.cron) : DEFAULT_BUILDER));

    // Problems are pointed out once saving has been tried, not while the form is still being filled in.
    const [attempts, setAttempts] = useState(0);
    const [formError, setFormError] = useState('');
    const [saving, setSaving] = useState(false);
    const messages = useRef<HTMLDivElement>(null);

    const attempted = attempts > 0;
    const built = buildCron(builder);
    const trimmedName = name.trim();
    const trimmedCommand = command.trim();
    const denied = deniedReason(action, access);
    const nameError = trimmedName ? '' : 'Give the schedule a name.';
    const commandError = action === 'command' && !trimmedCommand ? 'Enter the command to send.' : '';
    const invalid = Boolean(nameError || commandError || denied || !built.cron);
    const whenError = (field: BuilderField) => (attempted && built.field === field ? built.error : undefined);

    // The messages sit at the end of a form that can be taller than the dialog.
    useEffect(() => {
        if (attempts > 0) messages.current?.scrollIntoView({ block: 'nearest' });
    }, [attempts, formError]);

    const update = (changes: Partial<CronBuilder>) => {
        setBuilder((current) => ({ ...current, ...changes }));
    };

    const changeMode = (mode: RepeatMode) => {
        setBuilder((current) => {
            // Custom starts from what the simpler choices made, so it can be adjusted instead of typed from nothing.
            const made = buildCron(current);
            return { ...current, mode, custom: mode === 'custom' && made.cron ? made.cron : current.custom };
        });
    };

    const toggleDay = (day: number) => {
        setBuilder((current) => ({
            ...current,
            days: current.days.includes(day) ? current.days.filter((each) => each !== day) : [...current.days, day],
        }));
    };

    const submit = async (event: FormEvent) => {
        event.preventDefault();
        if (saving) return;

        setAttempts((current) => current + 1);
        setFormError('');
        if (invalid || !built.cron) return;

        const body = {
            name: trimmedName,
            cron: built.cron,
            action,
            payload: action === 'power' ? power : action === 'command' ? trimmedCommand : backupName.trim(),
            only_when_online: onlyWhenOnline,
        };

        setSaving(true);
        try {
            const res = schedule
                ? await api.put<Schedule>(`/services/${uuid}/schedules/${schedule.id}`, body)
                : await api.post<Schedule>(`/services/${uuid}/schedules`, body);
            onSaved(res.data, !schedule);
        } catch (err) {
            setFormError(errorMessage(err, schedule ? 'Could not save the schedule.' : 'Could not create the schedule.'));
            setSaving(false);
        }
    };

    return (
        <Modal
            open
            onClose={onClose}
            dismissible={!saving}
            size="lg"
            title={schedule ? `Edit ${schedule.name}` : 'New schedule'}
            footer={
                <>
                    <Button variant="ghost" onClick={onClose} disabled={saving}>
                        Cancel
                    </Button>
                    <Button type="submit" form={formId} variant="primary" loading={saving}>
                        {schedule ? 'Save changes' : 'Create schedule'}
                    </Button>
                </>
            }
        >
            <form id={formId} onSubmit={submit} className="space-y-8" noValidate>
                <Field label="Name" error={attempted && nameError ? nameError : undefined}>
                    {(field) => (
                        <Input
                            {...field}
                            value={name}
                            onChange={(event) => setName(event.target.value)}
                            placeholder="Nightly restart"
                            autoComplete="off"
                            maxLength={100}
                        />
                    )}
                </Field>

                <section className="space-y-4">
                    <h3 className="text-sm font-semibold text-foreground">What it does</h3>
                    <div className="grid items-start gap-4 sm:grid-cols-2">
                        <Field label="Action" error={denied || undefined}>
                            {(field) => (
                                <Select {...field} value={action} onChange={(event) => setAction(event.target.value as ScheduleAction)}>
                                    {actionOptions.map((option) => (
                                        <option key={option.value} value={option.value}>
                                            {option.label}
                                        </option>
                                    ))}
                                </Select>
                            )}
                        </Field>

                        {action === 'power' && (
                            <Field
                                label="Power action"
                                hint={power === 'kill' ? 'Stops the server at once, without giving it a chance to save.' : undefined}
                            >
                                {(field) => (
                                    <Select {...field} value={power} onChange={(event) => setPower(event.target.value as PowerAction)}>
                                        {powerOptions.map((option) => (
                                            <option key={option.value} value={option.value}>
                                                {option.label}
                                            </option>
                                        ))}
                                    </Select>
                                )}
                            </Field>
                        )}

                        {action === 'command' && (
                            <Field
                                label="Command"
                                error={attempted && commandError ? commandError : undefined}
                                hint="Sent to the console as if you had typed it. It is only sent while the server is running."
                            >
                                {(field) => (
                                    <Input
                                        {...field}
                                        value={command}
                                        onChange={(event) => setCommand(event.target.value)}
                                        placeholder="say Restarting in five minutes"
                                        className="font-mono"
                                        autoComplete="off"
                                        autoCapitalize="off"
                                        spellCheck={false}
                                        maxLength={500}
                                    />
                                )}
                            </Field>
                        )}

                        {action === 'backup' && (
                            <Field
                                label="Backup name"
                                aside="Optional"
                                hint="The date and time are added to it. Leave it empty to use the schedule's name."
                            >
                                {(field) => (
                                    <Input
                                        {...field}
                                        value={backupName}
                                        onChange={(event) => setBackupName(event.target.value)}
                                        placeholder="Nightly"
                                        autoComplete="off"
                                        maxLength={80}
                                    />
                                )}
                            </Field>
                        )}
                    </div>

                    {action === 'backup' &&
                        (backupsEnabled ? (
                            <p className="text-xs text-muted">
                                A run is skipped while another backup is in progress, and when every backup the server may keep was made by
                                hand. Otherwise the oldest automatic backup makes room for the new one.
                            </p>
                        ) : (
                            <Alert tone="warning">
                                Backups are not enabled for this server, so every run is skipped until an administrator allows them.
                            </Alert>
                        ))}

                    {action !== 'command' && (
                        <Switch
                            label="Only when the server is running"
                            description="Skips a run while the server is stopped."
                            checked={onlyWhenOnline}
                            onChange={setOnlyWhenOnline}
                        />
                    )}
                </section>

                <section className="space-y-4">
                    <h3 className="text-sm font-semibold text-foreground">When it runs</h3>
                    <div className="grid items-start gap-4 sm:grid-cols-2">
                        <Field label="Repeat">
                            {(field) => (
                                <Select {...field} value={builder.mode} onChange={(event) => changeMode(event.target.value as RepeatMode)}>
                                    {repeatOptions.map((option) => (
                                        <option key={option.value} value={option.value}>
                                            {option.label}
                                        </option>
                                    ))}
                                </Select>
                            )}
                        </Field>

                        {(builder.mode === 'daily' || builder.mode === 'weekly') && (
                            <Field label="Time" error={whenError('time')}>
                                {(field) => (
                                    <Input {...field} type="time" value={builder.time} onChange={(event) => update({ time: event.target.value })} />
                                )}
                            </Field>
                        )}

                        {builder.mode === 'hours' && (
                            <Field label="How often" error={whenError('every')}>
                                {(field) => (
                                    <Select
                                        {...field}
                                        value={builder.everyHours}
                                        onChange={(event) => update({ everyHours: Number(event.target.value) })}
                                    >
                                        {HOUR_GAPS.map((gap) => (
                                            <option key={gap} value={gap}>
                                                {gap === 1 ? 'Every hour' : `Every ${gap} hours`}
                                            </option>
                                        ))}
                                    </Select>
                                )}
                            </Field>
                        )}

                        {builder.mode === 'minutes' && (
                            <Field label="How often" error={whenError('every')}>
                                {(field) => (
                                    <Select
                                        {...field}
                                        value={builder.everyMinutes}
                                        onChange={(event) => update({ everyMinutes: Number(event.target.value) })}
                                    >
                                        {MINUTE_GAPS.map((gap) => (
                                            <option key={gap} value={gap}>
                                                {gap === 1 ? 'Every minute' : `Every ${gap} minutes`}
                                            </option>
                                        ))}
                                    </Select>
                                )}
                            </Field>
                        )}
                    </div>

                    {builder.mode === 'weekly' && (
                        <fieldset>
                            <legend className="text-sm font-medium text-foreground">Days</legend>
                            <div className="mt-1.5 flex flex-wrap gap-2">
                                {WEEK.map((day) => (
                                    <label key={day} className="relative">
                                        <input
                                            type="checkbox"
                                            className="peer sr-only"
                                            checked={builder.days.includes(day)}
                                            onChange={() => toggleDay(day)}
                                        />
                                        <span className={dayChip} title={DAY_NAMES[day]}>
                                            <span aria-hidden>{DAY_NAMES[day].slice(0, 3)}</span>
                                            <span className="sr-only">{DAY_NAMES[day]}</span>
                                        </span>
                                    </label>
                                ))}
                            </div>
                            {whenError('days') && (
                                <p role="alert" className="mt-1.5 text-xs text-danger">
                                    {whenError('days')}
                                </p>
                            )}
                        </fieldset>
                    )}

                    {builder.mode === 'custom' && (
                        <Field
                            label="Cron expression"
                            error={whenError('custom')}
                            hint={
                                <>
                                    Five values separated by spaces: minute, hour, day of the month, month and day of the week (0 is
                                    Sunday). Use <code className="font-mono">*</code> for every value, a list like{' '}
                                    <code className="font-mono">1,15</code>, a range like <code className="font-mono">1-5</code> or a step
                                    like <code className="font-mono">*/10</code>.
                                </>
                            }
                        >
                            {(field) => (
                                <Input
                                    {...field}
                                    value={builder.custom}
                                    onChange={(event) => update({ custom: event.target.value })}
                                    placeholder="0 4 * * 1"
                                    className="font-mono"
                                    autoComplete="off"
                                    autoCapitalize="off"
                                    spellCheck={false}
                                    maxLength={120}
                                />
                            )}
                        </Field>
                    )}

                    <div className="rounded-control border border-border bg-secondary px-3 py-2.5" aria-live="polite">
                        {built.cron ? (
                            <>
                                <p className="text-sm font-medium text-foreground">{describeCron(built.cron)}</p>
                                <p className="mt-1 text-xs text-muted">
                                    Expression: <code className="font-mono text-foreground">{built.cron}</code>
                                </p>
                            </>
                        ) : (
                            <p className="text-sm text-muted">{built.error}</p>
                        )}
                    </div>
                    {zoneNote && <p className="text-xs text-muted">{zoneNote}</p>}
                </section>

                {attempted && (formError || invalid) && (
                    <div ref={messages}>
                        <Alert tone="danger">{formError || 'Some fields need attention before the schedule can be saved.'}</Alert>
                    </div>
                )}
            </form>
        </Modal>
    );
}
