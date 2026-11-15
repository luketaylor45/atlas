import { useEffect, useMemo, useState, type FormEvent } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { ArrowLeft, Check, Egg as EggIcon, Lock } from 'lucide-react';
import api from '../../lib/api';
import { cn } from '../../lib/cn';
import { errorMessage, errorStatus } from '../../lib/errors';
import { formatCpu, formatMB } from '../../lib/format';
import { variableError } from '../../lib/rules';
import { eggImages, eggPorts, type Egg, type Nest, type Node, type Service, type User } from '../../lib/types';
import ResourceSlider from '../../components/ResourceSlider';
import VariableInput from '../../components/VariableInput';
import {
    Alert,
    Button,
    Card,
    CardHeader,
    EmptyState,
    Field,
    Input,
    LoadingState,
    PageHeader,
    Select,
    useToast,
} from '../../components/ui';
import { groupNests, nestEggs } from './shared/NestGroups';
import { resourceLimits } from './shared/resourceLimits';
import { portInMessage, portNumber, primaryLinkClass, wholeNumber } from './shared/ServiceForm';

interface Options {
    nodes: Node[];
    nests: Nest[];
    users: User[];
}

/** A titled set of eggs in the game picker. */
interface EggSection {
    key: string;
    title: string;
    eggs: Egg[];
}

const extraKey = (env: string) => `extra:${env}`;

function protocolLabel(protocol: string | undefined): string {
    const value = (protocol ?? '').trim().toLowerCase();
    if (value === 'tcp') return 'TCP';
    if (value === 'udp') return 'UDP';
    return 'TCP and UDP';
}

export default function CreateServicePage() {
    const navigate = useNavigate();
    const toast = useToast();

    const [options, setOptions] = useState<Options | null>(null);
    const [loadError, setLoadError] = useState('');

    const [eggId, setEggId] = useState<number | null>(null);
    const [eggSearch, setEggSearch] = useState('');
    const [name, setName] = useState('');
    const [userId, setUserId] = useState('');
    const [nodeId, setNodeId] = useState('');
    const [memory, setMemory] = useState('1024');
    const [cpu, setCpu] = useState('100');
    const [disk, setDisk] = useState('5120');
    const [image, setImage] = useState('');
    const [port, setPort] = useState('');
    const [extraPorts, setExtraPorts] = useState<Record<string, string>>({});
    const [variables, setVariables] = useState<Record<string, string>>({});

    const [errors, setErrors] = useState<Record<string, string>>({});
    const [formError, setFormError] = useState('');
    const [submitting, setSubmitting] = useState(false);

    const [attempt, setAttempt] = useState(0);

    useEffect(() => {
        let cancelled = false;
        Promise.all([
            api.get<Node[] | null>('/admin/nodes'),
            api.get<Nest[] | null>('/admin/nests'),
            api.get<User[] | null>('/admin/users'),
        ])
            .then(([nodes, nests, users]) => {
                if (cancelled) return;
                const loaded: Options = {
                    nodes: Array.isArray(nodes.data) ? nodes.data : [],
                    nests: Array.isArray(nests.data) ? nests.data : [],
                    users: Array.isArray(users.data) ? users.data : [],
                };
                setOptions(loaded);
                setLoadError('');
                // With a single node there is nothing to choose.
                if (loaded.nodes.length === 1) setNodeId((current) => current || String(loaded.nodes[0].id));
            })
            .catch((err: unknown) => {
                if (!cancelled) setLoadError(errorMessage(err, 'Could not load the nodes, eggs and users.'));
            });
        return () => {
            cancelled = true;
        };
    }, [attempt]);

    const sections = useMemo<EggSection[]>(() => {
        const result: EggSection[] = [];
        for (const group of groupNests(options?.nests ?? [])) {
            // Eggs can sit directly in a root nest as well as in its sub-nests.
            const direct = nestEggs(group.root);
            if (direct.length > 0) result.push({ key: `nest-${group.root.id}`, title: group.root.name, eggs: direct });
            for (const child of group.children) {
                const eggs = nestEggs(child);
                if (eggs.length > 0) {
                    result.push({ key: `nest-${child.id}`, title: `${group.root.name} / ${child.name}`, eggs });
                }
            }
        }
        return result;
    }, [options]);

    const allEggs = useMemo(() => sections.flatMap((section) => section.eggs), [sections]);
    const egg = allEggs.find((item) => item.id === eggId) ?? null;
    const images = eggImages(egg);
    const ports = eggPorts(egg);
    const eggVariables = egg?.variables ?? [];
    const node = options?.nodes.find((item) => String(item.id) === nodeId) ?? null;
    const limits = resourceLimits(node);

    // What is wrong with a variable that was changed. A value left at the
    // egg's default is not checked, here or by the API.
    const variableErrors: Record<string, string> = {};
    for (const variable of eggVariables) {
        const value = variables[variable.environment_variable] ?? variable.default_value ?? '';
        if (value === (variable.default_value ?? '')) continue;
        const problem = variableError(variable, value);
        if (problem) variableErrors[variable.environment_variable] = problem;
    }

    const visibleSections = useMemo(() => {
        const term = eggSearch.trim().toLowerCase();
        if (!term) return sections;
        return sections
            .map((section) => ({
                ...section,
                eggs: section.eggs.filter(
                    (item) => item.name.toLowerCase().includes(term) || section.title.toLowerCase().includes(term),
                ),
            }))
            .filter((section) => section.eggs.length > 0);
    }, [sections, eggSearch]);

    const clearError = (key: string) => {
        setErrors((current) => {
            if (!(key in current)) return current;
            const next = { ...current };
            delete next[key];
            return next;
        });
    };

    const chooseEgg = (chosen: Egg) => {
        if (chosen.id === eggId) return;
        setEggId(chosen.id);
        setImage(eggImages(chosen)[0] ?? '');
        setExtraPorts({});
        // Every variable starts at its default, including empty ones.
        setVariables(Object.fromEntries((chosen.variables ?? []).map((variable) => [variable.environment_variable, variable.default_value ?? ''])));
        clearError('egg');
    };

    const retry = () => {
        setLoadError('');
        setAttempt((current) => current + 1);
    };

    const focusFirstInvalid = () => {
        requestAnimationFrame(() => {
            const target = document.querySelector<HTMLElement>('form [aria-invalid="true"], form [data-invalid="true"]');
            target?.scrollIntoView({ block: 'center', behavior: 'smooth' });
            target?.focus({ preventScroll: true });
        });
    };

    const submit = async (event: FormEvent) => {
        event.preventDefault();
        if (submitting) return;

        const next: Record<string, string> = {};
        const trimmedName = name.trim();
        const ownerValue = wholeNumber(userId);
        const nodeValue = wholeNumber(nodeId);
        const memoryValue = wholeNumber(memory);
        const cpuValue = wholeNumber(cpu);
        const diskValue = wholeNumber(disk);
        const portValue = portNumber(port);

        if (!egg) next.egg = 'Choose the game this server runs.';
        if (!trimmedName) next.name = 'Enter a name for the server.';
        if (ownerValue === null) next.owner = 'Choose who owns the server.';
        if (nodeValue === null) next.node = 'Choose the node the server runs on.';
        if (memoryValue === null || memoryValue < 1) next.memory = 'Enter the memory limit in MB, as a whole number above zero.';
        else if (limits.memory.capacity && memoryValue > limits.memory.free) next.memory = limits.memory.tooMuch;
        if (cpuValue === null || cpuValue < 1) next.cpu = 'Enter the CPU limit as a percentage above zero.';
        if (diskValue === null) next.disk = 'Enter the disk limit in MB, as a whole number.';
        else if (limits.disk.capacity && diskValue > limits.disk.free) next.disk = limits.disk.tooMuch;
        if (portValue === null) next.port = 'Enter a port between 1 and 65535.';

        // Extra ports are optional, but the ones given must be valid and all different.
        const extra: Record<string, number> = {};
        const taken = new Set<number>(portValue === null ? [] : [portValue]);
        for (const item of ports) {
            // A port the game derives from its game port is not a choice.
            if (item.offset) {
                if (portValue !== null) taken.add(portValue + item.offset);
                continue;
            }
            const raw = (extraPorts[item.env] ?? '').trim();
            if (!raw) continue;
            const value = portNumber(raw);
            if (value === null) {
                next[extraKey(item.env)] = 'Enter a port between 1 and 65535, or leave it empty.';
            } else if (taken.has(value)) {
                next[extraKey(item.env)] = `Port ${value} is already used above. Every port must be different.`;
            } else {
                taken.add(value);
                extra[item.env] = value;
            }
        }

        setErrors(next);
        setFormError('');
        if (
            Object.keys(next).length > 0 ||
            Object.keys(variableErrors).length > 0 ||
            !egg ||
            ownerValue === null ||
            nodeValue === null ||
            memoryValue === null ||
            cpuValue === null ||
            diskValue === null ||
            portValue === null
        ) {
            toast.error('Some details are missing or not valid. Check the highlighted fields.');
            focusFirstInvalid();
            return;
        }

        const environment: Record<string, string> = {};
        for (const variable of eggVariables) {
            environment[variable.environment_variable] = variables[variable.environment_variable] ?? variable.default_value ?? '';
        }

        setSubmitting(true);
        try {
            const res = await api.post<Service>('/admin/services', {
                name: trimmedName,
                node_id: nodeValue,
                egg_id: egg.id,
                user_id: ownerValue,
                memory: memoryValue,
                disk: diskValue,
                cpu: cpuValue,
                port: portValue,
                environment: JSON.stringify(environment),
                docker_image: image,
                extra_ports: extra,
            });
            toast.success(`${trimmedName} was created and is now installing.`);
            navigate(res.data?.uuid ? `/services/${res.data.uuid}` : '/admin/services');
        } catch (err) {
            const message = errorMessage(err, 'Could not create the server.');
            if (errorStatus(err) === 409) {
                // Put a "port taken" answer on the port it is about.
                const conflicting = portInMessage(message);
                const env = Object.keys(extra).find((key) => extra[key] === conflicting);
                setErrors({ [env ? extraKey(env) : 'port']: message });
                focusFirstInvalid();
            } else {
                setFormError(message);
            }
            setSubmitting(false);
        }
    };

    const header = (
        <PageHeader
            title="Create server"
            description="Choose a game, who owns the server and where it runs. It starts installing as soon as it is created."
            eyebrow={
                <Link to="/admin/services" className="inline-flex items-center gap-1.5 hover:text-foreground">
                    <ArrowLeft size={16} aria-hidden />
                    All servers
                </Link>
            }
        />
    );

    if (!options) {
        return (
            <>
                {header}
                {loadError ? (
                    <Alert
                        tone="danger"
                        title="This page could not be loaded"
                        actions={
                            <Button size="sm" variant="outline" onClick={retry}>
                                Try again
                            </Button>
                        }
                    >
                        {loadError}
                    </Alert>
                ) : (
                    <LoadingState label="Loading nodes, eggs and users…" />
                )}
            </>
        );
    }

    if (allEggs.length === 0) {
        return (
            <>
                {header}
                <Card flush>
                    <EmptyState
                        icon={<EggIcon size={18} aria-hidden />}
                        title="There are no eggs yet"
                        description="An egg describes how to install and run a game. Import one before creating a server."
                        action={
                            <Link to="/admin/eggs/import" className={primaryLinkClass}>
                                Import an egg
                            </Link>
                        }
                    />
                </Card>
            </>
        );
    }

    if (options.nodes.length === 0) {
        return (
            <>
                {header}
                <Card flush>
                    <EmptyState
                        title="There are no nodes yet"
                        description="A node is the machine that runs servers. Add one before creating a server."
                        action={
                            <Link to="/admin/nodes/create" className={primaryLinkClass}>
                                Add a node
                            </Link>
                        }
                    />
                </Card>
            </>
        );
    }

    return (
        <>
            {header}

            <form onSubmit={submit} className="space-y-6" noValidate>
                {/* 1. Game */}
                <Card>
                    <CardHeader
                        title="Game"
                        description={egg ? `${egg.name} is selected.` : 'The egg decides how the server is installed and started.'}
                    />

                    {allEggs.length > 6 && (
                        <Field label="Find a game" className="mb-5 max-w-sm">
                            {(field) => (
                                <Input
                                    {...field}
                                    type="search"
                                    placeholder="Search by game or nest"
                                    value={eggSearch}
                                    onChange={(event) => setEggSearch(event.target.value)}
                                />
                            )}
                        </Field>
                    )}

                    {errors.egg && (
                        <p role="alert" className="mb-4 text-sm text-danger">
                            {errors.egg}
                        </p>
                    )}

                    {visibleSections.length === 0 ? (
                        <p className="text-sm text-muted">No game matches “{eggSearch.trim()}”.</p>
                    ) : (
                        <div role="radiogroup" aria-label="Game" className="space-y-5">
                            {visibleSections.map((section) => (
                                <div key={section.key}>
                                    <h3 className="mb-2 text-sm font-medium text-muted">{section.title}</h3>
                                    <div className="grid gap-2 sm:grid-cols-2 xl:grid-cols-3">
                                        {section.eggs.map((item) => {
                                            const selected = item.id === eggId;
                                            return (
                                                <button
                                                    key={item.id}
                                                    type="button"
                                                    role="radio"
                                                    aria-checked={selected}
                                                    data-invalid={errors.egg ? true : undefined}
                                                    onClick={() => chooseEgg(item)}
                                                    className={cn(
                                                        'flex min-w-0 items-start gap-3 rounded-control border p-3 text-left transition-colors',
                                                        selected
                                                            ? 'border-primary bg-primary/10 ring-1 ring-primary'
                                                            : 'border-border hover:border-border-strong hover:bg-secondary',
                                                    )}
                                                >
                                                    <span
                                                        className={cn(
                                                            'mt-0.5 flex h-4 w-4 shrink-0 items-center justify-center rounded-full border',
                                                            selected ? 'border-primary bg-primary text-primary-foreground' : 'border-border-strong',
                                                        )}
                                                        aria-hidden
                                                    >
                                                        {selected && <Check size={12} strokeWidth={3} />}
                                                    </span>
                                                    <span className="min-w-0">
                                                        <span className="block truncate text-sm font-medium text-foreground">{item.name}</span>
                                                        <span className="mt-0.5 line-clamp-2 block text-xs text-muted">
                                                            {item.description || 'No description.'}
                                                        </span>
                                                    </span>
                                                </button>
                                            );
                                        })}
                                    </div>
                                </div>
                            ))}
                        </div>
                    )}
                </Card>

                {/* 2. Details */}
                <Card>
                    <CardHeader title="Details" description="What the server is called, who owns it and which node runs it." />
                    <div className="grid gap-4 md:grid-cols-3">
                        <Field label="Server name" error={errors.name}>
                            {(field) => (
                                <Input
                                    {...field}
                                    value={name}
                                    maxLength={255}
                                    placeholder="Survival world"
                                    onChange={(event) => {
                                        setName(event.target.value);
                                        clearError('name');
                                    }}
                                />
                            )}
                        </Field>
                        <Field label="Owner" hint="The owner can manage the server and invite others." error={errors.owner}>
                            {(field) => (
                                <Select
                                    {...field}
                                    value={userId}
                                    onChange={(event) => {
                                        setUserId(event.target.value);
                                        clearError('owner');
                                    }}
                                >
                                    <option value="">Choose an owner</option>
                                    {options.users.map((user: User) => (
                                        <option key={user.id} value={user.id}>
                                            {user.username}
                                            {user.is_admin ? ' (administrator)' : ''}
                                        </option>
                                    ))}
                                </Select>
                            )}
                        </Field>
                        <Field
                            label="Node"
                            error={errors.node}
                            hint={
                                node && !node.is_online ? (
                                    <span className="text-warning">
                                        This node is offline. The server cannot be created until the node is back online.
                                    </span>
                                ) : node && node.total_ram > 0 ? (
                                    `${formatMB(node.total_ram)} memory and ${formatMB(node.total_disk)} disk in total.`
                                ) : undefined
                            }
                        >
                            {(field) => (
                                <Select
                                    {...field}
                                    value={nodeId}
                                    onChange={(event) => {
                                        setNodeId(event.target.value);
                                        clearError('node');
                                    }}
                                >
                                    <option value="">Choose a node</option>
                                    {options.nodes.map((item) => (
                                        <option key={item.id} value={item.id}>
                                            {item.name} — {item.is_online ? 'online' : 'offline'}
                                        </option>
                                    ))}
                                </Select>
                            )}
                        </Field>
                    </div>
                </Card>

                {/* 3. Resources */}
                <Card>
                    <CardHeader title="Resources" description="The most this server may use on its node." />
                    <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
                        <ResourceSlider
                            label="Memory"
                            unit="MB"
                            value={memory}
                            onChange={(value) => {
                                setMemory(value);
                                clearError('memory');
                            }}
                            min={256}
                            max={limits.memory.max}
                            step={256}
                            format={formatMB}
                            capacity={limits.memory.capacity}
                            hint={limits.memory.hint}
                            error={errors.memory}
                        />
                        <ResourceSlider
                            label="CPU"
                            unit="%"
                            value={cpu}
                            onChange={(value) => {
                                setCpu(value);
                                clearError('cpu');
                            }}
                            min={25}
                            max={limits.cpu.max}
                            step={25}
                            format={formatCpu}
                            hint={limits.cpu.hint}
                            error={errors.cpu}
                        />
                        <ResourceSlider
                            label="Disk"
                            unit="MB"
                            value={disk}
                            onChange={(value) => {
                                setDisk(value);
                                clearError('disk');
                            }}
                            min={1024}
                            max={limits.disk.max}
                            step={1024}
                            format={formatMB}
                            capacity={limits.disk.capacity}
                            hint={limits.disk.hint}
                            error={errors.disk}
                        />
                        <Field
                            label="Docker image"
                            className="sm:col-span-2 lg:col-span-3"
                            hint={
                                !egg
                                    ? 'Choose a game to see its images.'
                                    : images.length === 0
                                      ? 'This egg lists no Docker images, so the server cannot be created. Add one on the Eggs page.'
                                      : 'The environment the game runs in. The first one is the egg\'s default.'
                            }
                        >
                            {(field) => (
                                <Select
                                    {...field}
                                    value={image}
                                    disabled={images.length === 0}
                                    onChange={(event) => setImage(event.target.value)}
                                    className="font-mono text-xs"
                                >
                                    {images.length === 0 && <option value="">{egg ? 'No images' : 'Choose a game first'}</option>}
                                    {images.map((option) => (
                                        <option key={option} value={option}>
                                            {option}
                                        </option>
                                    ))}
                                </Select>
                            )}
                        </Field>
                    </div>
                </Card>

                {/* 4. Network */}
                <Card>
                    <CardHeader title="Network" description="Ports are opened on the node. Each one can be used by only one server." />
                    <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
                        <Field label="Primary port" hint="The port players connect to." error={errors.port}>
                            {(field) => (
                                <Input
                                    {...field}
                                    inputMode="numeric"
                                    placeholder="For example 25565"
                                    value={port}
                                    onChange={(event) => {
                                        setPort(event.target.value);
                                        clearError('port');
                                    }}
                                />
                            )}
                        </Field>
                        {ports.map((item) => {
                            const hasDefault = typeof item.default === 'number' && item.default > 0;
                            if (item.offset) {
                                const gamePort = portNumber(port);
                                return (
                                    <Field
                                        key={item.env}
                                        label={`${item.name || item.env} port`}
                                        aside={protocolLabel(item.protocol)}
                                        hint={`This game always uses its game port plus ${item.offset} here, so that port must be free too.`}
                                    >
                                        {(field) => (
                                            <Input
                                                {...field}
                                                disabled
                                                value={gamePort === null ? '' : String(gamePort + item.offset!)}
                                                placeholder={`Game port plus ${item.offset}`}
                                            />
                                        )}
                                    </Field>
                                );
                            }
                            return (
                                <Field
                                    key={item.env}
                                    label={`${item.name || item.env} port`}
                                    aside={`${protocolLabel(item.protocol)} · Optional`}
                                    hint="Leave empty to have a free port chosen for you."
                                    error={errors[extraKey(item.env)]}
                                >
                                    {(field) => (
                                        <Input
                                            {...field}
                                            inputMode="numeric"
                                            placeholder={hasDefault ? `Automatic (default ${item.default})` : 'Automatic'}
                                            value={extraPorts[item.env] ?? ''}
                                            onChange={(event) => {
                                                setExtraPorts((current) => ({ ...current, [item.env]: event.target.value }));
                                                clearError(extraKey(item.env));
                                            }}
                                        />
                                    )}
                                </Field>
                            );
                        })}
                    </div>
                    {!egg && <p className="mt-4 text-sm text-muted">Some games need more ports. They appear here once you choose a game.</p>}
                </Card>

                {/* 5. Variables */}
                <Card>
                    <CardHeader
                        title="Variables"
                        description="Settings the game is started with. You can change all of them; the owner can only change the ones that are not locked."
                    />
                    {!egg ? (
                        <p className="text-sm text-muted">Choose a game to see its variables.</p>
                    ) : eggVariables.length === 0 ? (
                        <p className="text-sm text-muted">{egg.name} has no variables.</p>
                    ) : (
                        <div className="grid gap-x-4 gap-y-5 md:grid-cols-2">
                            {eggVariables.map((variable) => (
                                <Field
                                    key={variable.environment_variable}
                                    label={variable.name || variable.environment_variable}
                                    aside={
                                        variable.user_editable ? undefined : (
                                            <span className="inline-flex items-center gap-1">
                                                <Lock size={12} aria-hidden />
                                                Locked for the owner
                                            </span>
                                        )
                                    }
                                    error={variableErrors[variable.environment_variable]}
                                    hint={
                                        <>
                                            {variable.description && <span className="block">{variable.description}</span>}
                                            <span className="block break-all font-mono text-subtle">{variable.environment_variable}</span>
                                        </>
                                    }
                                >
                                    {(field) => (
                                        <VariableInput
                                            {...field}
                                            variable={variable}
                                            value={variables[variable.environment_variable] ?? variable.default_value ?? ''}
                                            original={variable.default_value ?? ''}
                                            onChange={(next) =>
                                                setVariables((current) => ({
                                                    ...current,
                                                    [variable.environment_variable]: next,
                                                }))
                                            }
                                        />
                                    )}
                                </Field>
                            ))}
                        </div>
                    )}
                </Card>

                {formError && (
                    <Alert tone="danger" title="The server could not be created">
                        {formError}
                    </Alert>
                )}

                <div className="flex flex-wrap items-center justify-end gap-2">
                    <Button variant="ghost" onClick={() => navigate('/admin/services')} disabled={submitting}>
                        Cancel
                    </Button>
                    <Button type="submit" variant="primary" loading={submitting}>
                        Create server
                    </Button>
                </div>
            </form>
        </>
    );
}
