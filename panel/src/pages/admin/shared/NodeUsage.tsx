import { ProgressBar, type Tone } from '../../../components/ui';
import { cn } from '../../../lib/cn';
import { formatMB } from '../../../lib/format';
import type { Node } from '../../../lib/types';

function usageTone(percent: number): Tone {
    if (percent > 90) return 'danger';
    if (percent > 75) return 'warning';
    return 'primary';
}

function Meter({ label, value, percent }: { label: string; value: string; percent?: number }) {
    return (
        <div className="min-w-0">
            <div className="tabular flex items-baseline justify-between gap-3 text-xs">
                <span className="text-muted">{label}</span>
                <span className="truncate text-foreground">{value}</span>
            </div>
            {percent !== undefined && <ProgressBar value={percent} tone={usageTone(percent)} label={label} className="mt-1.5" />}
        </div>
    );
}

/** CPU and memory use of a node, as last reported by its daemon. */
export function NodeUsage({ node, className }: { node: Node; className?: string }) {
    if (!node.is_online) {
        return <p className={cn('text-xs text-muted', className)}>No usage data while offline</p>;
    }

    const cpu = Math.max(0, Number(node.used_cpu) || 0);
    const usedRam = Math.max(0, Number(node.used_ram) || 0);
    const totalRam = Math.max(0, Number(node.total_ram) || 0);
    // A total of 0 means no limit is set, so there is nothing to measure against.
    const ramPercent = totalRam > 0 ? (usedRam / totalRam) * 100 : undefined;

    return (
        <div className={cn('grid gap-3', className)}>
            <Meter label="CPU" value={`${cpu.toFixed(cpu >= 10 ? 0 : 1)}%`} percent={cpu} />
            <Meter
                label="Memory"
                value={totalRam > 0 ? `${formatMB(usedRam)} of ${formatMB(totalRam)}` : `${formatMB(usedRam)} used`}
                percent={ramPercent}
            />
        </div>
    );
}
