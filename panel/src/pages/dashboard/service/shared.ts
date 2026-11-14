import type { ServiceDetails } from '../../../lib/types';

export type PowerAction = 'start' | 'stop' | 'restart' | 'kill';

/** What the signed-in user may do on this server. Owners and admins may do everything. */
export interface ServiceAccess {
    isOwner: boolean;
    canConsole: boolean;
    canCommands: boolean;
    canFiles: boolean;
    canStartup: boolean;
    canPower: boolean;
    canBackups: boolean;
    canSchedules: boolean;
}

export function serviceAccess(details: ServiceDetails): ServiceAccess {
    const owner = Boolean(details.is_owner);
    const permissions = details.permissions;
    return {
        isOwner: owner,
        canConsole: owner || Boolean(permissions?.can_view_console),
        canCommands: owner || Boolean(permissions?.can_send_commands),
        canFiles: owner || Boolean(permissions?.can_manage_files),
        canStartup: owner || Boolean(permissions?.can_edit_startup),
        canPower: owner || Boolean(permissions?.can_control_power),
        canBackups: owner || Boolean(permissions?.can_manage_backups),
        canSchedules: owner || Boolean(permissions?.can_manage_schedules),
    };
}

/** True while the game container is up or changing state, so a console stream is expected. */
export function isLiveStatus(status: string | null | undefined): boolean {
    return status === 'running' || status === 'starting' || status === 'stopping';
}

// eslint-disable-next-line no-control-regex
const ANSI_PATTERN = /[\u001b\u009b][[()#;?]*(?:[0-9]{1,4}(?:;[0-9]{0,4})*)?[0-9A-ORZcf-nqry=><]/g;

/** Removes ANSI escape codes, for output shown as plain text. */
export function stripAnsi(text: string): string {
    return text.replace(ANSI_PATTERN, '');
}
