import { useId, useState, type FormEvent } from 'react';
import { Link } from 'react-router-dom';
import { KeyRound, Pencil, Plus, Trash2, Waypoints } from 'lucide-react';
import {
    Alert,
    Button,
    Card,
    EmptyState,
    IconButton,
    LoadingState,
    Modal,
    NodeStatusBadge,
    PageHeader,
    Table,
    Td,
    Th,
    Tr,
    useConfirm,
    useToast,
} from '../../components/ui';
import api from '../../lib/api';
import { errorMessage } from '../../lib/errors';
import { formatDateTime, formatMB, formatRelative } from '../../lib/format';
import type { Node } from '../../lib/types';
import { usePolling } from '../../lib/usePolling';
import { NodeFormFields } from './shared/NodeFormFields';
import {
    changedNodeFields,
    nodeFormToPayload,
    nodeToForm,
    validateNodeForm,
    type NodeFormErrors,
    type NodeFormValues,
} from './shared/NodeFormState';
import { NodeToken, NodeTokenSteps } from './shared/NodeToken';
import { NodeUsage } from './shared/NodeUsage';

const primaryLink =
    'inline-flex h-9 items-center justify-center gap-2 whitespace-nowrap rounded-control bg-primary px-4 text-sm font-medium ' +
    'text-primary-foreground shadow-card transition-colors hover:bg-primary-hover';

function EditNodeModal({ node, onClose, onSaved }: { node: Node; onClose: () => void; onSaved: (node: Node) => void }) {
    const toast = useToast();
    const formId = useId();
    const [values, setValues] = useState<NodeFormValues>(() => nodeToForm(node));
    const [errors, setErrors] = useState<NodeFormErrors>({});
    const [formError, setFormError] = useState<string | null>(null);
    const [saving, setSaving] = useState(false);

    const change = (field: keyof NodeFormValues, value: string) => {
        setValues((current) => ({ ...current, [field]: value }));
        setErrors((current) => ({ ...current, [field]: undefined }));
    };

    const submit = async (event: FormEvent<HTMLFormElement>) => {
        event.preventDefault();
        if (saving) return;

        const problems = validateNodeForm(values);
        setErrors(problems);
        if (Object.keys(problems).length > 0) return;

        const changes = changedNodeFields(node, nodeFormToPayload(values));
        if (Object.keys(changes).length === 0) {
            onClose();
            return;
        }

        setSaving(true);
        setFormError(null);
        try {
            const res = await api.put<Node>(`/admin/nodes/${node.id}`, changes);
            toast.success(`Saved ${res.data.name}`);
            onSaved(res.data);
        } catch (err) {
            setFormError(errorMessage(err, 'Could not save the node.'));
            setSaving(false);
        }
    };

    const connectionChanged =
        values.address.trim() !== (node.address ?? '') ||
        values.internal_address.trim() !== (node.internal_address ?? '') ||
        values.port.trim() !== String(node.port ?? '');

    return (
        <Modal
            open
            onClose={onClose}
            title={`Edit ${node.name}`}
            size="lg"
            dismissible={!saving}
            footer={
                <>
                    <Button variant="ghost" onClick={onClose} disabled={saving}>
                        Cancel
                    </Button>
                    <Button variant="primary" type="submit" form={formId} loading={saving}>
                        Save changes
                    </Button>
                </>
            }
        >
            <form id={formId} onSubmit={submit} noValidate className="space-y-4">
                {formError && <Alert tone="danger">{formError}</Alert>}
                <NodeFormFields values={values} errors={errors} onChange={change} />
                {connectionChanged && (
                    <Alert tone="warning">
                        The panel will use the new address and port straight away. Make sure the node's daemon can be reached there, or
                        the node will show as offline.
                    </Alert>
                )}
            </form>
        </Modal>
    );
}

export default function AdminNodesPage() {
    const toast = useToast();
    const confirm = useConfirm();
    const [nodes, setNodes] = useState<Node[] | null>(null);
    const [error, setError] = useState<string | null>(null);
    const [editing, setEditing] = useState<Node | null>(null);
    const [busyId, setBusyId] = useState<number | null>(null);
    const [newToken, setNewToken] = useState<{ name: string; token: string } | null>(null);

    const load = async () => {
        try {
            const res = await api.get<Node[]>('/admin/nodes');
            setNodes(Array.isArray(res.data) ? res.data : []);
            setError(null);
        } catch (err) {
            setError(errorMessage(err, 'Could not load nodes.'));
        }
    };

    usePolling(load, 15000);

    const retry = () => {
        setError(null);
        void load();
    };

    const regenerateToken = async (node: Node) => {
        const confirmed = await confirm({
            title: `Regenerate the token for ${node.name}?`,
            description: (
                <>
                    The current token stops working at once and the node goes offline. It stays offline until you put the new token in{' '}
                    <span className="font-mono text-foreground">NODE_TOKEN</span> in the node's{' '}
                    <span className="font-mono text-foreground">.env</span> file and restart its daemon. Servers that are running keep
                    running, but cannot be managed from the panel until then.
                </>
            ),
            confirmLabel: 'Regenerate token',
            danger: true,
        });
        if (!confirmed) return;

        setBusyId(node.id);
        try {
            const res = await api.post<{ token: string }>(`/admin/nodes/${node.id}/token`);
            setNewToken({ name: node.name, token: res.data.token });
            setNodes((current) => current?.map((item) => (item.id === node.id ? { ...item, is_online: false } : item)) ?? current);
        } catch (err) {
            toast.error(errorMessage(err, 'Could not regenerate the token.'));
        } finally {
            setBusyId(null);
        }
    };

    const deleteNode = async (node: Node) => {
        const confirmed = await confirm({
            title: `Delete ${node.name}?`,
            description:
                'The node is removed from the panel and its token stops working. A node that still hosts servers cannot be deleted; delete those servers first.',
            confirmLabel: 'Delete node',
            danger: true,
        });
        if (!confirmed) return;

        setBusyId(node.id);
        try {
            await api.delete(`/admin/nodes/${node.id}`);
            setNodes((current) => current?.filter((item) => item.id !== node.id) ?? current);
            toast.success(`Deleted ${node.name}`);
        } catch (err) {
            toast.error(errorMessage(err, 'Could not delete the node.'));
        } finally {
            setBusyId(null);
        }
    };

    const header = (
        <PageHeader
            title="Nodes"
            description="The machines that run your servers."
            actions={
                <Link to="/admin/nodes/create" className={primaryLink}>
                    <Plus size={16} aria-hidden />
                    Add node
                </Link>
            }
        />
    );

    if (!nodes) {
        return (
            <>
                {header}
                {error ? (
                    <Alert tone="danger" title="Could not load nodes" actions={<Button size="sm" onClick={retry}>Try again</Button>}>
                        {error}
                    </Alert>
                ) : (
                    <LoadingState label="Loading nodes…" />
                )}
            </>
        );
    }

    return (
        <>
            {header}

            <div className="space-y-4">
                {error && (
                    <Alert tone="warning" title="Could not refresh" actions={<Button size="sm" onClick={retry}>Try again</Button>}>
                        {error} Showing the last list received.
                    </Alert>
                )}

                <Card flush>
                    {nodes.length === 0 ? (
                        <EmptyState
                            icon={<Waypoints size={18} aria-hidden />}
                            title="No nodes yet"
                            description="A node is a machine running the Atlas daemon. Add one before you create servers."
                            action={
                                <Link to="/admin/nodes/create" className="text-sm font-medium text-primary-text hover:underline">
                                    Add your first node
                                </Link>
                            }
                        />
                    ) : (
                        <Table className="min-w-[56rem]">
                            <thead>
                                <tr>
                                    <Th>Node</Th>
                                    <Th>Status</Th>
                                    <Th>Address</Th>
                                    <Th>Capacity</Th>
                                    <Th className="w-56">Usage</Th>
                                    <Th className="text-right">Actions</Th>
                                </tr>
                            </thead>
                            <tbody>
                                {nodes.map((node) => (
                                    <Tr key={node.id}>
                                        <Td>
                                            <div className="max-w-56 truncate font-medium text-foreground" title={node.name}>
                                                {node.name}
                                            </div>
                                            <div className="max-w-56 truncate text-xs text-muted">
                                                {node.location && node.location !== 'Unknown' ? node.location : 'No location set'}
                                            </div>
                                        </Td>
                                        <Td>
                                            <NodeStatusBadge online={node.is_online} />
                                            <div className="mt-1 whitespace-nowrap text-xs text-muted" title={formatDateTime(node.last_heartbeat)}>
                                                {formatRelative(node.last_heartbeat) === '—'
                                                    ? 'No heartbeat yet'
                                                    : `Heartbeat ${formatRelative(node.last_heartbeat)}`}
                                            </div>
                                        </Td>
                                        <Td>
                                            <div className="max-w-64 truncate font-mono text-xs text-foreground" title={`${node.address}:${node.port}`}>
                                                {node.address}:{node.port}
                                            </div>
                                            <div className="mt-1 max-w-64 truncate text-xs text-muted">
                                                {node.internal_address ? (
                                                    <>
                                                        Internal: <span className="font-mono">{node.internal_address}</span>
                                                    </>
                                                ) : (
                                                    'Uses public address'
                                                )}
                                                <span className="tabular"> · SFTP {node.sftp_port}</span>
                                            </div>
                                        </Td>
                                        <Td className="whitespace-nowrap text-xs text-muted">
                                            <div>Memory: {node.total_ram > 0 ? formatMB(node.total_ram) : 'no limit'}</div>
                                            <div className="mt-1">Disk: {node.total_disk > 0 ? formatMB(node.total_disk) : 'no limit'}</div>
                                        </Td>
                                        <Td>
                                            <NodeUsage node={node} className="w-48 gap-2" />
                                        </Td>
                                        <Td>
                                            <div className="flex items-center justify-end gap-1">
                                                <IconButton
                                                    label={`Edit ${node.name}`}
                                                    size="sm"
                                                    disabled={busyId === node.id}
                                                    onClick={() => setEditing(node)}
                                                >
                                                    <Pencil size={16} aria-hidden />
                                                </IconButton>
                                                <IconButton
                                                    label={`Regenerate token for ${node.name}`}
                                                    size="sm"
                                                    disabled={busyId === node.id}
                                                    onClick={() => void regenerateToken(node)}
                                                >
                                                    <KeyRound size={16} aria-hidden />
                                                </IconButton>
                                                <IconButton
                                                    label={`Delete ${node.name}`}
                                                    size="sm"
                                                    className="hover:bg-danger/10 hover:text-danger"
                                                    loading={busyId === node.id}
                                                    onClick={() => void deleteNode(node)}
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
            </div>

            {editing && (
                <EditNodeModal
                    key={editing.id}
                    node={editing}
                    onClose={() => setEditing(null)}
                    onSaved={(saved) => {
                        setNodes((current) => current?.map((item) => (item.id === saved.id ? saved : item)) ?? current);
                        setEditing(null);
                    }}
                />
            )}

            <Modal
                open={Boolean(newToken)}
                onClose={() => setNewToken(null)}
                title={`New token for ${newToken?.name ?? 'node'}`}
                description="Copy it now. It is shown only this once and cannot be shown again."
                footer={
                    <Button variant="primary" onClick={() => setNewToken(null)}>
                        I have saved the token
                    </Button>
                }
            >
                {newToken && (
                    <div className="space-y-4">
                        <NodeToken token={newToken.token} />
                        <NodeTokenSteps lastStep="The node shows Online again within a few seconds. Until then it stays offline." />
                    </div>
                )}
            </Modal>
        </>
    );
}
