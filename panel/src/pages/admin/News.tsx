import { useId, useState, type FormEvent } from 'react';
import { Bell, Pencil, Plus, Trash2 } from 'lucide-react';
import {
    Alert,
    Badge,
    Button,
    Card,
    EmptyState,
    Field,
    IconButton,
    Input,
    LoadingState,
    Modal,
    PageHeader,
    Select,
    Textarea,
    useConfirm,
    useToast,
    type Tone,
} from '../../components/ui';
import api from '../../lib/api';
import { errorMessage } from '../../lib/errors';
import { formatDateTime, formatRelative } from '../../lib/format';
import type { NewsItem } from '../../lib/types';
import { usePolling } from '../../lib/usePolling';

// The values are what the API stores; the dashboard reads them too.
const newsTypes: { value: string; label: string; tone: Tone }[] = [
    { value: 'Release', label: 'Release or update', tone: 'primary' },
    { value: 'Status', label: 'System status', tone: 'info' },
    { value: 'Maintenance', label: 'Maintenance', tone: 'warning' },
];

function typeInfo(type: string): { label: string; tone: Tone } {
    return newsTypes.find((item) => item.value === type) ?? { label: type || 'General', tone: 'neutral' };
}

interface NewsFormErrors {
    title?: string;
    content?: string;
}

function NewsModal({ item, onClose, onSaved }: { item: NewsItem | null; onClose: () => void; onSaved: (item: NewsItem) => void }) {
    const toast = useToast();
    const formId = useId();
    const [title, setTitle] = useState(item?.title ?? '');
    const [type, setType] = useState(item?.type || newsTypes[0].value);
    const [content, setContent] = useState(item?.content ?? '');
    const [errors, setErrors] = useState<NewsFormErrors>({});
    const [formError, setFormError] = useState<string | null>(null);
    const [saving, setSaving] = useState(false);

    // An announcement saved with a type this form does not list keeps it.
    const options = newsTypes.some((option) => option.value === type)
        ? newsTypes
        : [...newsTypes, { value: type, label: type, tone: 'neutral' as Tone }];

    const submit = async (event: FormEvent<HTMLFormElement>) => {
        event.preventDefault();
        if (saving) return;

        const problems: NewsFormErrors = {};
        if (!title.trim()) problems.title = 'Give the announcement a title.';
        else if (title.trim().length > 255) problems.title = 'Use at most 255 characters.';
        if (!content.trim()) problems.content = 'Write what you want to tell your users.';
        setErrors(problems);
        if (problems.title || problems.content) return;

        const payload = { title: title.trim(), content: content.trim(), type };
        setSaving(true);
        setFormError(null);
        try {
            const res = item
                ? await api.put<NewsItem>(`/admin/news/${item.id}`, payload)
                : await api.post<NewsItem>('/admin/news', payload);
            toast.success(item ? 'Announcement saved' : 'Announcement posted');
            onSaved(res.data);
        } catch (err) {
            setFormError(errorMessage(err, item ? 'Could not save the announcement.' : 'Could not post the announcement.'));
            setSaving(false);
        }
    };

    return (
        <Modal
            open
            onClose={onClose}
            title={item ? 'Edit announcement' : 'New announcement'}
            description="Announcements are shown to everyone on their dashboard."
            size="lg"
            dismissible={!saving}
            footer={
                <>
                    <Button variant="ghost" onClick={onClose} disabled={saving}>
                        Cancel
                    </Button>
                    <Button variant="primary" type="submit" form={formId} loading={saving}>
                        {item ? 'Save changes' : 'Post announcement'}
                    </Button>
                </>
            }
        >
            <form id={formId} onSubmit={submit} noValidate className="space-y-4">
                {formError && <Alert tone="danger">{formError}</Alert>}

                <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
                    <Field label="Title" error={errors.title} className="sm:col-span-2">
                        {(field) => (
                            <Input
                                {...field}
                                value={title}
                                onChange={(event) => {
                                    setTitle(event.target.value);
                                    setErrors((current) => ({ ...current, title: undefined }));
                                }}
                                placeholder="Scheduled maintenance on Sunday"
                                maxLength={255}
                            />
                        )}
                    </Field>

                    <Field label="Type">
                        {(field) => (
                            <Select {...field} value={type} onChange={(event) => setType(event.target.value)}>
                                {options.map((option) => (
                                    <option key={option.value} value={option.value}>
                                        {option.label}
                                    </option>
                                ))}
                            </Select>
                        )}
                    </Field>
                </div>

                <Field label="Content" error={errors.content} hint="Plain text. Line breaks are kept.">
                    {(field) => (
                        <Textarea
                            {...field}
                            value={content}
                            onChange={(event) => {
                                setContent(event.target.value);
                                setErrors((current) => ({ ...current, content: undefined }));
                            }}
                            rows={7}
                        />
                    )}
                </Field>
            </form>
        </Modal>
    );
}

type ModalState = { mode: 'create' } | { mode: 'edit'; item: NewsItem } | null;

export default function AdminNewsPage() {
    const toast = useToast();
    const confirm = useConfirm();
    const [news, setNews] = useState<NewsItem[] | null>(null);
    const [error, setError] = useState<string | null>(null);
    const [modal, setModal] = useState<ModalState>(null);
    const [deletingId, setDeletingId] = useState<number | null>(null);

    const load = async () => {
        try {
            const res = await api.get<NewsItem[]>('/news');
            setNews(Array.isArray(res.data) ? res.data : []);
            setError(null);
        } catch (err) {
            setError(errorMessage(err, 'Could not load announcements.'));
        }
    };

    usePolling(load, 30000);

    const retry = () => {
        setError(null);
        void load();
    };

    const deleteItem = async (item: NewsItem) => {
        const confirmed = await confirm({
            title: 'Delete this announcement?',
            description: `"${item.title}" is removed from everyone's dashboard. This cannot be undone.`,
            confirmLabel: 'Delete announcement',
            danger: true,
        });
        if (!confirmed) return;

        setDeletingId(item.id);
        try {
            await api.delete(`/admin/news/${item.id}`);
            setNews((current) => current?.filter((entry) => entry.id !== item.id) ?? current);
            toast.success('Announcement deleted');
        } catch (err) {
            toast.error(errorMessage(err, 'Could not delete the announcement.'));
        } finally {
            setDeletingId(null);
        }
    };

    const saved = (item: NewsItem) => {
        setNews((current) => {
            if (!current) return current;
            // New announcements go first, matching the API's newest-first order.
            return current.some((entry) => entry.id === item.id)
                ? current.map((entry) => (entry.id === item.id ? item : entry))
                : [item, ...current];
        });
        setModal(null);
    };

    const header = (
        <PageHeader
            title="Announcements"
            description="News shown to everyone on their dashboard."
            actions={
                <Button variant="primary" icon={<Plus size={16} aria-hidden />} onClick={() => setModal({ mode: 'create' })}>
                    New announcement
                </Button>
            }
        />
    );

    const modalElement = modal && (
        <NewsModal
            key={modal.mode === 'edit' ? modal.item.id : 'create'}
            item={modal.mode === 'edit' ? modal.item : null}
            onClose={() => setModal(null)}
            onSaved={saved}
        />
    );

    if (!news) {
        return (
            <>
                {header}
                {error ? (
                    <Alert tone="danger" title="Could not load announcements" actions={<Button size="sm" onClick={retry}>Try again</Button>}>
                        {error}
                    </Alert>
                ) : (
                    <LoadingState label="Loading announcements…" />
                )}
                {modalElement}
            </>
        );
    }

    return (
        <>
            {header}

            <div className="space-y-4">
                {error && (
                    <Alert tone="warning" title="Could not refresh" actions={<Button size="sm" onClick={retry}>Try again</Button>}>
                        {error} Showing the last list received.
                    </Alert>
                )}

                <Card flush>
                    {news.length === 0 ? (
                        <EmptyState
                            icon={<Bell size={18} aria-hidden />}
                            title="No announcements yet"
                            description="Post one to tell your users about updates, maintenance or outages."
                            action={<Button onClick={() => setModal({ mode: 'create' })}>New announcement</Button>}
                        />
                    ) : (
                        <ul className="divide-y divide-border">
                            {news.map((item) => {
                                const info = typeInfo(item.type);
                                return (
                                    <li key={item.id} className="flex items-start gap-3 px-5 py-4 sm:px-6">
                                        <div className="min-w-0 flex-1">
                                            <div className="flex flex-wrap items-center gap-x-2 gap-y-1">
                                                <h2 className="min-w-0 break-words text-sm font-medium text-foreground">{item.title}</h2>
                                                <Badge tone={info.tone}>{info.label}</Badge>
                                            </div>
                                            <time
                                                dateTime={item.created_at}
                                                title={formatDateTime(item.created_at)}
                                                className="mt-0.5 block text-xs text-muted"
                                            >
                                                {formatRelative(item.created_at)}
                                            </time>
                                            <p className="mt-2 line-clamp-2 whitespace-pre-line break-words text-sm text-muted">
                                                {item.content}
                                            </p>
                                        </div>
                                        <div className="flex shrink-0 items-center gap-1">
                                            <IconButton
                                                label={`Edit ${item.title}`}
                                                size="sm"
                                                disabled={deletingId === item.id}
                                                onClick={() => setModal({ mode: 'edit', item })}
                                            >
                                                <Pencil size={16} aria-hidden />
                                            </IconButton>
                                            <IconButton
                                                label={`Delete ${item.title}`}
                                                size="sm"
                                                className="hover:bg-danger/10 hover:text-danger"
                                                loading={deletingId === item.id}
                                                onClick={() => void deleteItem(item)}
                                            >
                                                <Trash2 size={16} aria-hidden />
                                            </IconButton>
                                        </div>
                                    </li>
                                );
                            })}
                        </ul>
                    )}
                </Card>
            </div>

            {modalElement}
        </>
    );
}
