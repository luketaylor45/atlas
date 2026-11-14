import { useState } from 'react';
import { ArrowDown, ArrowUp, Cpu, MemoryStick, Network } from 'lucide-react';
import api from '../../../lib/api';
import { formatBytes, formatMB } from '../../../lib/format';
import type { Service, ServiceStats } from '../../../lib/types';
import { usePolling } from '../../../lib/usePolling';
import { ProgressBar, StatTile } from '../../../components/ui';

function finite(value: unknown): number | null {
    if (value === null || value === undefined || value === '') return null;
    const parsed = Number(value);
    return Number.isFinite(parsed) ? parsed : null;
}

/** Live CPU, memory and network use. Polled only while the server is starting or running. */
export function ResourceCards({ service }: { service: Service }) {
    // The game's process is up from the moment it starts, well before it shows as running.
    const up = service.status === 'running' || service.status === 'starting';
    const [stats, setStats] = useState<ServiceStats | null>(null);

    usePolling(
        async () => {
            try {
                const res = await api.get<ServiceStats>(`/services/${service.uuid}/stats`);
                setStats(res.data);
            } catch {
                // Usage is best-effort: show dashes until the next poll succeeds.
                setStats(null);
            }
        },
        3000,
        up,
    );

    const current = up ? stats : null;
    const cpu = finite(current?.cpu);
    const memory = finite(current?.memory);
    const rx = finite(current?.network?.rx);
    const tx = finite(current?.network?.tx);
    const memoryLimit = finite(service.memory) ?? 0;
    const cpuLimit = finite(service.cpu) ?? 0;

    return (
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
            <StatTile
                label="CPU"
                icon={<Cpu size={16} aria-hidden />}
                value={cpu === null ? '—' : `${cpu.toFixed(1)}%`}
                detail={cpuLimit > 0 ? `Limit ${cpuLimit}%` : 'No limit'}
            />
            <StatTile
                label="Memory"
                icon={<MemoryStick size={16} aria-hidden />}
                value={memory === null ? '—' : formatMB(memory)}
                detail={
                    <div className="space-y-2">
                        <span>{memoryLimit > 0 ? `of ${formatMB(memoryLimit)}` : 'No limit'}</span>
                        {memoryLimit > 0 && (
                            <ProgressBar
                                label="Memory used"
                                value={memory === null ? 0 : (memory / memoryLimit) * 100}
                                tone={memory !== null && memory / memoryLimit > 0.9 ? 'warning' : 'primary'}
                            />
                        )}
                    </div>
                }
            />
            <StatTile
                label="Network"
                icon={<Network size={16} aria-hidden />}
                value={
                    <span className="flex flex-wrap items-center gap-x-4 gap-y-1 text-lg">
                        <span className="inline-flex items-center gap-1.5" title="Received">
                            <ArrowDown size={16} className="text-muted" aria-hidden />
                            <span className="sr-only">Received</span>
                            {rx === null ? '—' : formatBytes(rx)}
                        </span>
                        <span className="inline-flex items-center gap-1.5" title="Sent">
                            <ArrowUp size={16} className="text-muted" aria-hidden />
                            <span className="sr-only">Sent</span>
                            {tx === null ? '—' : formatBytes(tx)}
                        </span>
                    </span>
                }
                detail={up ? 'In and out since the server started' : 'Shown while the server is running'}
            />
        </div>
    );
}
