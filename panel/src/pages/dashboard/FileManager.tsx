import { lazy, Suspense, useEffect, useRef, useState, type ChangeEvent, type DragEvent } from 'react';
import { ArchiveRestore, File, FilePlus, Folder, FolderOpen, FolderPlus, KeyRound, RefreshCw, Trash2, Upload } from 'lucide-react';
import {
    Alert,
    Button,
    Card,
    EmptyState,
    IconButton,
    LoadingState,
    Modal,
    ProgressBar,
    Table,
    Td,
    Th,
    Tr,
    useConfirm,
    useToast,
} from '../../components/ui';
import api from '../../lib/api';
import { cn } from '../../lib/cn';
import { errorMessage } from '../../lib/errors';
import { formatBytes, formatDateTime, formatRelative } from '../../lib/format';
import Breadcrumbs from './files/Breadcrumbs';
import NameDialog, { type NameDialogKind } from './files/NameDialog';
import SftpDetails from './files/SftpDetails';
import {
    isZip,
    joinPath,
    MAX_EDIT_BYTES,
    MAX_UPLOAD_BYTES,
    normaliseEntries,
    uneditableReason,
    type FileEntry,
    type FileManagerService,
    type UneditableReason,
} from './files/util';

// CodeMirror is large; it is only downloaded when a file is opened.
const FileEditor = lazy(() => import('./files/FileEditor'));

interface FileManagerProps {
    uuid: string;
    service: FileManagerService;
}

interface Listing {
    path: string;
    entries: FileEntry[];
}

interface UploadProgress {
    name: string;
    index: number;
    total: number;
    percent: number;
}

interface UploadFailure {
    name: string;
    message: string;
}

export default function FileManager({ uuid, service }: FileManagerProps) {
    // A different server starts again from its own root folder.
    return <FileManagerView key={uuid} uuid={uuid} service={service} />;
}

function FileManagerView({ uuid, service }: FileManagerProps) {
    const toast = useToast();
    const confirm = useConfirm();

    const [path, setPath] = useState('');
    const [reloadCount, setReloadCount] = useState(0);
    const [listing, setListing] = useState<Listing | null>(null);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState<string | null>(null);

    const [editing, setEditing] = useState<string | null>(null);
    const [nameDialog, setNameDialog] = useState<NameDialogKind | null>(null);
    const [sftpOpen, setSftpOpen] = useState(false);
    const [blocked, setBlocked] = useState<{ name: string; reason: UneditableReason } | null>(null);
    const [busy, setBusy] = useState<{ name: string; action: 'delete' | 'extract' } | null>(null);

    const [upload, setUpload] = useState<UploadProgress | null>(null);
    const [uploadFailures, setUploadFailures] = useState<UploadFailure[]>([]);
    const [dragging, setDragging] = useState(false);
    const dragDepth = useRef(0);
    const fileInput = useRef<HTMLInputElement>(null);

    useEffect(() => {
        let cancelled = false;
        api.get<unknown>(`/services/${uuid}/files/list`, { params: { path } })
            .then((response) => {
                if (cancelled) return;
                setListing({ path, entries: normaliseEntries(response.data) });
                setError(null);
                setLoading(false);
            })
            .catch((err: unknown) => {
                if (cancelled) return;
                setError(errorMessage(err, 'Could not load this folder.'));
                setLoading(false);
            });
        return () => {
            cancelled = true;
        };
    }, [uuid, path, reloadCount]);

    /** Reloads the current folder, keeping the user's place. */
    const refresh = () => {
        setLoading(true);
        setError(null);
        setReloadCount((count) => count + 1);
    };

    const openFolder = (next: string) => {
        if (next === path) {
            refresh();
            return;
        }
        setLoading(true);
        setError(null);
        setPath(next);
    };

    // Only show entries that belong to the folder currently selected.
    const current = listing && listing.path === path ? listing : null;
    const entries = current?.entries ?? [];
    const showModified = entries.some((entry) => entry.modified);
    const location = `/${path}`;

    const openFile = (entry: FileEntry) => {
        const reason = uneditableReason(entry);
        if (reason) {
            setBlocked({ name: entry.name, reason });
            return;
        }
        setEditing(joinPath(path, entry.name));
    };

    const createItem = async (name: string) => {
        const target = joinPath(path, name);
        if (nameDialog === 'folder') {
            try {
                await api.post(`/services/${uuid}/files/create-folder`, { path: target });
            } catch (err) {
                throw new Error(errorMessage(err, 'Could not create the folder.'), { cause: err });
            }
            setNameDialog(null);
            toast.success(`Created folder ${name}`);
            refresh();
            return;
        }

        try {
            await api.post(`/services/${uuid}/files/write`, { path: target, content: '' }, { params: { path: target } });
        } catch (err) {
            throw new Error(errorMessage(err, 'Could not create the file.'), { cause: err });
        }
        setNameDialog(null);
        refresh();
        setEditing(target);
    };

    const deleteEntry = async (entry: FileEntry) => {
        const confirmed = await confirm({
            title: entry.is_dir ? `Delete the folder "${entry.name}"?` : `Delete "${entry.name}"?`,
            description: entry.is_dir
                ? 'The folder and everything inside it will be permanently deleted. This cannot be undone.'
                : 'The file will be permanently deleted. This cannot be undone.',
            confirmLabel: entry.is_dir ? 'Delete folder' : 'Delete file',
            danger: true,
        });
        if (!confirmed) return;

        setBusy({ name: entry.name, action: 'delete' });
        try {
            await api.delete(`/services/${uuid}/files`, { params: { path: joinPath(path, entry.name) } });
            toast.success(`Deleted ${entry.name}`);
        } catch (err) {
            toast.error(errorMessage(err, `Could not delete ${entry.name}.`));
        } finally {
            setBusy(null);
            refresh();
        }
    };

    const extractEntry = async (entry: FileEntry) => {
        const confirmed = await confirm({
            title: `Extract "${entry.name}"?`,
            description: 'Its contents are unpacked into this folder. Existing files with the same names are overwritten.',
            confirmLabel: 'Extract archive',
        });
        if (!confirmed) return;

        setBusy({ name: entry.name, action: 'extract' });
        try {
            const response = await api.post<{ extracted?: number }>(`/services/${uuid}/files/extract`, {
                path: joinPath(path, entry.name),
            });
            const count = response.data?.extracted;
            toast.success(
                typeof count === 'number'
                    ? `Extracted ${count} ${count === 1 ? 'item' : 'items'} from ${entry.name}`
                    : `Extracted ${entry.name}`,
            );
        } catch (err) {
            toast.error(errorMessage(err, `Could not extract ${entry.name}.`));
        } finally {
            setBusy(null);
            refresh();
        }
    };

    const uploadFiles = async (files: File[]) => {
        if (files.length === 0 || upload) return;

        const target = path;
        const failures: UploadFailure[] = [];
        let uploaded = 0;
        setUploadFailures([]);

        // One at a time, so a big file does not starve the others and progress stays readable.
        for (let index = 0; index < files.length; index += 1) {
            const file = files[index];
            setUpload({ name: file.name, index: index + 1, total: files.length, percent: 0 });

            if (file.size > MAX_UPLOAD_BYTES) {
                failures.push({ name: file.name, message: `Larger than the ${formatBytes(MAX_UPLOAD_BYTES)} upload limit. Use SFTP for this file.` });
                continue;
            }

            const form = new FormData();
            form.append('file', file);
            try {
                await api.post(`/services/${uuid}/files/upload`, form, {
                    params: { path: target },
                    headers: { 'Content-Type': 'multipart/form-data' },
                    onUploadProgress: (event) => {
                        const total = event.total ?? file.size;
                        const percent = total > 0 ? Math.min(100, Math.round((event.loaded / total) * 100)) : 0;
                        setUpload((state) => (state ? { ...state, percent } : state));
                    },
                });
                uploaded += 1;
            } catch (err) {
                failures.push({ name: file.name, message: errorMessage(err, 'The upload failed.') });
            }
        }

        setUpload(null);
        setUploadFailures(failures);
        if (uploaded > 0) {
            toast.success(uploaded === 1 ? 'Uploaded 1 file' : `Uploaded ${uploaded} files`);
        }
        refresh();
    };

    const onFilesChosen = (event: ChangeEvent<HTMLInputElement>) => {
        const files = Array.from(event.target.files ?? []);
        // Reset so choosing the same file again still fires a change event.
        event.target.value = '';
        void uploadFiles(files);
    };

    const hasFiles = (event: DragEvent) => Array.from(event.dataTransfer.types).includes('Files');

    const onDragEnter = (event: DragEvent<HTMLDivElement>) => {
        if (!hasFiles(event)) return;
        event.preventDefault();
        dragDepth.current += 1;
        setDragging(true);
    };

    const onDragOver = (event: DragEvent<HTMLDivElement>) => {
        if (!hasFiles(event)) return;
        event.preventDefault();
        event.dataTransfer.dropEffect = upload ? 'none' : 'copy';
    };

    const onDragLeave = (event: DragEvent<HTMLDivElement>) => {
        if (!hasFiles(event)) return;
        dragDepth.current = Math.max(0, dragDepth.current - 1);
        if (dragDepth.current === 0) setDragging(false);
    };

    const onDrop = (event: DragEvent<HTMLDivElement>) => {
        if (!hasFiles(event)) return;
        event.preventDefault();
        dragDepth.current = 0;
        setDragging(false);

        if (upload) {
            toast.info('Wait for the current upload to finish first.');
            return;
        }

        const files: File[] = [];
        let folders = 0;
        for (const item of Array.from(event.dataTransfer.items)) {
            if (item.kind !== 'file') continue;
            if (item.webkitGetAsEntry?.()?.isDirectory) {
                folders += 1;
                continue;
            }
            const file = item.getAsFile();
            if (file) files.push(file);
        }
        if (folders > 0) {
            toast.info('Folders cannot be uploaded here. Use SFTP, or upload a .zip and extract it.');
        }
        void uploadFiles(files);
    };

    const sftpModal = <SftpDetails open={sftpOpen} onClose={() => setSftpOpen(false)} service={service} />;

    if (editing !== null) {
        return (
            <>
                <Suspense
                    fallback={
                        <Card>
                            <LoadingState label="Opening editor…" />
                        </Card>
                    }
                >
                    <FileEditor
                        key={editing}
                        uuid={uuid}
                        path={editing}
                        onShowSftp={() => setSftpOpen(true)}
                        onClose={() => {
                            setEditing(null);
                            refresh();
                        }}
                    />
                </Suspense>
                {sftpModal}
            </>
        );
    }

    const actionsDisabled = busy !== null;

    return (
        <div className="space-y-4">
            <div className="flex flex-wrap items-center justify-between gap-3">
                <Breadcrumbs path={path} onNavigate={openFolder} />
                <div className="flex flex-wrap items-center gap-2">
                    <Button icon={<FolderPlus size={16} aria-hidden />} onClick={() => setNameDialog('folder')} disabled={!current}>
                        New folder
                    </Button>
                    <Button icon={<FilePlus size={16} aria-hidden />} onClick={() => setNameDialog('file')} disabled={!current}>
                        New file
                    </Button>
                    <Button
                        variant="primary"
                        icon={<Upload size={16} aria-hidden />}
                        onClick={() => fileInput.current?.click()}
                        loading={upload !== null}
                    >
                        Upload
                    </Button>
                    <Button variant="outline" icon={<KeyRound size={16} aria-hidden />} onClick={() => setSftpOpen(true)}>
                        SFTP details
                    </Button>
                    <IconButton label="Refresh" variant="outline" onClick={refresh} loading={loading}>
                        <RefreshCw size={16} aria-hidden />
                    </IconButton>
                    <input ref={fileInput} type="file" multiple className="sr-only" tabIndex={-1} aria-hidden onChange={onFilesChosen} />
                </div>
            </div>

            {upload && (
                <Card className="p-4 sm:p-4" role="status">
                    <div className="mb-2 flex flex-wrap items-baseline justify-between gap-x-3 gap-y-1 text-sm">
                        <span className="min-w-0 break-all text-foreground">
                            Uploading {upload.name}
                            {upload.total > 1 && <span className="text-muted"> ({upload.index} of {upload.total})</span>}
                        </span>
                        <span className="tabular text-muted">{upload.percent >= 100 ? 'Finishing…' : `${upload.percent}%`}</span>
                    </div>
                    <ProgressBar value={upload.percent} label={`Uploading ${upload.name}`} />
                </Card>
            )}

            {uploadFailures.length > 0 && (
                <Alert
                    tone="danger"
                    title={uploadFailures.length === 1 ? '1 file was not uploaded' : `${uploadFailures.length} files were not uploaded`}
                    actions={
                        <Button size="sm" variant="ghost" onClick={() => setUploadFailures([])}>
                            Dismiss
                        </Button>
                    }
                >
                    <ul className="space-y-1">
                        {uploadFailures.map((failure, index) => (
                            <li key={`${failure.name}-${index}`} className="break-all">
                                <span className="font-medium text-foreground">{failure.name}</span>: {failure.message}
                            </li>
                        ))}
                    </ul>
                </Alert>
            )}

            <div onDragEnter={onDragEnter} onDragOver={onDragOver} onDragLeave={onDragLeave} onDrop={onDrop}>
                <Card flush className={cn('overflow-hidden transition-colors', dragging && 'border-primary')}>
                    {dragging && (
                        <div className="border-b border-border bg-primary/10 px-5 py-2.5 text-sm text-primary-text">
                            {upload ? 'Wait for the current upload to finish.' : `Drop files to upload them to ${location}`}
                        </div>
                    )}

                    {error ? (
                        <div className="p-4">
                            <Alert
                                tone="danger"
                                title="Could not load this folder"
                                actions={
                                    <Button size="sm" onClick={refresh} loading={loading}>
                                        Try again
                                    </Button>
                                }
                            >
                                {error}
                            </Alert>
                        </div>
                    ) : !current ? (
                        <LoadingState label="Loading files…" />
                    ) : entries.length === 0 ? (
                        <EmptyState
                            icon={<FolderOpen size={20} aria-hidden />}
                            title="This folder is empty"
                            description="Drop files here to upload them, or create a new file or folder."
                            action={
                                <Button variant="outline" icon={<Upload size={16} aria-hidden />} onClick={() => fileInput.current?.click()}>
                                    Upload files
                                </Button>
                            }
                        />
                    ) : (
                        <Table className={cn(loading && 'opacity-60')}>
                            <thead>
                                <tr>
                                    <Th>Name</Th>
                                    <Th className="text-right">Size</Th>
                                    {showModified && <Th>Modified</Th>}
                                    <Th className="text-right">Actions</Th>
                                </tr>
                            </thead>
                            <tbody>
                                {entries.map((entry) => {
                                    const rowBusy = busy?.name === entry.name ? busy.action : null;
                                    return (
                                        <Tr key={entry.name}>
                                            <Td className="w-full min-w-48">
                                                <button
                                                    type="button"
                                                    onClick={() => (entry.is_dir ? openFolder(joinPath(path, entry.name)) : openFile(entry))}
                                                    className="group flex max-w-full items-center gap-3 rounded-control text-left text-foreground"
                                                >
                                                    {entry.is_dir ? (
                                                        <Folder size={16} className="shrink-0 text-primary-text" aria-hidden />
                                                    ) : (
                                                        <File size={16} className="shrink-0 text-subtle" aria-hidden />
                                                    )}
                                                    <span className="break-all font-medium group-hover:underline">{entry.name}</span>
                                                    <span className="sr-only">{entry.is_dir ? '(folder)' : '(file)'}</span>
                                                </button>
                                            </Td>
                                            <Td className="tabular whitespace-nowrap text-right text-muted">
                                                {entry.is_dir ? '' : formatBytes(entry.size)}
                                            </Td>
                                            {showModified && (
                                                <Td className="whitespace-nowrap text-muted">
                                                    {entry.modified && <span title={formatDateTime(entry.modified)}>{formatRelative(entry.modified)}</span>}
                                                </Td>
                                            )}
                                            <Td>
                                                <div className="flex items-center justify-end gap-1">
                                                    {!entry.is_dir && isZip(entry.name) && (
                                                        <Button
                                                            size="sm"
                                                            variant="ghost"
                                                            icon={<ArchiveRestore size={16} aria-hidden />}
                                                            onClick={() => void extractEntry(entry)}
                                                            loading={rowBusy === 'extract'}
                                                            disabled={actionsDisabled}
                                                            aria-label={`Extract ${entry.name}`}
                                                        >
                                                            Extract
                                                        </Button>
                                                    )}
                                                    <IconButton
                                                        size="sm"
                                                        label={`Delete ${entry.name}`}
                                                        className="hover:bg-danger/10 hover:text-danger"
                                                        onClick={() => void deleteEntry(entry)}
                                                        loading={rowBusy === 'delete'}
                                                        disabled={actionsDisabled}
                                                    >
                                                        <Trash2 size={16} aria-hidden />
                                                    </IconButton>
                                                </div>
                                            </Td>
                                        </Tr>
                                    );
                                })}
                            </tbody>
                        </Table>
                    )}
                </Card>
            </div>

            <p className="text-xs text-muted">
                Choose files or drop them on the list to upload them here (up to {formatBytes(MAX_UPLOAD_BYTES)} each). For large files or whole
                folders, SFTP is faster and more reliable.
            </p>

            {nameDialog && (
                <NameDialog
                    kind={nameDialog}
                    location={location}
                    existing={entries}
                    onCreate={createItem}
                    onClose={() => setNameDialog(null)}
                />
            )}

            <Modal
                open={blocked !== null}
                onClose={() => setBlocked(null)}
                size="sm"
                title="This file cannot be edited here"
                footer={
                    <>
                        <Button variant="ghost" onClick={() => setBlocked(null)}>
                            Close
                        </Button>
                        <Button
                            variant="primary"
                            onClick={() => {
                                setBlocked(null);
                                setSftpOpen(true);
                            }}
                        >
                            SFTP details
                        </Button>
                    </>
                }
            >
                {blocked && (
                    <p className="text-sm text-muted">
                        <span className="break-all font-medium text-foreground">{blocked.name}</span>{' '}
                        {blocked.reason === 'binary'
                            ? 'is not a text file, so it cannot be opened in the editor. Use SFTP to download or replace it.'
                            : `is larger than ${formatBytes(MAX_EDIT_BYTES)}, which is too big for the browser editor. Use SFTP to download it, edit it on your computer and upload it again.`}
                    </p>
                )}
            </Modal>

            {sftpModal}
        </div>
    );
}
