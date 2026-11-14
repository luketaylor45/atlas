import type { Tone } from '../components/ui/Surface';

export interface StatusInfo {
    label: string;
    tone: Tone;
    /** True while the state is transitional and worth animating. */
    busy: boolean;
}

const statuses: Record<string, StatusInfo> = {
    running: { label: 'Running', tone: 'success', busy: false },
    starting: { label: 'Starting', tone: 'info', busy: true },
    stopping: { label: 'Stopping', tone: 'warning', busy: true },
    installing: { label: 'Installing', tone: 'primary', busy: true },
    installation_failed: { label: 'Install failed', tone: 'danger', busy: false },
    offline: { label: 'Offline', tone: 'neutral', busy: false },
};

/** How to present a service status. Unknown values read as offline. */
export function serviceStatus(status: string | null | undefined, suspended = false): StatusInfo {
    if (suspended) return { label: 'Suspended', tone: 'warning', busy: false };
    return statuses[(status || '').toLowerCase()] ?? statuses.offline;
}

/** True when the game container does not exist yet (or failed to be created). */
export function isInstallState(status: string | null | undefined): boolean {
    return status === 'installing' || status === 'installation_failed';
}
