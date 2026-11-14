import { useId, useRef, useState, type FormEvent } from 'react';
import { Plus, Trash2 } from 'lucide-react';
import api from '../../../lib/api';
import { errorMessage } from '../../../lib/errors';
import { eggImages, eggPorts, type Egg } from '../../../lib/types';
import { Alert, Badge, Button, Field, IconButton, Input, Modal, Select, Table, Td, Textarea, Th, Tr } from '../../../components/ui';
import { portNumber } from './ServiceForm';

export interface EggEditorModalProps {
    egg: Egg;
    onClose: () => void;
    onSaved: (egg: Egg) => void;
}

interface PortRow {
    id: number;
    name: string;
    env: string;
    defaultPort: string;
    protocol: string;
}

type FieldName = 'name' | 'startup' | 'images' | 'ports';

const mono = 'font-mono text-xs leading-relaxed';

function normalizeProtocol(protocol: string | undefined): string {
    const value = (protocol ?? '').trim().toLowerCase();
    return value === 'tcp' || value === 'udp' ? value : 'both';
}

/**
 * Edits an egg. Render it only while it should be open, keyed by the egg, so
 * the form starts from the egg's values.
 */
export function EggEditorModal({ egg, onClose, onSaved }: EggEditorModalProps) {
    const formId = useId();
    const nextRowId = useRef(0);

    const [name, setName] = useState(egg.name ?? '');
    const [description, setDescription] = useState(egg.description ?? '');
    const [startup, setStartup] = useState(egg.startup_command ?? '');
    const [stop, setStop] = useState(egg.stop_command ?? '');
    const [images, setImages] = useState(() => eggImages(egg).join('\n'));
    const [scriptContainer, setScriptContainer] = useState(egg.script_container ?? '');
    const [scriptEntry, setScriptEntry] = useState(egg.script_entry ?? '');
    const [scriptInstall, setScriptInstall] = useState(egg.script_install ?? '');
    const [ports, setPorts] = useState<PortRow[]>(() =>
        eggPorts(egg).map((port, index) => ({
            id: -1 - index,
            name: port.name ?? '',
            env: port.env,
            defaultPort: port.default ? String(port.default) : '',
            protocol: normalizeProtocol(port.protocol),
        })),
    );

    const [errors, setErrors] = useState<Partial<Record<FieldName, string>>>({});
    const [formError, setFormError] = useState('');
    const [saving, setSaving] = useState(false);

    const variables = egg.variables ?? [];

    const addPort = () => {
        nextRowId.current += 1;
        const id = nextRowId.current;
        setPorts((current) => [...current, { id, name: '', env: '', defaultPort: '', protocol: 'both' }]);
    };

    const updatePort = (id: number, change: Partial<PortRow>) => {
        setPorts((current) => current.map((row) => (row.id === id ? { ...row, ...change } : row)));
    };

    const submit = async (event: FormEvent) => {
        event.preventDefault();
        if (saving) return;

        const next: Partial<Record<FieldName, string>> = {};
        const trimmedName = name.trim();
        const imageList = images
            .split('\n')
            .map((line) => line.trim())
            .filter(Boolean);

        if (!trimmedName) next.name = 'Enter a name for the egg.';
        if (!startup.trim()) next.startup = 'Enter the command that starts the game.';
        if (imageList.length === 0) next.images = 'List at least one Docker image.';

        const seen = new Set<string>();
        const portList: { name: string; env: string; default: number; protocol: string }[] = [];
        for (const row of ports) {
            const env = row.env.trim();
            const label = row.name.trim() || env || 'A port';
            const defaultPort = row.defaultPort.trim() ? portNumber(row.defaultPort) : 0;
            if (!env) {
                next.ports = `${label} needs a variable, for example QUERY_PORT.`;
            } else if (!/^[A-Za-z_][A-Za-z0-9_]*$/.test(env)) {
                next.ports = `${env} is not a valid variable. Use letters, numbers and underscores.`;
            } else if (env === 'SERVER_PORT') {
                next.ports = 'SERVER_PORT is the primary port and is always there. Remove that row.';
            } else if (seen.has(env)) {
                next.ports = `${env} is listed twice. Each port needs its own variable.`;
            } else if (defaultPort === null) {
                next.ports = `${label}: the default must be a port between 1 and 65535, or empty.`;
            } else {
                seen.add(env);
                portList.push({ name: row.name.trim() || env, env, default: defaultPort, protocol: normalizeProtocol(row.protocol) });
                continue;
            }
            break;
        }

        setErrors(next);
        setFormError('');
        if (Object.keys(next).length > 0) return;

        // The whole egg is sent back. Relations are left out: the API does not
        // update them through this endpoint.
        const payload: Record<string, unknown> = {
            ...egg,
            name: trimmedName,
            description,
            startup_command: startup,
            stop_command: stop.trim(),
            docker_images: JSON.stringify(imageList),
            script_container: scriptContainer.trim(),
            script_entry: scriptEntry.trim(),
            script_install: scriptInstall,
            ports: portList.length > 0 ? JSON.stringify(portList) : '',
        };
        delete payload.nest;
        delete payload.variables;
        delete payload.servers;

        setSaving(true);
        try {
            const res = await api.put<Egg>(`/admin/eggs/${egg.id}`, payload);
            onSaved(res.data);
        } catch (err) {
            setFormError(errorMessage(err, 'Could not save the egg.'));
            setSaving(false);
        }
    };

    return (
        <Modal
            open
            onClose={onClose}
            dismissible={!saving}
            size="xl"
            title={`Edit ${egg.name}`}
            description={
                egg.source === 'import'
                    ? undefined
                    : 'This egg comes from the catalog. Once you save it here, catalog updates no longer overwrite it.'
            }
            footer={
                <>
                    <Button variant="ghost" onClick={onClose} disabled={saving}>
                        Cancel
                    </Button>
                    <Button type="submit" form={formId} variant="primary" loading={saving}>
                        Save egg
                    </Button>
                </>
            }
        >
            <form id={formId} onSubmit={submit} className="space-y-8" noValidate>
                {formError && <Alert tone="danger">{formError}</Alert>}
                {(errors.name || errors.startup || errors.images || errors.ports) && (
                    <Alert tone="danger">Some fields need attention before the egg can be saved.</Alert>
                )}

                <section className="space-y-4">
                    <h3 className="text-sm font-semibold text-foreground">General</h3>
                    <Field label="Name" error={errors.name}>
                        {(field) => <Input {...field} value={name} maxLength={255} onChange={(event) => setName(event.target.value)} />}
                    </Field>
                    <Field label="Description" aside="Optional">
                        {(field) => (
                            <Textarea {...field} rows={2} value={description} onChange={(event) => setDescription(event.target.value)} />
                        )}
                    </Field>
                </section>

                <section className="space-y-4">
                    <h3 className="text-sm font-semibold text-foreground">Starting and stopping</h3>
                    <Field
                        label="Startup command"
                        error={errors.startup}
                        hint={
                            <>
                                Variables are written as <code className="font-mono">{'{{NAME}}'}</code>, for example{' '}
                                <code className="font-mono">{'{{SERVER_PORT}}'}</code>.
                            </>
                        }
                    >
                        {(field) => (
                            <Textarea
                                {...field}
                                rows={3}
                                spellCheck={false}
                                className={mono}
                                value={startup}
                                onChange={(event) => setStartup(event.target.value)}
                            />
                        )}
                    </Field>
                    <Field
                        label="Stop command"
                        hint={
                            <>
                                The console command that stops the game cleanly, for example <code className="font-mono">stop</code>.{' '}
                                <code className="font-mono">^C</code> sends an interrupt instead.
                            </>
                        }
                    >
                        {(field) => (
                            <Input
                                {...field}
                                spellCheck={false}
                                maxLength={255}
                                className="font-mono text-xs sm:max-w-xs"
                                value={stop}
                                onChange={(event) => setStop(event.target.value)}
                            />
                        )}
                    </Field>
                    <Field
                        label="Docker images"
                        error={errors.images}
                        hint="One image per line. The first is the default for new servers."
                    >
                        {(field) => (
                            <Textarea
                                {...field}
                                rows={4}
                                spellCheck={false}
                                wrap="off"
                                className={mono}
                                value={images}
                                onChange={(event) => setImages(event.target.value)}
                            />
                        )}
                    </Field>
                </section>

                <section className="space-y-4">
                    <h3 className="text-sm font-semibold text-foreground">Installation</h3>
                    <div className="grid gap-4 sm:grid-cols-2">
                        <Field label="Install container image" hint="The image the install script runs in.">
                            {(field) => (
                                <Input
                                    {...field}
                                    spellCheck={false}
                                    maxLength={255}
                                    className="font-mono text-xs"
                                    value={scriptContainer}
                                    onChange={(event) => setScriptContainer(event.target.value)}
                                />
                            )}
                        </Field>
                        <Field
                            label="Install script interpreter"
                            hint={
                                <>
                                    What runs the script, for example <code className="font-mono">bash</code> or{' '}
                                    <code className="font-mono">ash</code>.
                                </>
                            }
                        >
                            {(field) => (
                                <Input
                                    {...field}
                                    spellCheck={false}
                                    maxLength={255}
                                    className="font-mono text-xs"
                                    value={scriptEntry}
                                    onChange={(event) => setScriptEntry(event.target.value)}
                                />
                            )}
                        </Field>
                    </div>
                    <Field label="Install script" hint="Runs once when a server is created or reinstalled.">
                        {(field) => (
                            <Textarea
                                {...field}
                                rows={16}
                                spellCheck={false}
                                wrap="off"
                                className={`${mono} min-h-64`}
                                value={scriptInstall}
                                onChange={(event) => setScriptInstall(event.target.value)}
                            />
                        )}
                    </Field>
                </section>

                <section className="space-y-4">
                    <div className="flex flex-wrap items-start justify-between gap-3">
                        <div>
                            <h3 className="text-sm font-semibold text-foreground">Extra ports</h3>
                            <p className="mt-1 text-sm text-muted">
                                Ports the game needs besides its primary one, such as query or RCON. Each is passed to the game in its
                                variable.
                            </p>
                        </div>
                        <Button size="sm" variant="outline" icon={<Plus size={16} aria-hidden />} onClick={addPort}>
                            Add port
                        </Button>
                    </div>

                    {errors.ports && (
                        <p role="alert" className="text-xs text-danger">
                            {errors.ports}
                        </p>
                    )}

                    {ports.length === 0 ? (
                        <p className="text-sm text-muted">This egg uses only its primary port.</p>
                    ) : (
                        <ul className="space-y-3">
                            {ports.map((row) => (
                                <li
                                    key={row.id}
                                    className="grid items-start gap-3 rounded-control border border-border p-3 sm:grid-cols-2 lg:grid-cols-[1fr_1fr_8rem_10rem_auto]"
                                >
                                    <Field label="Name">
                                        {(field) => (
                                            <Input
                                                {...field}
                                                placeholder="Query"
                                                value={row.name}
                                                onChange={(event) => updatePort(row.id, { name: event.target.value })}
                                            />
                                        )}
                                    </Field>
                                    <Field label="Variable">
                                        {(field) => (
                                            <Input
                                                {...field}
                                                placeholder="QUERY_PORT"
                                                spellCheck={false}
                                                className="font-mono text-xs"
                                                value={row.env}
                                                onChange={(event) => updatePort(row.id, { env: event.target.value })}
                                            />
                                        )}
                                    </Field>
                                    <Field label="Default">
                                        {(field) => (
                                            <Input
                                                {...field}
                                                inputMode="numeric"
                                                placeholder="27015"
                                                value={row.defaultPort}
                                                onChange={(event) => updatePort(row.id, { defaultPort: event.target.value })}
                                            />
                                        )}
                                    </Field>
                                    <Field label="Protocol">
                                        {(field) => (
                                            <Select
                                                {...field}
                                                value={row.protocol}
                                                onChange={(event) => updatePort(row.id, { protocol: event.target.value })}
                                            >
                                                <option value="both">TCP and UDP</option>
                                                <option value="tcp">TCP</option>
                                                <option value="udp">UDP</option>
                                            </Select>
                                        )}
                                    </Field>
                                    <div className="flex justify-end lg:pt-7">
                                        <IconButton
                                            label={`Remove ${row.name.trim() || row.env.trim() || 'port'}`}
                                            size="sm"
                                            className="hover:bg-danger/10 hover:text-danger"
                                            onClick={() => setPorts((current) => current.filter((item) => item.id !== row.id))}
                                        >
                                            <Trash2 size={16} aria-hidden />
                                        </IconButton>
                                    </div>
                                </li>
                            ))}
                        </ul>
                    )}
                </section>

                <section className="space-y-3">
                    <div>
                        <h3 className="text-sm font-semibold text-foreground">Variables</h3>
                        <p className="mt-1 text-sm text-muted">
                            Shown for reference: variables cannot be changed here, and saving the egg leaves them as they are.
                        </p>
                    </div>
                    {variables.length === 0 ? (
                        <p className="text-sm text-muted">This egg has no variables.</p>
                    ) : (
                        <div className="rounded-control border border-border">
                            <Table>
                                <thead>
                                    <tr>
                                        <Th>Name</Th>
                                        <Th>Variable</Th>
                                        <Th>Default</Th>
                                        <Th>Owner access</Th>
                                    </tr>
                                </thead>
                                <tbody>
                                    {variables.map((variable) => (
                                        <Tr key={variable.id}>
                                            <Td className="whitespace-nowrap">{variable.name}</Td>
                                            <Td className="whitespace-nowrap font-mono text-xs text-muted">{variable.environment_variable}</Td>
                                            <Td className="max-w-64 truncate font-mono text-xs text-muted" title={variable.default_value}>
                                                {variable.default_value || <span className="font-sans text-subtle">Empty</span>}
                                            </Td>
                                            <Td>
                                                {variable.user_editable ? (
                                                    <Badge tone="success">Can edit</Badge>
                                                ) : variable.user_viewable ? (
                                                    <Badge>View only</Badge>
                                                ) : (
                                                    <Badge>Hidden</Badge>
                                                )}
                                            </Td>
                                        </Tr>
                                    ))}
                                </tbody>
                            </Table>
                        </div>
                    )}
                </section>
            </form>
        </Modal>
    );
}
