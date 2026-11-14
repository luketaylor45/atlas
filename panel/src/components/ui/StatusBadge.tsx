import { serviceStatus } from '../../lib/status';
import { Badge } from './Surface';

/** The status pill for a service, identical everywhere it appears. */
export function StatusBadge({ status, suspended, className }: { status?: string | null; suspended?: boolean; className?: string }) {
    const info = serviceStatus(status, suspended);
    return (
        <Badge tone={info.tone} dot pulse={info.busy} className={className}>
            {info.label}
        </Badge>
    );
}

/** Online/offline pill for a node. */
export function NodeStatusBadge({ online, className }: { online: boolean; className?: string }) {
    return (
        <Badge tone={online ? 'success' : 'danger'} dot className={className}>
            {online ? 'Online' : 'Offline'}
        </Badge>
    );
}
