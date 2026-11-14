import { useMemo, useState } from 'react';
import { Link } from 'react-router-dom';
import { isAxiosError } from 'axios';
import { ExternalLink, Pencil, Plus, Search, Server, Trash2 } from 'lucide-react';
import api from '../../lib/api';
import { errorMessage } from '../../lib/errors';
import { formatCpu, formatMB } from '../../lib/format';
import type { Service } from '../../lib/types';
import { usePolling } from '../../lib/usePolling';
import {
    Alert,
    Button,
    Card,
    EmptyState,
    IconButton,
    Input,
    LoadingState,
    PageHeader,
    StatusBadge,
    Table,
    Td,
    Th,
    Tr,
    useConfirm,
    useToast,
} from '../../components/ui';
import { ServiceEditModal } from './shared/ServiceEditModal';
import { iconLinkClass, primaryLinkClass } from './shared/ServiceForm';

function canForceDelete(error: unknown): boolean {
    if (!isAxiosError(error)) return false;
    const data = error.response?.data as { can_force?: unknown } | undefined;
    return data?.can_force === true;
}

function matches(service: Service, term: string): boolean {
    return [service.name, service.uuid, service.user?.username, service.egg?.name].some((value) =>
        (value ?? '').toLowerCase().includes(term),
    );
}

export default function AdminServicesPage() {
    const toast = useToast();
    const confirm = useConfirm();

    const [services, setServices] = useState<Service[] | null>(null);
    const [error, setError] = useState('');
    const [search, setSearch] = useState('');
    const [editing, setEditing] = useState<Service | null>(null);
    const [deletingId, setDeletingId] = useState<number | null>(null);

    const load = async () => {
        try {
            const res = await api.get<Service[] | null>('/admin/services');
            setServices(Array.isArray(res.data) ? res.data : []);
            setError('');
        } catch (err) {
            setError(errorMessage(err, 'Could not load the servers.'));
        }
    };

    usePolling(load, 10000);

    const visible = useMemo(() => {
        const term = search.trim().toLowerCase();
        const list = [...(services ?? [])].sort((a, b) => a.name.localeCompare(b.name));
        return term ? list.filter((service) => matches(service, term)) : list;
    }, [services, search]);

    const retry = () => {
        setError('');
        void load();
    };

    const remove = async (service: Service) => {
        const confirmed = await confirm({
            title: `Delete ${service.name}?`,
            description: 'All of its files are permanently deleted, along with its settings and ports. This cannot be undone.',
            confirmLabel: 'Delete server',
            danger: true,
            requireText: service.name,
        });
        if (!confirmed) return;

        setDeletingId(service.id);
        try {
            try {
                await api.delete(`/admin/services/${service.id}`);
            } catch (err) {
                if (!canForceDelete(err)) throw err;

                const forced = await confirm({
                    title: 'Remove it from the panel anyway?',
                    description: (
                        <>
                            {errorMessage(err, 'The node could not remove this server.')}
                            <span className="mt-2 block">
                                Removing it from the panel anyway will leave its container and files on the node, where they have to
                                be cleaned up by hand. Only do this if the node is gone for good.
                            </span>
                        </>
                    ),
                    confirmLabel: 'Remove from panel',
                    danger: true,
                });
                if (!forced) return;
                await api.delete(`/admin/services/${service.id}`, { params: { force: true } });
            }
            setServices((current) => current?.filter((item) => item.id !== service.id) ?? current);
            toast.success(`${service.name} was deleted.`);
        } catch (err) {
            toast.error(errorMessage(err, 'Could not delete the server.'));
        } finally {
            setDeletingId(null);
        }
    };

    const saved = (updated: Service, warning?: string) => {
        setEditing(null);
        // The response does not carry every relation, so only the edited fields are merged before the refresh.
        const { name, memory, cpu, disk, port, docker_image } = updated;
        setServices(
            (current) =>
                current?.map((item) => (item.id === updated.id ? { ...item, name, memory, cpu, disk, port, docker_image } : item)) ??
                current,
        );
        toast.success(`${updated.name} was saved.`);
        if (warning) toast.info(warning);
        void load();
    };

    const total = services?.length ?? 0;

    return (
        <>
            <PageHeader
                title="All servers"
                description="Every server on every node. Change a server's limits, port or image, or delete it."
                actions={
                    <Link to="/admin/services/create" className={primaryLinkClass}>
                        <Plus size={16} aria-hidden />
                        Create server
                    </Link>
                }
            />

            {services === null && !error && <LoadingState label="Loading servers…" />}

            {error && (
                <Alert
                    tone="danger"
                    title={services === null ? 'The servers could not be loaded' : 'The list could not be refreshed'}
                    className="mb-4"
                    actions={
                        <Button size="sm" variant="outline" onClick={retry}>
                            Try again
                        </Button>
                    }
                >
                    {error}
                </Alert>
            )}

            {services !== null && total === 0 && (
                <Card flush>
                    <EmptyState
                        icon={<Server size={18} aria-hidden />}
                        title="No servers yet"
                        description="Create the first server by choosing a game, an owner and a node."
                        action={
                            <Link to="/admin/services/create" className={primaryLinkClass}>
                                <Plus size={16} aria-hidden />
                                Create server
                            </Link>
                        }
                    />
                </Card>
            )}

            {services !== null && total > 0 && (
                <Card flush>
                    <div className="flex flex-wrap items-center justify-between gap-3 border-b border-border p-4 sm:px-5">
                        <div className="relative w-full sm:max-w-xs">
                            <Search
                                size={16}
                                className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-subtle"
                                aria-hidden
                            />
                            <Input
                                type="search"
                                aria-label="Search servers"
                                placeholder="Search by name, ID, owner or game"
                                value={search}
                                onChange={(event) => setSearch(event.target.value)}
                                className="pl-9"
                            />
                        </div>
                        <p className="tabular text-sm text-muted">
                            {visible.length === total ? `${total} ${total === 1 ? 'server' : 'servers'}` : `${visible.length} of ${total}`}
                        </p>
                    </div>

                    {visible.length === 0 ? (
                        <EmptyState
                            icon={<Search size={18} aria-hidden />}
                            title="No servers match your search"
                            description="Search looks at the name, ID, owner and game."
                            action={
                                <Button variant="outline" onClick={() => setSearch('')}>
                                    Clear search
                                </Button>
                            }
                        />
                    ) : (
                        <Table>
                            <thead>
                                <tr>
                                    <Th>Server</Th>
                                    <Th>Status</Th>
                                    <Th>Owner</Th>
                                    <Th>Game</Th>
                                    <Th>Node</Th>
                                    <Th>Address</Th>
                                    <Th>Limits</Th>
                                    <Th className="text-right">Actions</Th>
                                </tr>
                            </thead>
                            <tbody>
                                {visible.map((service) => (
                                    <Tr key={service.id}>
                                        <Td>
                                            <Link
                                                to={`/services/${service.uuid}`}
                                                className="block max-w-56 truncate font-medium text-foreground hover:text-primary-text hover:underline"
                                            >
                                                {service.name}
                                            </Link>
                                            <span className="block font-mono text-xs text-subtle" title={service.uuid}>
                                                {service.uuid.slice(0, 8)}
                                            </span>
                                        </Td>
                                        <Td>
                                            <StatusBadge status={service.status} suspended={service.is_suspended} />
                                        </Td>
                                        <Td className="whitespace-nowrap">{service.user?.username ?? <Missing />}</Td>
                                        <Td className="whitespace-nowrap">{service.egg?.name ?? <Missing />}</Td>
                                        <Td className="whitespace-nowrap">
                                            {service.node ? (
                                                <span className="inline-flex items-center gap-2">
                                                    {service.node.name}
                                                    {!service.node.is_online && <span className="text-xs text-danger">Offline</span>}
                                                </span>
                                            ) : (
                                                <Missing />
                                            )}
                                        </Td>
                                        <Td className="whitespace-nowrap font-mono text-xs text-muted">
                                            {service.node?.address ? `${service.node.address}:${service.port}` : `:${service.port}`}
                                        </Td>
                                        <Td className="tabular whitespace-nowrap text-xs text-muted">
                                            <div>
                                                {formatMB(service.memory)} memory · {formatCpu(service.cpu)}
                                            </div>
                                            <div>{formatMB(service.disk)} disk</div>
                                        </Td>
                                        <Td>
                                            <div className="flex items-center justify-end gap-1">
                                                <Link
                                                    to={`/services/${service.uuid}`}
                                                    className={iconLinkClass}
                                                    aria-label={`Open ${service.name}`}
                                                    title="Open"
                                                >
                                                    <ExternalLink size={16} aria-hidden />
                                                </Link>
                                                <IconButton label={`Edit ${service.name}`} size="sm" onClick={() => setEditing(service)}>
                                                    <Pencil size={16} aria-hidden />
                                                </IconButton>
                                                <IconButton
                                                    label={`Delete ${service.name}`}
                                                    size="sm"
                                                    className="hover:bg-danger/10 hover:text-danger"
                                                    loading={deletingId === service.id}
                                                    onClick={() => void remove(service)}
                                                >
                                                    <Trash2 size={16} aria-hidden />
                                                </IconButton>
                                            </div>
                                        </Td>
                                    </Tr>
                                ))}
                            </tbody>
                        </Table>
                    )}
                </Card>
            )}

            {editing && <ServiceEditModal key={editing.id} service={editing} onClose={() => setEditing(null)} onSaved={saved} />}
        </>
    );
}

function Missing() {
    return <span className="text-subtle">—</span>;
}
