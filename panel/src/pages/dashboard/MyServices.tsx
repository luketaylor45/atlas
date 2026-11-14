import { useMemo, useState } from 'react';
import { Search, SearchX } from 'lucide-react';
import ServiceCard, { NoServers } from '../../components/ServiceCard';
import { Alert, Button, Card, EmptyState, Input, LoadingState, PageHeader } from '../../components/ui';
import { useAuth } from '../../context/auth';
import type { Service } from '../../lib/types';
import { useResource } from './useResource';

export default function MyServices() {
    const { user } = useAuth();
    const servers = useResource<Service[]>('/services', 'Could not load your servers.', 10_000);
    const [query, setQuery] = useState('');

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
                        ? `${list.length} ${list.length === 1 ? 'server' : 'servers'} you own or have been given access to.`
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
                ) : (
                    <Card flush className="overflow-hidden">
                        <ul className="divide-y divide-border">
                            {matches.map((service) => (
                                <ServiceCard key={service.uuid} service={service} />
                            ))}
                        </ul>
                    </Card>
                )}
            </div>
        </>
    );
}
