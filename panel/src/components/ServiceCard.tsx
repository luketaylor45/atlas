import type { ReactNode } from 'react';
import { Link } from 'react-router-dom';
import { AlertTriangle, ChevronRight, Cpu, HardDrive, MapPin, MemoryStick, Plus } from 'lucide-react';
import { formatCpu, formatMB } from '../lib/format';
import { gameIdentity } from '../lib/gameIdentity';
import type { Service, ServiceStats } from '../lib/types';
import { useLiveUsage } from '../lib/useLiveUsage';
import { cn } from '../lib/cn';
import GameCover, { GameIcon } from './GameCover';
import OrbitArt from './OrbitArt';
import { ProgressBar, StatusBadge } from './ui';

/** The address players connect to: the node's public address and the game port. */
function serviceAddress(service: Service): string {
    const port = service.ports?.find((entry) => entry.primary)?.port ?? service.port;
    return service.node?.address ? `${service.node.address}:${port}` : `Port ${port}`;
}

function finite(value: unknown): number | null {
    if (value === null || value === undefined || value === '') return null;
    const parsed = Number(value);
    return Number.isFinite(parsed) ? parsed : null;
}

interface Usage {
    cpu: { value: number; percent: number; label: string } | null;
    memory: { value: number; percent: number; label: string } | null;
}

/** CPU in cores and memory in MB, against the server's limits. */
function usageOf(service: Service, stats: ServiceStats | null): Usage {
    const cpu = finite(stats?.cpu);
    const memory = finite(stats?.memory);
    const cpuLimit = finite(service.cpu) ?? 0;
    const memoryLimit = finite(service.memory) ?? 0;
    const cores = (value: number) => (value / 100).toFixed(1);

    return {
        cpu:
            cpu === null
                ? null
                : {
                      value: cpu,
                      percent: (cpu / (cpuLimit > 0 ? cpuLimit : 100)) * 100,
                      label: cpuLimit > 0 ? `${cores(cpu)} of ${formatCpu(cpuLimit)}` : `${cores(cpu)} cores`,
                  },
        memory:
            memory === null
                ? null
                : {
                      value: memory,
                      percent: memoryLimit > 0 ? (memory / memoryLimit) * 100 : 0,
                      label: memoryLimit > 0 ? `${formatMB(memory)} of ${formatMB(memoryLimit)}` : formatMB(memory),
                  },
    };
}

function UsageBar({ label, icon, usage }: { label: string; icon: ReactNode; usage: NonNullable<Usage['cpu']> }) {
    return (
        <div className="min-w-0">
            <div className="mb-1.5 flex items-center justify-between gap-2 text-xs">
                <span className="inline-flex items-center gap-1.5 text-muted">
                    {icon}
                    {label}
                </span>
                <span className="tabular truncate font-medium text-foreground">{usage.label}</span>
            </div>
            <ProgressBar label={`${label} used`} value={usage.percent} tone={usage.percent > 90 ? 'warning' : 'primary'} />
        </div>
    );
}

/** The limits a server was given, for when it is not running. */
function Allocation({ service, className }: { service: Service; className?: string }) {
    return (
        <div className={cn('flex flex-wrap gap-x-4 gap-y-1.5 text-xs text-muted', className)}>
            <span className="inline-flex items-center gap-1.5">
                <MemoryStick size={13} aria-hidden />
                {formatMB(service.memory)}
            </span>
            <span className="inline-flex items-center gap-1.5">
                <Cpu size={13} aria-hidden />
                {formatCpu(service.cpu)}
            </span>
            {Number(service.disk) > 0 && (
                <span className="inline-flex items-center gap-1.5">
                    <HardDrive size={13} aria-hidden />
                    {formatMB(service.disk)} disk
                </span>
            )}
        </div>
    );
}

/** Install progress, an install failure, live usage or the limits: whichever fits the state. */
function ServerBody({ service, stats, dense = false }: { service: Service; stats: ServiceStats | null; dense?: boolean }) {
    if (service.status === 'installing') {
        const progress = Math.round(Number(service.installation_progress) || 0);
        return (
            <div>
                <ProgressBar value={progress} label={`Installing ${service.name}`} />
                <p className="mt-1.5 truncate text-xs text-muted">
                    {service.installation_stage || 'Installing'}
                    <span className="tabular"> · {progress}%</span>
                </p>
            </div>
        );
    }

    if (service.status === 'installation_failed') {
        return (
            <p className="flex items-start gap-1.5 text-xs text-danger">
                <AlertTriangle size={14} className="mt-px shrink-0" aria-hidden />
                <span className={dense ? 'truncate' : 'line-clamp-2'}>
                    {service.installation_stage || 'Installation failed. Open the server to try again.'}
                </span>
            </p>
        );
    }

    const usage = usageOf(service, stats);
    if (usage.cpu || usage.memory) {
        return (
            <div className={cn('grid gap-3', dense ? 'grid-cols-2' : 'grid-cols-1')}>
                {usage.cpu && <UsageBar label="CPU" icon={<Cpu size={13} aria-hidden />} usage={usage.cpu} />}
                {usage.memory && <UsageBar label="Memory" icon={<MemoryStick size={13} aria-hidden />} usage={usage.memory} />}
            </div>
        );
    }

    return <Allocation service={service} />;
}

function isUp(service: Service) {
    return !service.is_suspended && (service.status === 'running' || service.status === 'starting');
}

/**
 * One server as a card with a generated cover, linking to its page. Used in
 * a `<ul>` grid.
 */
export default function ServerTile({ service }: { service: Service }) {
    const identity = gameIdentity(service);
    const { track, stats } = useLiveUsage(service.uuid, isUp(service));
    const location = [service.node?.name, service.node?.location].filter(Boolean).join(' · ');

    return (
        <li ref={track} className="flex">
            <Link
                to={`/services/${service.uuid}`}
                className="surface group flex w-full flex-col overflow-hidden rounded-card border border-border bg-card shadow-card transition-[border-color,box-shadow,transform] duration-200 hover:-translate-y-0.5 hover:border-border-strong hover:shadow-pop"
            >
                <GameCover identity={identity} className="h-[var(--cover-height)] shrink-0">
                    <span className="absolute right-3 top-3 max-w-[60%] truncate rounded-full bg-black/30 px-2.5 py-0.5 text-[11px] font-medium text-white backdrop-blur-sm">
                        {identity.label}
                    </span>
                </GameCover>

                <div className="flex flex-1 flex-col px-4 pb-4">
                    <div className="relative -mt-6 mb-3 flex items-end justify-between gap-3">
                        <GameIcon
                            identity={identity}
                            size={22}
                            className="h-12 w-12 rounded-[calc(var(--card-radius)-2px)] ring-4 ring-card"
                        />
                        <StatusBadge status={service.status} suspended={service.is_suspended} className="mb-0.5" />
                    </div>

                    <h3 className="truncate text-[15px] font-semibold tracking-tight text-foreground" title={service.name}>
                        {service.name}
                    </h3>
                    <p className="mt-0.5 truncate font-mono text-xs text-muted">{serviceAddress(service)}</p>

                    <div className="mb-4 mt-4">
                        <ServerBody service={service} stats={stats} />
                    </div>

                    <div className="mt-auto flex items-center justify-between gap-3 border-t border-border pt-3 text-xs text-muted">
                        <span className="inline-flex min-w-0 items-center gap-1.5">
                            <MapPin size={13} className="shrink-0" aria-hidden />
                            <span className="truncate">{location || 'No node'}</span>
                        </span>
                        <span className="inline-flex items-center gap-0.5 font-medium text-primary-text opacity-0 transition-opacity group-hover:opacity-100 group-focus-visible:opacity-100">
                            Open
                            <ChevronRight size={14} aria-hidden />
                        </span>
                    </div>
                </div>
            </Link>
        </li>
    );
}

/**
 * One server as a dense list row linking to its page. Used inside a `<ul>`
 * with `divide-y divide-border`.
 */
export function ServerRow({ service }: { service: Service }) {
    const identity = gameIdentity(service);
    const { track, stats } = useLiveUsage(service.uuid, isUp(service));

    return (
        <li ref={track}>
            <Link
                to={`/services/${service.uuid}`}
                className="flex items-center gap-4 px-4 py-3 transition-colors hover:bg-secondary/60 focus-visible:-outline-offset-2 sm:px-5"
            >
                <GameIcon identity={identity} size={17} className="h-9 w-9 rounded-control" />

                <div className="grid min-w-0 flex-1 grid-cols-1 items-center gap-x-6 gap-y-2 md:grid-cols-[minmax(0,1.1fr)_minmax(0,1fr)]">
                    <div className="min-w-0">
                        <div className="flex min-w-0 flex-wrap items-center gap-x-2.5 gap-y-1">
                            <span className="min-w-0 truncate text-sm font-medium text-foreground" title={service.name}>
                                {service.name}
                            </span>
                            <StatusBadge status={service.status} suspended={service.is_suspended} />
                        </div>
                        <div className="mt-0.5 flex min-w-0 flex-wrap gap-x-3 text-xs text-muted">
                            <span className="truncate">{identity.label}</span>
                            <span className="truncate font-mono">{serviceAddress(service)}</span>
                        </div>
                    </div>
                    <div className="min-w-0">
                        <ServerBody service={service} stats={stats} dense />
                    </div>
                </div>

                <ChevronRight size={16} className="shrink-0 text-subtle" aria-hidden />
            </Link>
        </li>
    );
}

/** What a server list shows when the person has no servers yet. */
export function NoServers({ isAdmin }: { isAdmin: boolean }) {
    return (
        <div className="flex flex-col items-center justify-center px-6 py-12 text-center">
            <OrbitArt className="mb-2 h-32 w-32 text-primary-text" />
            <p className="text-base font-semibold text-foreground">No servers yet</p>
            <p className="mt-1 max-w-sm text-sm text-muted">
                {isAdmin
                    ? 'Create your first server and it will show up here, ready to start.'
                    : 'Servers are created by an administrator. Once one is assigned to you, it will appear here.'}
            </p>
            {isAdmin && (
                <Link
                    to="/admin/services/create"
                    className="mt-5 inline-flex h-9 items-center justify-center gap-2 rounded-control bg-primary px-4 text-sm font-medium text-primary-foreground shadow-card transition-colors hover:bg-primary-hover"
                >
                    <Plus size={16} aria-hidden />
                    Create server
                </Link>
            )}
        </div>
    );
}
