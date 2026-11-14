// Shapes returned by the Atlas API. Field names match the JSON exactly.

export interface User {
    id: number;
    username: string;
    is_admin: boolean;
    created_at?: string;
}

export interface Node {
    id: number;
    name: string;
    /** Public hostname or IP shown to players and SFTP users. */
    address: string;
    /** How the panel reaches the daemon; empty for non-admins. */
    internal_address?: string;
    port: string;
    sftp_port: string;
    total_ram: number;
    total_disk: number;
    used_ram: number;
    used_cpu: number;
    location: string;
    is_online: boolean;
    last_heartbeat?: string;
}

export interface EggVariable {
    id: number;
    egg_id: number;
    name: string;
    description: string;
    environment_variable: string;
    default_value: string;
    user_viewable: boolean;
    user_editable: boolean;
    rules: string;
    input_type: string;
}

/** An additional port an egg needs besides the primary one. */
export interface EggPort {
    name?: string;
    env: string;
    default?: number;
    protocol?: string;
}

export interface Egg {
    id: number;
    uuid: string;
    nest_id: number;
    nest?: Nest;
    available: boolean;
    author: string;
    name: string;
    description: string;
    /** JSON-encoded string[] of image names. */
    docker_images: string;
    startup_command: string;
    stop_command: string;
    /** Empty for non-admins. */
    script_install: string;
    script_entry: string;
    script_container: string;
    /** JSON-encoded EggPort[]. */
    ports: string;
    source: string;
    customized: boolean;
    variables?: EggVariable[];
}

export interface Nest {
    id: number;
    uuid: string;
    code: string;
    name: string;
    description: string;
    parent_id: number | null;
    sub_nests?: Nest[];
    eggs?: Egg[];
}

export interface ServicePort {
    id: number;
    port: number;
    env: string;
    name: string;
    protocol: string;
    primary: boolean;
}

export type ServiceStatus =
    | 'installing'
    | 'installation_failed'
    | 'starting'
    | 'running'
    | 'stopping'
    | 'offline';

export interface Service {
    id: number;
    uuid: string;
    name: string;
    user_id: number;
    user?: User;
    node_id: number;
    node?: Node;
    egg_id: number;
    egg?: Egg;
    memory: number;
    disk: number;
    cpu: number;
    port: number;
    ports?: ServicePort[];
    docker_image: string;
    is_suspended: boolean;
    status: ServiceStatus | string;
    installation_stage: string;
    installation_progress: number;
    startup_command: string;
    /** JSON-encoded Record<string, string>. */
    environment: string;
    created_at: string;
    updated_at: string;
}

/** A sub-user's permissions on one service. */
export interface ServicePermissions {
    can_view_console: boolean;
    can_send_commands: boolean;
    can_manage_files: boolean;
    can_edit_startup: boolean;
    can_control_power: boolean;
    can_access_sftp: boolean;
}

export interface ServiceUser extends ServicePermissions {
    id: number;
    service_id: number;
    user_id: number;
    user?: User;
    created_at: string;
}

/** Response of GET /services/:uuid. */
export interface ServiceDetails {
    service: Service;
    is_owner: boolean;
    /** Present only for sub-users. */
    permissions?: ServiceUser;
}

export interface ActivityLog {
    id: number;
    service_id: number | null;
    user_id: number | null;
    user?: User | null;
    action: string;
    resource: string;
    description: string;
    ip_address: string;
    user_agent: string;
    metadata: string;
    created_at: string;
}

export interface NewsItem {
    id: number;
    title: string;
    content: string;
    type: string;
    created_at: string;
}

export interface ServiceStats {
    cpu: string | number;
    /** Megabytes. */
    memory: number;
    network: { rx: number; tx: number };
}

/** Safely parses a JSON string field, returning the fallback on any problem. */
export function parseJSON<T>(value: string | null | undefined, fallback: T): T {
    if (!value) return fallback;
    try {
        const parsed = JSON.parse(value) as T;
        return parsed ?? fallback;
    } catch {
        return fallback;
    }
}

export function eggImages(egg: Pick<Egg, 'docker_images'> | null | undefined): string[] {
    const images = parseJSON<unknown>(egg?.docker_images, []);
    return Array.isArray(images) ? images.filter((image): image is string => typeof image === 'string') : [];
}

export function eggPorts(egg: Pick<Egg, 'ports'> | null | undefined): EggPort[] {
    const ports = parseJSON<unknown>(egg?.ports, []);
    return Array.isArray(ports) ? ports.filter((port): port is EggPort => Boolean(port && (port as EggPort).env)) : [];
}
