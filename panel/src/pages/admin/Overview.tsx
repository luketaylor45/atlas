import { useState } from 'react';
import { Link } from 'react-router-dom';
import { MemoryStick, Server, Users, Waypoints } from 'lucide-react';
import {
    Alert,
    Badge,
    Button,
    Card,
    CardHeader,
    EmptyState,
    LoadingState,
    NodeStatusBadge,
    PageHeader,
    StatTile,
} from '../../components/ui';
import api from '../../lib/api';
import { errorMessage } from '../../lib/errors';
import { formatDateTime, formatMB, formatRelative } from '../../lib/format';
import type { ActivityLog, Node } from '../../lib/types';
import { usePolling } from '../../lib/usePolling';
import { NodeUsage } from './shared/NodeUsage';

interface Overview {
    users: number;
    services: number;
    nodes_total: number;
    nodes_healthy: number;
    total_memory: number;
    total_disk: number;
    total_cpu: number;
    nodes: Node[] | null;
    logs: ActivityLog[] | null;
    system_health: string;
}

function actionLabel(action: string): string {
    const text = (action || 'event').replace(/[_-]+/g, ' ').trim();
    return text.charAt(0).toUpperCase() + text.slice(1);
}

export default function AdminOverviewPage() {
    const [overview, setOverview] = useState<Overview | null>(null);
    const [error, setError] = useState<string | null>(null);

    const load = async () => {
        try {
            const res = await api.get<Overview>('/admin/overview');
            setOverview(res.data);
            setError(null);
        } catch (err) {
            setError(errorMessage(err, 'Could not load the overview.'));
        }
    };

    usePolling(load, 15000);

    const retry = () => {
        setError(null);
        void load();
    };

    const header = <PageHeader title="Overview" description="Your servers, users and nodes at a glance." />;

    if (!overview) {
        return (
            <>
                {header}
                {error ? (
                    <Alert tone="danger" title="Could not load the overview" actions={<Button size="sm" onClick={retry}>Try again</Button>}>
                        {error}
                    </Alert>
                ) : (
                    <LoadingState label="Loading overview…" />
                )}
            </>
        );
    }

    const nodes = overview.nodes ?? [];
    const logs = overview.logs ?? [];
    const nodesTotal = Number(overview.nodes_total) || 0;
    const nodesOnline = Number(overview.nodes_healthy) || 0;
    const nodesOffline = Math.max(0, nodesTotal - nodesOnline);

    return (
        <>
            {header}

            <div className="space-y-6">
                {error && (
                    <Alert tone="warning" title="Could not refresh" actions={<Button size="sm" onClick={retry}>Try again</Button>}>
                        {error} Showing the last figures received.
                    </Alert>
                )}

                <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">
                    <StatTile label="Servers" value={Number(overview.services) || 0} icon={<Server size={16} aria-hidden />} />
                    <StatTile label="Users" value={Number(overview.users) || 0} icon={<Users size={16} aria-hidden />} />
                    <StatTile
                        label="Nodes online"
                        value={`${nodesOnline} of ${nodesTotal}`}
                        detail={
                            nodesTotal === 0 ? (
                                'No nodes yet'
                            ) : nodesOffline > 0 ? (
                                <span className="text-danger">{nodesOffline} offline</span>
                            ) : (
                                'All nodes are online'
                            )
                        }
                        icon={<Waypoints size={16} aria-hidden />}
                    />
                    <StatTile
                        label="Memory allocated"
                        value={formatMB(overview.total_memory)}
                        detail={`${formatMB(overview.total_disk)} of disk allocated to servers`}
                        icon={<MemoryStick size={16} aria-hidden />}
                    />
                </div>

                <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
                    <Card flush>
                        <CardHeader
                            title="Nodes"
                            description="Usage as last reported by each node."
                            className="mb-0 px-5 pt-5 sm:px-6 sm:pt-6"
                            actions={
                                <Link to="/admin/nodes" className="text-sm font-medium text-primary-text hover:underline">
                                    Manage nodes
                                </Link>
                            }
                        />
                        {nodes.length === 0 ? (
                            <EmptyState
                                icon={<Waypoints size={18} aria-hidden />}
                                title="No nodes yet"
                                description="A node is a machine that runs your servers. Add one to get started."
                                action={
                                    <Link to="/admin/nodes/create" className="text-sm font-medium text-primary-text hover:underline">
                                        Add a node
                                    </Link>
                                }
                            />
                        ) : (
                            <ul className="mt-4 divide-y divide-border border-t border-border">
                                {nodes.map((node) => (
                                    <li key={node.id} className="px-5 py-4 sm:px-6">
                                        <div className="flex flex-wrap items-center justify-between gap-x-3 gap-y-1">
                                            <div className="min-w-0">
                                                <span className="block truncate text-sm font-medium text-foreground" title={node.name}>
                                                    {node.name}
                                                </span>
                                                {node.location && node.location !== 'Unknown' && (
                                                    <span className="block truncate text-xs text-muted">{node.location}</span>
                                                )}
                                            </div>
                                            <NodeStatusBadge online={node.is_online} />
                                        </div>
                                        <NodeUsage node={node} className="mt-3 sm:grid-cols-2 sm:gap-6" />
                                    </li>
                                ))}
                            </ul>
                        )}
                    </Card>

                    <Card flush>
                        <CardHeader
                            title="Recent activity"
                            description="The latest actions taken in the panel."
                            className="mb-0 px-5 pt-5 sm:px-6 sm:pt-6"
                        />
                        {logs.length === 0 ? (
                            <EmptyState
                                title="No activity yet"
                                description="Actions such as creating a server or adding a user will appear here."
                            />
                        ) : (
                            <ul className="mt-4 divide-y divide-border border-t border-border">
                                {logs.map((log) => (
                                    <li key={log.id} className="flex items-start gap-3 px-5 py-3.5 sm:px-6">
                                        <Badge className="mt-0.5 shrink-0">{actionLabel(log.action)}</Badge>
                                        <div className="min-w-0 flex-1">
                                            <p className="break-words text-sm text-foreground">
                                                {log.description || `${actionLabel(log.action)} ${log.resource || ''}`.trim()}
                                            </p>
                                            <time
                                                dateTime={log.created_at}
                                                title={formatDateTime(log.created_at)}
                                                className="mt-0.5 block text-xs text-muted"
                                            >
                                                {formatRelative(log.created_at)}
                                            </time>
                                        </div>
                                    </li>
                                ))}
                            </ul>
                        )}
                    </Card>
                </div>
            </div>
        </>
    );
}
