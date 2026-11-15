import { useMemo, useState } from 'react';
import { LayoutGrid, List, Search, SearchX } from 'lucide-react';
import ServerTile, { NoServers, ServerRow } from '../../components/ServiceCard';
import { Alert, Button, Card, EmptyState, Input, LoadingState, PageHeader } from '../../components/ui';
import { useAuth } from '../../context/auth';
import { useTheme } from '../../context/theme';
import { cn } from '../../lib/cn';
import { serviceStatus } from '../../lib/status';
import type { Service } from '../../lib/types';
import { useResource } from './useResource';

type View = 'cards' | 'list';
const VIEW_STORAGE_KEY = 'atlas-servers-view';

function readView(): View | null {
    try {
        const stored = localStorage.getItem(VIEW_STORAGE_KEY);
        return stored === 'cards' || stored === 'list' ? stored : null;
    } catch {
        return null;
    }
}

const views: { value: View; label: string; icon: typeof List }[] = [
    { value: 'cards', label: 'Cards', icon: LayoutGrid },
    { value: 'list', label: 'List', icon: List },
];

/** Cards or a dense list. Until someone picks, the Compact theme starts on the list. */
function useView(): [View, (view: View) => void] {
    const { layout } = useTheme();
    const [chosen, setChosen] = useState<View | null>(readView);
    const choose = (view: View) => {
        setChosen(view);
        try {
            localStorage.setItem(VIEW_STORAGE_KEY, view);
        } catch {
            // The choice still applies for this visit.
        }
    };
    return [chosen ?? (layout === 'compact' ? 'list' : 'cards'), choose];
}

/** Counts by status, e.g. "3 running · 1 installing". */
function statusSummary(list: Service[]): string {
    const counts = new Map<string, number>();
    for (const service of list) {
        const label = serviceStatus(service.status, service.is_suspended).label.toLowerCase();
        counts.set(label, (counts.get(label) ?? 0) + 1);
    }
    return Array.from(counts, ([label, count]) => `${count} ${label}`).join(' · ');
}

export default function MyServices() {
    const { user } = useAuth();
    const servers = useResource<Service[]>('/services', 'Could not load your servers.', 10_000);
    const [query, setQuery] = useState('');
    const [view, setView] = useView();

    const list = useMemo(() => servers.data ?? [], [servers.data]);
    const term = query.trim().toLowerCase();
    const matches = useMemo(
        () => (term ? list.filter((service) => service.name.toLowerCase().includes(term)) : list),
        [list, term],
    );

    return (
        <>
            <PageHeader
                title="Servers"
                description={
                    list.length > 0
                        ? `${list.length} ${list.length === 1 ? 'server' : 'servers'} you own or have been given access to: ${statusSummary(list)}.`
                        : 'Servers you own or have been given access to.'
                }
            />

            <div className="space-y-4">
                {servers.error && (
                    <Alert
                        tone="danger"
                        actions={
                            <Button size="sm" variant="outline" loading={servers.retrying} onClick={servers.retry}>
                                Try again
                            </Button>
                        }
                    >
                        {servers.error}
                    </Alert>
                )}

                {list.length > 0 && (
                    <div className="flex flex-wrap items-center justify-between gap-3">
                        <div className="relative w-full sm:max-w-xs" role="search">
                            <label htmlFor="server-search" className="sr-only">
                                Search servers by name
                            </label>
                            <Search
                                size={16}
                                className="pointer-events-none absolute inset-y-0 left-3 my-auto text-subtle"
                                aria-hidden
                            />
                            <Input
                                id="server-search"
                                type="search"
                                placeholder="Search servers"
                                autoComplete="off"
                                className="pl-9"
                                value={query}
                                onChange={(event) => setQuery(event.target.value)}
                            />
                        </div>

                        <div
                            role="radiogroup"
                            aria-label="Show servers as"
                            className="inline-flex items-center gap-0.5 rounded-control border border-border bg-secondary p-0.5"
                        >
                            {views.map(({ value, label, icon: Icon }) => (
                                <button
                                    key={value}
                                    type="button"
                                    role="radio"
                                    aria-checked={view === value}
                                    onClick={() => setView(value)}
                                    className={cn(
                                        'inline-flex h-7 items-center gap-1.5 rounded-[calc(var(--control-radius)-2px)] px-2.5 text-xs font-medium transition-colors',
                                        view === value ? 'bg-card text-foreground shadow-card' : 'text-muted hover:text-foreground',
                                    )}
                                >
                                    <Icon size={14} aria-hidden />
                                    {label}
                                </button>
                            ))}
                        </div>
                    </div>
                )}

                {servers.loading ? (
                    <LoadingState label="Loading servers…" />
                ) : list.length === 0 ? (
                    !servers.error && (
                        <Card flush>
                            <NoServers isAdmin={Boolean(user?.is_admin)} />
                        </Card>
                    )
                ) : matches.length === 0 ? (
                    <Card flush>
                        <EmptyState
                            icon={<SearchX size={18} aria-hidden />}
                            title="No servers match your search"
                            description={`Nothing is named like “${query.trim()}”.`}
                            action={<Button onClick={() => setQuery('')}>Clear search</Button>}
                        />
                    </Card>
                ) : view === 'cards' ? (
                    <ul className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4">
                        {matches.map((service) => (
                            <ServerTile key={service.uuid} service={service} />
                        ))}
                    </ul>
                ) : (
                    <Card flush className="overflow-hidden">
                        <ul className="divide-y divide-border">
                            {matches.map((service) => (
                                <ServerRow key={service.uuid} service={service} />
                            ))}
                        </ul>
                    </Card>
                )}
            </div>
        </>
    );
}
