import { useCallback, useEffect, useMemo, useState } from 'react';
import { Link } from 'react-router-dom';
import { Download, Egg as EggIcon, FolderPlus, Pencil, Search, Trash2 } from 'lucide-react';
import api from '../../lib/api';
import { errorMessage } from '../../lib/errors';
import { eggConfigFiles, eggStartupDone, type Egg, type Nest } from '../../lib/types';
import {
    Alert,
    Badge,
    Button,
    Card,
    EmptyState,
    IconButton,
    Input,
    LoadingState,
    PageHeader,
    useConfirm,
    useToast,
} from '../../components/ui';
import { EggEditorModal } from './shared/EggEditorModal';
import { NestFormModal } from './shared/NestFormModal';
import { groupNests, rootNests } from './shared/NestGroups';
import { primaryLinkClass } from './shared/ServiceForm';

interface Catalog {
    eggs: Egg[];
    nests: Nest[];
}

type NestDialog = { mode: 'create' } | { mode: 'edit'; nest: Nest };

const dangerHover = 'hover:bg-danger/10 hover:text-danger';

export default function AdminEggsPage() {
    const toast = useToast();
    const confirm = useConfirm();

    const [catalog, setCatalog] = useState<Catalog | null>(null);
    const [error, setError] = useState('');
    const [search, setSearch] = useState('');
    const [editingEgg, setEditingEgg] = useState<Egg | null>(null);
    const [nestDialog, setNestDialog] = useState<NestDialog | null>(null);
    const [busy, setBusy] = useState<string | null>(null);

    const load = useCallback(async () => {
        try {
            const [eggs, nests] = await Promise.all([api.get<Egg[] | null>('/admin/eggs'), api.get<Nest[] | null>('/admin/nests')]);
            setCatalog({
                eggs: Array.isArray(eggs.data) ? eggs.data : [],
                nests: Array.isArray(nests.data) ? nests.data : [],
            });
            setError('');
        } catch (err) {
            setError(errorMessage(err, 'Could not load the eggs.'));
        }
    }, []);

    useEffect(() => {
        void load();
    }, [load]);

    const retry = () => {
        setError('');
        void load();
    };

    const term = search.trim().toLowerCase();

    const eggsByNest = useMemo(() => {
        const map = new Map<number, Egg[]>();
        for (const egg of catalog?.eggs ?? []) {
            const list = map.get(egg.nest_id) ?? [];
            list.push(egg);
            map.set(egg.nest_id, list);
        }
        for (const list of map.values()) list.sort((a, b) => a.name.localeCompare(b.name));
        return map;
    }, [catalog]);

    const groups = useMemo(() => groupNests(catalog?.nests ?? []), [catalog]);

    const eggsIn = (nest: Nest): Egg[] => {
        const eggs = eggsByNest.get(nest.id) ?? [];
        if (!term) return eggs;
        return eggs.filter((egg) => egg.name.toLowerCase().includes(term) || (egg.description ?? '').toLowerCase().includes(term));
    };

    // While searching, only nests with a matching egg are shown.
    const visibleGroups = term
        ? groups
              .map((group) => ({ ...group, children: group.children.filter((child) => eggsIn(child).length > 0) }))
              .filter((group) => group.children.length > 0 || eggsIn(group.root).length > 0)
        : groups;

    const deleteEgg = async (egg: Egg) => {
        const confirmed = await confirm({
            title: `Delete ${egg.name}?`,
            description: 'The egg and its variables are removed, so no new servers can be created from it. An egg that servers still use cannot be deleted.',
            confirmLabel: 'Delete egg',
            danger: true,
        });
        if (!confirmed) return;

        setBusy(`egg-${egg.id}`);
        try {
            await api.delete(`/admin/eggs/${egg.id}`);
            toast.success(`${egg.name} was deleted.`);
            await load();
        } catch (err) {
            toast.error(errorMessage(err, 'Could not delete the egg.'));
        } finally {
            setBusy(null);
        }
    };

    const deleteNest = async (nest: Nest, children: Nest[]) => {
        const eggCount = [nest, ...children].reduce((sum, item) => sum + (eggsByNest.get(item.id)?.length ?? 0), 0);
        const contents = [
            children.length > 0 ? `${children.length} ${children.length === 1 ? 'sub-nest' : 'sub-nests'}` : '',
            eggCount > 0 ? `${eggCount} ${eggCount === 1 ? 'egg' : 'eggs'}` : '',
        ].filter(Boolean);

        const confirmed = await confirm({
            title: `Delete ${nest.name}?`,
            description:
                contents.length > 0
                    ? `Its ${contents.join(' and ')} are deleted with it. A nest whose eggs are still used by servers cannot be deleted.`
                    : 'This nest is empty, so nothing else is removed.',
            confirmLabel: 'Delete nest',
            danger: true,
        });
        if (!confirmed) return;

        setBusy(`nest-${nest.id}`);
        try {
            await api.delete(`/admin/nests/${nest.id}`);
            toast.success(`${nest.name} was deleted.`);
            await load();
        } catch (err) {
            toast.error(errorMessage(err, 'Could not delete the nest.'));
        } finally {
            setBusy(null);
        }
    };

    const nestSaved = (nest: Nest) => {
        toast.success(nestDialog?.mode === 'edit' ? `${nest.name} was saved.` : `${nest.name} was created.`);
        setNestDialog(null);
        void load();
    };

    const eggSaved = (egg: Egg) => {
        toast.success(`${egg.name} was saved.`);
        setEditingEgg(null);
        void load();
    };

    const nestActions = (nest: Nest, children: Nest[]) => (
        <div className="flex shrink-0 items-center gap-1">
            <IconButton label={`Edit ${nest.name}`} size="sm" onClick={() => setNestDialog({ mode: 'edit', nest })}>
                <Pencil size={16} aria-hidden />
            </IconButton>
            <IconButton
                label={`Delete ${nest.name}`}
                size="sm"
                className={dangerHover}
                loading={busy === `nest-${nest.id}`}
                onClick={() => void deleteNest(nest, children)}
            >
                <Trash2 size={16} aria-hidden />
            </IconButton>
        </div>
    );

    const eggList = (nest: Nest) => {
        const eggs = eggsIn(nest);
        if (eggs.length === 0) {
            return (
                <p className="px-4 py-4 text-sm text-muted sm:px-5">
                    No eggs in this nest yet.{' '}
                    <Link to={`/admin/eggs/import?nest=${nest.id}`} className="font-medium text-primary-text hover:underline">
                        Import an egg
                    </Link>
                </p>
            );
        }
        return (
            <ul className="divide-y divide-border">
                {eggs.map((egg) => {
                    const variableCount = egg.variables?.length ?? 0;
                    const configCount = eggConfigFiles(egg).length;
                    const summary = [
                        `${variableCount} ${variableCount === 1 ? 'variable' : 'variables'}`,
                        configCount > 0 ? `${configCount} ${configCount === 1 ? 'config file' : 'config files'}` : '',
                        eggStartupDone(egg).length > 0 ? 'startup detection' : '',
                    ].filter(Boolean);
                    return (
                        <li key={egg.id} className="flex flex-wrap items-center gap-x-4 gap-y-2 px-4 py-3 sm:flex-nowrap sm:px-5">
                            <div className="min-w-0 flex-1 basis-56">
                                <div className="flex flex-wrap items-center gap-2">
                                    <span className="text-sm font-medium text-foreground">{egg.name}</span>
                                    {egg.customized && (
                                        <span title="Edited here, so catalog updates will not overwrite it.">
                                            <Badge tone="primary">Customized</Badge>
                                        </span>
                                    )}
                                    {egg.source === 'import' && <Badge>Imported</Badge>}
                                </div>
                                {egg.description && <p className="mt-0.5 line-clamp-2 text-sm text-muted">{egg.description}</p>}
                            </div>
                            <span className="tabular shrink-0 whitespace-nowrap text-sm text-muted">{summary.join(' · ')}</span>
                            <div className="ml-auto flex shrink-0 items-center gap-1">
                                <IconButton label={`Edit ${egg.name}`} size="sm" onClick={() => setEditingEgg(egg)}>
                                    <Pencil size={16} aria-hidden />
                                </IconButton>
                                <IconButton
                                    label={`Delete ${egg.name}`}
                                    size="sm"
                                    className={dangerHover}
                                    loading={busy === `egg-${egg.id}`}
                                    onClick={() => void deleteEgg(egg)}
                                >
                                    <Trash2 size={16} aria-hidden />
                                </IconButton>
                            </div>
                        </li>
                    );
                })}
            </ul>
        );
    };

    return (
        <>
            <PageHeader
                title="Eggs"
                description="An egg describes how to install and run one game. Nests keep related eggs together."
                actions={
                    <>
                        <Button variant="outline" icon={<FolderPlus size={16} aria-hidden />} onClick={() => setNestDialog({ mode: 'create' })}>
                            New nest
                        </Button>
                        <Link to="/admin/eggs/import" className={primaryLinkClass}>
                            <Download size={16} aria-hidden />
                            Import egg
                        </Link>
                    </>
                }
            />

            {catalog === null && !error && <LoadingState label="Loading eggs…" />}

            {error && (
                <Alert
                    tone="danger"
                    title={catalog === null ? 'The eggs could not be loaded' : 'The list could not be refreshed'}
                    className="mb-4"
                    actions={
                        <Button size="sm" variant="outline" onClick={retry}>
                            Try again
                        </Button>
                    }
                >
                    {error}
                </Alert>
            )}

            {catalog !== null && groups.length === 0 && (
                <Card flush>
                    <EmptyState
                        icon={<EggIcon size={18} aria-hidden />}
                        title="No nests yet"
                        description="Create a nest, then import an egg into it."
                        action={
                            <Button variant="outline" icon={<FolderPlus size={16} aria-hidden />} onClick={() => setNestDialog({ mode: 'create' })}>
                                New nest
                            </Button>
                        }
                    />
                </Card>
            )}

            {catalog !== null && groups.length > 0 && (
                <div className="space-y-6">
                    <div className="relative w-full sm:max-w-xs">
                        <Search size={16} className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-subtle" aria-hidden />
                        <Input
                            type="search"
                            aria-label="Search eggs"
                            placeholder="Search eggs"
                            value={search}
                            onChange={(event) => setSearch(event.target.value)}
                            className="pl-9"
                        />
                    </div>

                    {visibleGroups.length === 0 && (
                        <Card flush>
                            <EmptyState
                                icon={<Search size={18} aria-hidden />}
                                title="No eggs match your search"
                                description="Search looks at the name and description of each egg."
                                action={
                                    <Button variant="outline" onClick={() => setSearch('')}>
                                        Clear search
                                    </Button>
                                }
                            />
                        </Card>
                    )}

                    {visibleGroups.map(({ root, children }) => {
                        const allChildren = groups.find((group) => group.root.id === root.id)?.children ?? children;
                        // A root nest that only holds sub-nests does not need an empty "no eggs" line of its own.
                        const showRootEggs = eggsIn(root).length > 0 || (!term && children.length === 0);
                        return (
                            <Card key={root.id} flush className="overflow-hidden">
                                <div className="flex items-start justify-between gap-3 px-4 py-4 sm:px-5">
                                    <div className="min-w-0">
                                        <h2 className="text-base font-semibold tracking-tight text-foreground">{root.name}</h2>
                                        {root.description && <p className="mt-1 text-sm text-muted">{root.description}</p>}
                                    </div>
                                    {nestActions(root, allChildren)}
                                </div>

                                {showRootEggs && <div className="border-t border-border">{eggList(root)}</div>}

                                {children.map((child) => (
                                    <section key={child.id} className="border-t border-border">
                                        <div className="flex items-start justify-between gap-3 border-b border-border bg-secondary/60 px-4 py-2.5 sm:px-5">
                                            <div className="min-w-0">
                                                <h3 className="text-sm font-medium text-foreground">{child.name}</h3>
                                                {child.description && <p className="mt-0.5 text-xs text-muted">{child.description}</p>}
                                            </div>
                                            {nestActions(child, [])}
                                        </div>
                                        {eggList(child)}
                                    </section>
                                ))}
                            </Card>
                        );
                    })}
                </div>
            )}

            {nestDialog && (
                <NestFormModal
                    key={nestDialog.mode === 'edit' ? `edit-${nestDialog.nest.id}` : 'create'}
                    nest={nestDialog.mode === 'edit' ? nestDialog.nest : undefined}
                    parents={rootNests(catalog?.nests ?? [])}
                    onClose={() => setNestDialog(null)}
                    onSaved={nestSaved}
                />
            )}

            {editingEgg && <EggEditorModal key={editingEgg.id} egg={editingEgg} onClose={() => setEditingEgg(null)} onSaved={eggSaved} />}
        </>
    );
}
