import { useCallback, useEffect, useMemo, useState } from 'react';
import CodeMirror from '@uiw/react-codemirror';
import { ArrowLeft, Save } from 'lucide-react';
import { Alert, Badge, Button, Card, LoadingState, useConfirm, useToast } from '../../../components/ui';
import { useTheme } from '../../../context/theme';
import api from '../../../lib/api';
import { errorMessage } from '../../../lib/errors';
import { languageFor } from './languages';
import { baseName, looksBinary, MAX_EDIT_BYTES, textErrorMessage } from './util';

interface FileEditorProps {
    uuid: string;
    /** Path of the file inside the server, without a leading slash. */
    path: string;
    /** Called when the user goes back to the listing. */
    onClose: () => void;
    onShowSftp: () => void;
}

type LoadState =
    | { status: 'loading' }
    | { status: 'error'; message: string }
    | { status: 'unsupported'; reason: 'binary' | 'large' }
    | { status: 'ready' };

/** A full-width text editor for one file. Mount with `key={path}`. */
export default function FileEditor({ uuid, path, onClose, onShowSftp }: FileEditorProps) {
    const { theme } = useTheme();
    const toast = useToast();
    const confirm = useConfirm();

    const [load, setLoad] = useState<LoadState>({ status: 'loading' });
    const [attempt, setAttempt] = useState(0);
    const [value, setValue] = useState('');
    const [saved, setSaved] = useState('');
    // Files written on Windows keep their CRLF line endings when saved.
    const [crlf, setCrlf] = useState(false);
    const [saving, setSaving] = useState(false);
    const [saveError, setSaveError] = useState<string | null>(null);

    const name = baseName(path);
    const language = useMemo(() => languageFor(name), [name]);
    const dirty = load.status === 'ready' && value !== saved;

    useEffect(() => {
        let cancelled = false;
        api.get<string>(`/services/${uuid}/files/content`, {
            params: { path },
            // The body is the raw file. Without this axios would parse JSON files into objects.
            responseType: 'text',
            transformResponse: [(data: unknown) => data],
        })
            .then((response) => {
                if (cancelled) return;
                const raw = typeof response.data === 'string' ? response.data : '';
                if (looksBinary(raw)) {
                    setLoad({ status: 'unsupported', reason: 'binary' });
                    return;
                }
                if (raw.length > MAX_EDIT_BYTES) {
                    setLoad({ status: 'unsupported', reason: 'large' });
                    return;
                }
                const text = raw.replace(/\r\n?/g, '\n');
                setCrlf(raw.includes('\r\n'));
                setValue(text);
                setSaved(text);
                setLoad({ status: 'ready' });
            })
            .catch((err: unknown) => {
                if (cancelled) return;
                setLoad({ status: 'error', message: textErrorMessage(err, 'Could not open this file.') });
            });
        return () => {
            cancelled = true;
        };
    }, [uuid, path, attempt]);

    const retry = () => {
        setLoad({ status: 'loading' });
        setAttempt((current) => current + 1);
    };

    const save = useCallback(async () => {
        if (!dirty || saving) return;
        const content = value;
        setSaving(true);
        setSaveError(null);
        try {
            await api.post(
                `/services/${uuid}/files/write`,
                { path, content: crlf ? content.replace(/\n/g, '\r\n') : content },
                { params: { path } },
            );
            setSaved(content);
            toast.success(`Saved ${name}`);
        } catch (err) {
            const message = errorMessage(err, 'Could not save this file.');
            setSaveError(message);
            toast.error(message);
        } finally {
            setSaving(false);
        }
    }, [crlf, dirty, name, path, saving, toast, uuid, value]);

    // Ctrl/Cmd+S saves instead of opening the browser's "save page" dialog.
    useEffect(() => {
        const onKeyDown = (event: KeyboardEvent) => {
            if ((event.ctrlKey || event.metaKey) && !event.altKey && event.key.toLowerCase() === 's') {
                event.preventDefault();
                void save();
            }
        };
        window.addEventListener('keydown', onKeyDown);
        return () => window.removeEventListener('keydown', onKeyDown);
    }, [save]);

    // Warn before a reload or tab close throws the changes away.
    useEffect(() => {
        if (!dirty) return;
        const onBeforeUnload = (event: BeforeUnloadEvent) => {
            event.preventDefault();
            event.returnValue = '';
        };
        window.addEventListener('beforeunload', onBeforeUnload);
        return () => window.removeEventListener('beforeunload', onBeforeUnload);
    }, [dirty]);

    const leave = async () => {
        if (dirty) {
            const discard = await confirm({
                title: 'Discard unsaved changes?',
                description: `Your changes to ${name} have not been saved and will be lost.`,
                confirmLabel: 'Discard changes',
                cancelLabel: 'Keep editing',
                danger: true,
            });
            if (!discard) return;
        }
        onClose();
    };

    return (
        <div className="space-y-4">
            <div className="flex flex-wrap items-center justify-between gap-3">
                <div className="flex min-w-0 flex-wrap items-center gap-3">
                    <Button variant="outline" icon={<ArrowLeft size={16} aria-hidden />} onClick={() => void leave()} disabled={saving}>
                        Back to files
                    </Button>
                    <span className="min-w-0 break-all font-mono text-sm text-foreground" title={`/${path}`}>
                        /{path}
                    </span>
                </div>
                {load.status === 'ready' && (
                    <div className="flex flex-wrap items-center gap-3">
                        {dirty && (
                            <Badge tone="warning" dot>
                                Unsaved changes
                            </Badge>
                        )}
                        <Button
                            variant="primary"
                            icon={<Save size={16} aria-hidden />}
                            onClick={() => void save()}
                            loading={saving}
                            disabled={!dirty}
                        >
                            Save
                        </Button>
                    </div>
                )}
            </div>

            {saveError && (
                <Alert tone="danger" title="The file was not saved">
                    {saveError} Your changes are still in the editor.
                </Alert>
            )}

            {load.status === 'loading' && (
                <Card>
                    <LoadingState label="Opening file…" />
                </Card>
            )}

            {load.status === 'error' && (
                <Alert
                    tone="danger"
                    title="Could not open this file"
                    actions={
                        <Button size="sm" onClick={retry}>
                            Try again
                        </Button>
                    }
                >
                    {load.message}
                </Alert>
            )}

            {load.status === 'unsupported' && (
                <Alert
                    tone="info"
                    title="This file cannot be edited here"
                    actions={
                        <Button size="sm" onClick={onShowSftp}>
                            SFTP details
                        </Button>
                    }
                >
                    {load.reason === 'binary'
                        ? 'It is not a text file, so editing it in the browser would damage it. Use SFTP to download or replace it.'
                        : 'It is too large for the browser editor. Use SFTP to download it, edit it on your computer and upload it again.'}
                </Alert>
            )}

            {load.status === 'ready' && (
                <>
                    <Card
                        flush
                        className="overflow-hidden [&_.cm-editor]:!bg-transparent [&_.cm-editor]:!outline-none [&_.cm-editor]:text-[13px] [&_.cm-gutters]:!border-border [&_.cm-gutters]:!bg-transparent [&_.cm-scroller]:!font-mono [&_.cm-scroller]:leading-relaxed"
                    >
                        <CodeMirror
                            value={value}
                            onChange={setValue}
                            extensions={language.extensions}
                            theme={theme === 'dark' ? 'dark' : 'light'}
                            height="max(400px, calc(100dvh - 320px))"
                            aria-label={`Contents of ${name}`}
                        />
                    </Card>
                    <p className="text-xs text-muted">
                        {language.label}
                        {crlf ? ' · Windows line endings' : ''} · Press Ctrl+S (or Cmd+S) to save
                    </p>
                </>
            )}
        </div>
    );
}
