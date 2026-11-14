import { isAxiosError } from 'axios';
import { errorMessage } from '../../../lib/errors';
import { parseJSON } from '../../../lib/types';

/** One row of GET /services/:uuid/files/list. */
export interface FileEntry {
    name: string;
    size: number;
    is_dir: boolean;
    /** Not sent by every daemon version; the column is hidden when absent. */
    modified?: string;
}

/** The service fields the file manager needs. A full `Service` satisfies this. */
export interface FileManagerService {
    uuid: string;
    node?: {
        address: string;
        sftp_port?: string | number;
    };
}

/** Files larger than this are not opened in the browser editor. */
export const MAX_EDIT_BYTES = 2 * 1024 * 1024;

/** The API rejects uploads larger than this. */
export const MAX_UPLOAD_BYTES = 1024 * 1024 * 1024;

export function joinPath(folder: string, name: string): string {
    return folder === '' ? name : `${folder}/${name}`;
}

export function baseName(path: string): string {
    return path.split('/').pop() ?? path;
}

export function extensionOf(name: string): string {
    const dot = name.lastIndexOf('.');
    return dot <= 0 ? '' : name.slice(dot + 1).toLowerCase();
}

export function isZip(name: string): boolean {
    return extensionOf(name) === 'zip';
}

/** Turns whatever the list endpoint returned into sorted, well-formed rows. */
export function normaliseEntries(data: unknown): FileEntry[] {
    if (!Array.isArray(data)) return [];

    const entries: FileEntry[] = [];
    for (const item of data as unknown[]) {
        if (!item || typeof item !== 'object') continue;
        const raw = item as Record<string, unknown>;
        if (typeof raw.name !== 'string' || raw.name === '') continue;
        const modified = raw.modified ?? raw.modified_at ?? raw.mod_time;
        entries.push({
            name: raw.name,
            size: Number(raw.size) || 0,
            is_dir: raw.is_dir === true,
            modified: typeof modified === 'string' ? modified : undefined,
        });
    }

    // Folders first, then files, each by name ignoring case.
    return entries.sort((a, b) => {
        if (a.is_dir !== b.is_dir) return a.is_dir ? -1 : 1;
        return a.name.toLowerCase().localeCompare(b.name.toLowerCase()) || a.name.localeCompare(b.name);
    });
}

/** Returns a message if the name cannot be used for a new file or folder. */
export function validateName(name: string, existing: FileEntry[]): string | null {
    const trimmed = name.trim();
    if (trimmed === '') return 'Enter a name.';
    if (/[\\/]/.test(trimmed)) return 'The name cannot contain slashes.';
    if (trimmed === '.' || trimmed === '..') return 'That name is not allowed.';
    if (trimmed.length > 255) return 'The name is too long.';
    if (existing.some((entry) => entry.name === trimmed)) return 'Something with that name already exists in this folder.';
    return null;
}

const BINARY_EXTENSIONS = new Set([
    // archives
    'zip', 'tar', 'gz', 'tgz', 'bz2', 'xz', '7z', 'rar', 'zst', 'jar', 'war',
    // programs and libraries
    'exe', 'dll', 'so', 'dylib', 'bin', 'o', 'a', 'class', 'pyc', 'wasm',
    // images, audio, video, fonts
    'png', 'jpg', 'jpeg', 'gif', 'bmp', 'ico', 'webp', 'tga', 'dds', 'psd',
    'mp3', 'ogg', 'wav', 'flac', 'mp4', 'mkv', 'avi', 'mov', 'webm',
    'ttf', 'otf', 'woff', 'woff2',
    // game and database data
    'dat', 'db', 'sqlite', 'sqlite3', 'mdb', 'mca', 'mcr', 'nbt', 'dem', 'bsp', 'vpk', 'gma', 'pak', 'sav',
    'vtf', 'mdl', 'vvd', 'vtx', 'phy', 'nav', 'ain', 'unity3d', 'assets', 'uasset', 'umap',
    // documents
    'pdf', 'doc', 'docx', 'xls', 'xlsx',
]);

export type UneditableReason = 'binary' | 'large';

/** Why a file cannot be opened in the editor, or null if it can. */
export function uneditableReason(entry: FileEntry): UneditableReason | null {
    if (BINARY_EXTENSIONS.has(extensionOf(entry.name))) return 'binary';
    if (entry.size > MAX_EDIT_BYTES) return 'large';
    return null;
}

/** True if text decoded from a file shows it was not really text. */
export function looksBinary(content: string): boolean {
    return content.includes('\u0000') || content.includes('�');
}

/**
 * Like `errorMessage`, but also understands failures of requests made with
 * `responseType: 'text'`, where the `{ error }` body arrives as a string.
 */
export function textErrorMessage(error: unknown, fallback: string): string {
    if (isAxiosError(error) && typeof error.response?.data === 'string') {
        const parsed = parseJSON<{ error?: unknown } | null>(error.response.data, null);
        if (parsed && typeof parsed.error === 'string' && parsed.error.trim()) {
            return parsed.error.trim();
        }
    }
    return errorMessage(error, fallback);
}
