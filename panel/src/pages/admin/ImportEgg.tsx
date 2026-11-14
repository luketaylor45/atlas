import { useEffect, useMemo, useRef, useState, type ChangeEvent, type FormEvent } from 'react';
import { Link, useNavigate, useSearchParams } from 'react-router-dom';
import { ArrowLeft, FileUp, FolderPlus } from 'lucide-react';
import api from '../../lib/api';
import { errorMessage, errorStatus } from '../../lib/errors';
import { parseInteger } from '../../lib/format';
import { parseJSON, type Nest } from '../../lib/types';
import { Alert, Button, Card, CardHeader, Field, LoadingState, PageHeader, Select, Textarea, useToast } from '../../components/ui';
import { NestFormModal } from './shared/NestFormModal';
import { groupNests, rootNests } from './shared/NestGroups';
import { primaryLinkClass } from './shared/ServiceForm';

/** Response of POST /admin/eggs/import: the new egg, plus what the importer adapted or left out. */
interface ImportResponse {
    name?: string;
    import_notes?: string[] | null;
}

interface EggPreview {
    name: string;
    format: 'Atlas' | 'Pterodactyl';
    variables: number;
    images: number;
}

type Inspection = { preview: EggPreview; error?: undefined } | { preview?: undefined; error: string };

/** Reads just enough of the pasted egg to describe it, the same way the API tells the formats apart. */
function inspectEgg(content: string): Inspection {
    const parsed = parseJSON<unknown>(content, undefined);
    if (parsed === undefined) return { error: 'This is not valid JSON. Check that the whole file was pasted.' };
    if (typeof parsed !== 'object' || parsed === null || Array.isArray(parsed)) {
        return { error: 'An egg file is a single JSON object, starting with { and ending with }.' };
    }

    const file = parsed as Record<string, unknown>;
    const images = file.docker_images;
    return {
        preview: {
            name: typeof file.name === 'string' ? file.name.trim() : '',
            format: 'startup_command' in file ? 'Atlas' : 'Pterodactyl',
            variables: Array.isArray(file.variables) ? file.variables.length : 0,
            images: Array.isArray(images) ? images.length : images && typeof images === 'object' ? Object.keys(images).length : 0,
        },
    };
}

export default function ImportEggPage() {
    const navigate = useNavigate();
    const toast = useToast();
    const [searchParams] = useSearchParams();
    const requestedNest = searchParams.get('nest') ?? '';
    const fileInput = useRef<HTMLInputElement>(null);

    const [nests, setNests] = useState<Nest[] | null>(null);
    const [loadError, setLoadError] = useState('');

    const [nestId, setNestId] = useState('');
    const [content, setContent] = useState('');
    const [fileName, setFileName] = useState('');
    const [touched, setTouched] = useState(false);
    const [nestError, setNestError] = useState('');
    const [contentError, setContentError] = useState('');
    const [formError, setFormError] = useState('');
    const [creatingNest, setCreatingNest] = useState(false);
    const [submitting, setSubmitting] = useState(false);
    // Set after an import that came with notes, which stay on screen until the admin moves on.
    const [imported, setImported] = useState<{ name: string; notes: string[] } | null>(null);

    const [attempt, setAttempt] = useState(0);

    useEffect(() => {
        let cancelled = false;
        api.get<Nest[] | null>('/admin/nests')
            .then((res) => {
                if (cancelled) return;
                const list = Array.isArray(res.data) ? res.data : [];
                setNests(list);
                setLoadError('');
                // Keep whatever is already chosen; otherwise use the nest the link asked for, or the only one there is.
                setNestId((current) => {
                    if (current && list.some((nest) => String(nest.id) === current)) return current;
                    if (list.some((nest) => String(nest.id) === requestedNest)) return requestedNest;
                    return list.length === 1 ? String(list[0].id) : '';
                });
            })
            .catch((err: unknown) => {
                if (!cancelled) setLoadError(errorMessage(err, 'Could not load the nests.'));
            });
        return () => {
            cancelled = true;
        };
    }, [attempt, requestedNest]);

    const retry = () => {
        setLoadError('');
        setAttempt((current) => current + 1);
    };

    const groups = useMemo(() => groupNests(nests ?? []), [nests]);
    const inspection = useMemo(() => (content.trim() ? inspectEgg(content) : null), [content]);
    const shownContentError = contentError || (touched && inspection?.error) || '';

    const changeContent = (value: string) => {
        setContent(value);
        setContentError('');
    };

    const chooseFile = async (event: ChangeEvent<HTMLInputElement>) => {
        const file = event.target.files?.[0];
        // Allow the same file to be chosen again after fixing it.
        event.target.value = '';
        if (!file) return;
        try {
            const text = await file.text();
            setFileName(file.name);
            setTouched(true);
            changeContent(text);
        } catch {
            setContentError('That file could not be read. Try pasting its contents instead.');
        }
    };

    const nestCreated = (nest: Nest) => {
        setCreatingNest(false);
        // Add and select the new nest straight away, so the egg cannot end up in another one.
        setNests((current) => (current && !current.some((item) => item.id === nest.id) ? [...current, { ...nest, eggs: [], sub_nests: [] }] : current));
        setNestId(String(nest.id));
        setNestError('');
        toast.success(`${nest.name} was created and is selected.`);
    };

    const submit = async (event: FormEvent) => {
        event.preventDefault();
        if (submitting) return;

        setTouched(true);
        setFormError('');
        const nest = parseInteger(nestId, 0);
        const nextNestError = nest > 0 ? '' : 'Choose the nest to import the egg into.';
        const nextContentError = !content.trim() ? 'Choose a file or paste the egg JSON.' : (inspection?.error ?? '');
        setNestError(nextNestError);
        setContentError(nextContentError);
        if (nextNestError || nextContentError) return;

        setSubmitting(true);
        try {
            const res = await api.post<ImportResponse>('/admin/eggs/import', { nest_id: nest, content });
            const eggName = res.data?.name || 'The egg';
            const notes = Array.isArray(res.data?.import_notes)
                ? res.data.import_notes.filter((note) => typeof note === 'string' && note.trim() !== '')
                : [];
            toast.success(`${eggName} was imported.`);
            if (notes.length === 0) {
                navigate('/admin/eggs');
                return;
            }
            // The notes say what was changed to make the egg fit, and are not stored anywhere.
            setImported({ name: eggName, notes });
            setSubmitting(false);
        } catch (err) {
            const status = errorStatus(err);
            const message = errorMessage(err, 'Could not import the egg.');
            if (status === 400) {
                // The API explains what is wrong with the file, capitalised here to read as a sentence.
                setContentError(message.charAt(0).toUpperCase() + message.slice(1) + (message.endsWith('.') ? '' : '.'));
            } else if (status === 404) {
                setNestError(message);
            } else {
                setFormError(message);
            }
            setSubmitting(false);
        }
    };

    const importAnother = () => {
        setImported(null);
        setContent('');
        setFileName('');
        setTouched(false);
        setContentError('');
    };

    const header = (
        <PageHeader
            title="Import egg"
            description="Add a game from an egg file. Atlas eggs and Pterodactyl eggs both work."
            eyebrow={
                <Link to="/admin/eggs" className="inline-flex items-center gap-1.5 hover:text-foreground">
                    <ArrowLeft size={16} aria-hidden />
                    Eggs
                </Link>
            }
        />
    );

    if (nests === null) {
        return (
            <>
                {header}
                {loadError ? (
                    <Alert
                        tone="danger"
                        title="The nests could not be loaded"
                        actions={
                            <Button size="sm" variant="outline" onClick={retry}>
                                Try again
                            </Button>
                        }
                    >
                        {loadError}
                    </Alert>
                ) : (
                    <LoadingState label="Loading nests…" />
                )}
            </>
        );
    }

    if (imported) {
        return (
            <>
                {header}
                <Alert tone="info" title={`${imported.name} was imported`}>
                    <p>A few things were adapted or left out to make it fit:</p>
                    <ul className="mt-2 list-disc space-y-1 pl-5">
                        {imported.notes.map((note) => (
                            <li key={note}>{note}</li>
                        ))}
                    </ul>
                </Alert>
                <div className="mt-6 flex flex-wrap items-center justify-end gap-2">
                    <Button variant="ghost" onClick={importAnother}>
                        Import another egg
                    </Button>
                    <Link to="/admin/eggs" className={primaryLinkClass}>
                        Go to eggs
                    </Link>
                </div>
            </>
        );
    }

    return (
        <>
            {header}

            <form onSubmit={submit} className="space-y-6" noValidate>
                <Card>
                    <CardHeader title="Nest" description="The group the egg is filed under." />
                    <div className="flex flex-wrap items-start gap-3">
                        <Field
                            label="Import into"
                            className="min-w-0 flex-1 basis-64 sm:max-w-md"
                            error={nestError}
                            hint={nests.length === 0 ? 'There are no nests yet. Create one to import the egg into.' : undefined}
                        >
                            {(field) => (
                                <Select
                                    {...field}
                                    value={nestId}
                                    onChange={(event) => {
                                        setNestId(event.target.value);
                                        setNestError('');
                                    }}
                                >
                                    <option value="">{nests.length === 0 ? 'No nests yet' : 'Choose a nest'}</option>
                                    {groups.map(({ root, children }) =>
                                        children.length === 0 ? (
                                            <option key={root.id} value={root.id}>
                                                {root.name}
                                            </option>
                                        ) : (
                                            <optgroup key={root.id} label={root.name}>
                                                <option value={root.id}>{root.name} (top level)</option>
                                                {children.map((child) => (
                                                    <option key={child.id} value={child.id}>
                                                        {child.name}
                                                    </option>
                                                ))}
                                            </optgroup>
                                        ),
                                    )}
                                </Select>
                            )}
                        </Field>
                        {/* Lines the button up with the select, below the label. */}
                        <div className="pt-[26px]">
                            <Button variant="outline" icon={<FolderPlus size={16} aria-hidden />} onClick={() => setCreatingNest(true)}>
                                New nest
                            </Button>
                        </div>
                    </div>
                </Card>

                <Card>
                    <CardHeader
                        title="Egg file"
                        description="Choose the egg's .json file, or paste its contents below."
                        actions={
                            <>
                                <input
                                    ref={fileInput}
                                    type="file"
                                    accept=".json,application/json"
                                    className="sr-only"
                                    tabIndex={-1}
                                    aria-label="Egg file"
                                    onChange={(event) => void chooseFile(event)}
                                />
                                <Button variant="outline" icon={<FileUp size={16} aria-hidden />} onClick={() => fileInput.current?.click()}>
                                    Choose file
                                </Button>
                            </>
                        }
                    />

                    <Field
                        label="Egg JSON"
                        aside={fileName ? `From ${fileName}` : undefined}
                        error={shownContentError}
                        hint="Nothing is imported until you press Import egg."
                    >
                        {(field) => (
                            <Textarea
                                {...field}
                                rows={14}
                                spellCheck={false}
                                wrap="off"
                                placeholder={'{\n  "name": "…",\n  …\n}'}
                                className="min-h-72 font-mono text-xs leading-relaxed"
                                value={content}
                                onChange={(event) => {
                                    changeContent(event.target.value);
                                    setFileName('');
                                }}
                                onBlur={() => setTouched(true)}
                            />
                        )}
                    </Field>

                    {inspection?.preview && (
                        <dl className="mt-5 grid grid-cols-2 gap-x-6 gap-y-4 rounded-control border border-border bg-secondary/60 p-4 sm:grid-cols-4">
                            <PreviewItem label="Name" value={inspection.preview.name || 'No name'} />
                            <PreviewItem label="Format" value={inspection.preview.format} />
                            <PreviewItem label="Variables" value={String(inspection.preview.variables)} />
                            <PreviewItem label="Docker images" value={String(inspection.preview.images)} />
                        </dl>
                    )}
                </Card>

                {formError && (
                    <Alert tone="danger" title="The egg could not be imported">
                        {formError}
                    </Alert>
                )}

                <div className="flex flex-wrap items-center justify-end gap-2">
                    <Button variant="ghost" onClick={() => navigate('/admin/eggs')} disabled={submitting}>
                        Cancel
                    </Button>
                    <Button type="submit" variant="primary" loading={submitting}>
                        Import egg
                    </Button>
                </div>
            </form>

            {creatingNest && (
                <NestFormModal parents={rootNests(nests)} onClose={() => setCreatingNest(false)} onSaved={nestCreated} />
            )}
        </>
    );
}

function PreviewItem({ label, value }: { label: string; value: string }) {
    return (
        <div className="min-w-0">
            <dt className="text-xs text-muted">{label}</dt>
            <dd className="mt-0.5 truncate text-sm font-medium text-foreground" title={value}>
                {value}
            </dd>
        </div>
    );
}
