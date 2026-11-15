import { formatCpu, formatMB } from '../../../lib/format';
import type { Node } from '../../../lib/types';

export interface ResourceLimit {
    /** The top of the slider. */
    max: number;
    /** What is left on the node; only meaningful with a capacity. */
    free: number;
    capacity: { allocated: number; total: number } | null;
    hint: string;
    tooMuch: string;
}

/**
 * What the sliders can offer on a node. Memory and disk are limited by what
 * the node allows minus what its servers already have. CPU is shared, so it
 * is only bounded by the machine's cores.
 */
export function resourceLimits(
    node: Node | null,
    /** The server's own current limits when editing it, which are not "taken". */
    own?: { memory: number; disk: number; cpu: number },
): { memory: ResourceLimit; disk: ResourceLimit; cpu: ResourceLimit } {
    const pick = (total: number, allocated: number, fallback: number, what: string): ResourceLimit => {
        if (!node) {
            return { max: fallback, free: fallback, capacity: null, hint: 'Choose a node to see how much is free.', tooMuch: '' };
        }
        if (total <= 0) {
            return {
                max: fallback,
                free: fallback,
                capacity: null,
                hint: `${node.name} has no ${what} limit set, so any amount is accepted.`,
                tooMuch: '',
            };
        }
        const free = Math.max(0, total - allocated);
        return {
            max: free,
            free,
            capacity: { allocated, total },
            hint:
                free > 0
                    ? `${formatMB(free)} of ${formatMB(total)} available on ${node.name}.`
                    : `${node.name} has no ${what} left. Raise its limit on the Nodes page or free some up.`,
            tooMuch: `That is more than ${node.name} has free (${formatMB(free)}).`,
        };
    };

    // Without an admin limit, memory is still bounded by the machine itself.
    const memory = pick(node?.total_ram ?? 0, Math.max(0, (node?.allocated_memory ?? 0) - (own?.memory ?? 0)), node?.host_ram || 16384, 'memory');
    const disk = pick(node?.total_disk ?? 0, Math.max(0, (node?.allocated_disk ?? 0) - (own?.disk ?? 0)), 102400, 'disk');

    const cores = node?.cpu_cores ?? 0;
    const given = Math.max(0, (node?.allocated_cpu ?? 0) - (own?.cpu ?? 0));
    const cpu: ResourceLimit = {
        max: cores > 0 ? cores * 100 : 800,
        free: Infinity,
        capacity: null,
        hint: !node
            ? '100% is one full core.'
            : cores > 0
              ? `${node.name} has ${cores} cores. Cores are shared, so this is a ceiling, not a reservation${given > 0 ? `; ${own ? 'its other servers' : 'its servers'} have ${formatCpu(given)} between them` : ''}.`
              : '100% is one full core. The node has not reported its core count yet.',
        tooMuch: '',
    };
    return { memory, disk, cpu };
}
