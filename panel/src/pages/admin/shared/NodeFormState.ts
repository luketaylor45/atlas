import { parseInteger } from '../../../lib/format';
import type { Node } from '../../../lib/types';

/** The node form keeps every field as text so inputs never hold NaN. */
export interface NodeFormValues {
    name: string;
    address: string;
    internal_address: string;
    port: string;
    sftp_port: string;
    total_ram: string;
    total_disk: string;
    location: string;
}

export type NodeFormErrors = Partial<Record<keyof NodeFormValues, string>>;

/** What POST and PUT /admin/nodes accept. Ports are strings in the API. */
export interface NodePayload {
    name: string;
    address: string;
    internal_address: string;
    port: string;
    sftp_port: string;
    total_ram: number;
    total_disk: number;
    location: string;
}

export const emptyNodeForm: NodeFormValues = {
    name: '',
    address: '',
    internal_address: '',
    port: '8081',
    sftp_port: '2022',
    total_ram: '0',
    total_disk: '0',
    location: '',
};

export function nodeToForm(node: Node): NodeFormValues {
    return {
        name: node.name ?? '',
        address: node.address ?? '',
        internal_address: node.internal_address ?? '',
        port: String(node.port ?? ''),
        sftp_port: String(node.sftp_port ?? ''),
        total_ram: String(node.total_ram ?? 0),
        total_disk: String(node.total_disk ?? 0),
        location: node.location ?? '',
    };
}

function hostError(value: string): string | undefined {
    if (value.includes('://')) return 'Leave out http:// or https://.';
    if (/\s/.test(value)) return 'An address cannot contain spaces.';
    if (/[/\\?#@]/.test(value)) return 'Enter only a hostname or IP address, without a path.';
    if (/:\d+$/.test(value) && !value.slice(0, value.lastIndexOf(':')).includes(':')) {
        return 'Leave the port out; it has its own field.';
    }
    if (value.length > 253) return 'This address is too long.';
    return undefined;
}

function portError(value: string): string | undefined {
    if (!/^\d+$/.test(value)) return 'Enter a port number between 1 and 65535.';
    const port = parseInteger(value, 0);
    if (port < 1 || port > 65535) return 'Enter a port number between 1 and 65535.';
    return undefined;
}

function sizeError(value: string): string | undefined {
    if (!/^\d+$/.test(value)) return 'Enter a whole number of MB, or 0 for no limit.';
    return undefined;
}

/** Mirrors the API's rules so problems show on the field before anything is sent. */
export function validateNodeForm(values: NodeFormValues): NodeFormErrors {
    const errors: NodeFormErrors = {};

    if (!values.name.trim()) errors.name = 'Give the node a name.';

    const address = values.address.trim();
    if (!address) errors.address = 'Enter the hostname or IP address of the node.';
    else {
        const problem = hostError(address);
        if (problem) errors.address = problem;
    }

    const internal = values.internal_address.trim();
    if (internal) {
        const problem = hostError(internal);
        if (problem) errors.internal_address = problem;
    }

    const port = portError(values.port.trim());
    if (port) errors.port = port;
    const sftpPort = portError(values.sftp_port.trim());
    if (sftpPort) errors.sftp_port = sftpPort;

    const ram = sizeError(values.total_ram.trim());
    if (ram) errors.total_ram = ram;
    const disk = sizeError(values.total_disk.trim());
    if (disk) errors.total_disk = disk;

    return errors;
}

/** Call only after `validateNodeForm` returned no errors. */
export function nodeFormToPayload(values: NodeFormValues): NodePayload {
    return {
        name: values.name.trim(),
        address: values.address.trim(),
        internal_address: values.internal_address.trim(),
        port: String(parseInteger(values.port.trim(), 8081)),
        sftp_port: String(parseInteger(values.sftp_port.trim(), 2022)),
        total_ram: parseInteger(values.total_ram.trim(), 0),
        total_disk: parseInteger(values.total_disk.trim(), 0),
        location: values.location.trim(),
    };
}

/** The fields of `payload` that differ from the saved node. */
export function changedNodeFields(node: Node, payload: NodePayload): Partial<NodePayload> {
    const saved = nodeFormToPayload(nodeToForm(node));
    const changes: Partial<NodePayload> = {};
    if (payload.name !== saved.name) changes.name = payload.name;
    if (payload.address !== saved.address) changes.address = payload.address;
    if (payload.internal_address !== saved.internal_address) changes.internal_address = payload.internal_address;
    if (payload.port !== saved.port) changes.port = payload.port;
    if (payload.sftp_port !== saved.sftp_port) changes.sftp_port = payload.sftp_port;
    if (payload.total_ram !== saved.total_ram) changes.total_ram = payload.total_ram;
    if (payload.total_disk !== saved.total_disk) changes.total_disk = payload.total_disk;
    if (payload.location !== saved.location) changes.location = payload.location;
    return changes;
}
