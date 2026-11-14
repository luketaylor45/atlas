import { Link } from 'react-router-dom';
import { ChevronRight, Plus, Server } from 'lucide-react';
import { formatCpu, formatMB } from '../lib/format';
import type { Service } from '../lib/types';
import { EmptyState, ProgressBar, StatusBadge } from './ui';

/** The address players connect to: the node's public address and the game port. */
function serviceAddress(service: Service): string {
    const port = service.ports?.find((entry) => entry.primary)?.port ?? service.port;
    return service.node?.address ? `${service.node.address}:${port}` : `Port ${port}`;
}

interface ServiceCardProps {
    service: Service;
    /** Leaves out the memory and CPU limits, for short lists. */
    compact?: boolean;
}

/**
 * One server as a list row linking to its page. Used inside a `<ul>` with
 * `divide-y divide-border`.
 */
export default function ServiceCard({ service, compact = false }: ServiceCardProps) {
    const installing = service.status === 'installing';

    return (
        <li>
            <Link
                to={`/services/${service.uuid}`}
                className="flex items-center gap-4 px-5 py-4 transition-colors hover:bg-secondary/60 focus-visible:-outline-offset-2 sm:px-6"
            >
                <div className="min-w-0 flex-1">
                    <div className="flex flex-wrap items-center gap-x-3 gap-y-1.5">
                        <span className="min-w-0 truncate text-sm font-medium text-foreground">{service.name}</span>
                        <StatusBadge status={service.status} suspended={service.is_suspended} />
                    </div>

                    <div className="mt-1.5 flex flex-wrap gap-x-4 gap-y-1 text-xs text-muted">
                        {service.egg?.name && <span>{service.egg.name}</span>}
                        <span className="break-all font-mono">{serviceAddress(service)}</span>
                        {!compact && (
                            <>
                                <span>{formatMB(service.memory)} memory</span>
                                <span>{formatCpu(service.cpu)}</span>
                            </>
                        )}
                    </div>

                    {installing && (
                        <div className="mt-3 max-w-md">
                            <ProgressBar value={service.installation_progress} label={`Installing ${service.name}`} />
                            <p className="mt-1.5 text-xs text-muted">
                                {service.installation_stage || 'Installing'}
                                <span className="tabular"> · {Math.round(Number(service.installation_progress) || 0)}%</span>
                            </p>
                        </div>
                    )}
                </div>

                <ChevronRight size={16} className="shrink-0 text-subtle" aria-hidden />
            </Link>
        </li>
    );
}

/** What a server list shows when the person has no servers yet. */
export function NoServers({ isAdmin }: { isAdmin: boolean }) {
    return (
        <EmptyState
            icon={<Server size={18} aria-hidden />}
            title="No servers yet"
            description={
                isAdmin
                    ? 'Create a server and it will appear here.'
                    : 'Servers are created by an administrator. Once one is assigned to you, it will appear here.'
            }
            action={
                isAdmin && (
                    <Link
                        to="/admin/services/create"
                        className="inline-flex h-9 items-center justify-center gap-2 rounded-control bg-primary px-4 text-sm font-medium text-primary-foreground shadow-card transition-colors hover:bg-primary-hover"
                    >
                        <Plus size={16} aria-hidden />
                        Create server
                    </Link>
                )
            }
        />
    );
}
