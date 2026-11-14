import { Link } from 'react-router-dom';
import { Activity, Megaphone, Play, Server } from 'lucide-react';
import ServiceCard, { NoServers } from '../../components/ServiceCard';
import {
    Alert,
    Badge,
    Button,
    Card,
    CardHeader,
    EmptyState,
    PageHeader,
    Skeleton,
    StatTile,
    type Tone,
} from '../../components/ui';
import { useAuth } from '../../context/auth';
import { formatDateTime, formatRelative } from '../../lib/format';
import type { NewsItem, Service } from '../../lib/types';
import { useResource, type Resource } from './useResource';

interface Overview {
    total_services: number;
    running_services: number;
    health: 'OPTIMAL' | 'DEGRADED' | string;
}

const SERVERS_SHOWN = 5;
const NEWS_SHOWN = 5;

const newsTones: Record<string, Tone> = {
    release: 'primary',
    status: 'info',
    maintenance: 'warning',
};

function RetryAlert({ resource, className }: { resource: Resource<unknown>; className?: string }) {
    if (!resource.error) return null;
    return (
        <Alert
            tone="danger"
            className={className}
            actions={
                <Button size="sm" variant="outline" loading={resource.retrying} onClick={resource.retry}>
                    Try again
                </Button>
            }
        >
            {resource.error}
        </Alert>
    );
}

function OverviewTiles({ overview }: { overview: Resource<Overview> }) {
    if (overview.loading) {
        return (
            <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
                {[0, 1, 2].map((index) => (
                    <Skeleton key={index} className="h-[104px] rounded-card" />
                ))}
            </div>
        );
    }

    const data = overview.data;
    const healthy = data?.health === 'OPTIMAL';

    return (
        <div className="space-y-4">
            <RetryAlert resource={overview} />
            {data && (
                <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
                    <StatTile label="Servers" value={data.total_services} icon={<Server size={16} aria-hidden />} />
                    <StatTile
                        label="Running"
                        value={data.running_services}
                        detail={`of ${data.total_services}`}
                        icon={<Play size={16} aria-hidden />}
                    />
                    <StatTile
                        label="Health"
                        value={
                            <span className={healthy ? 'block text-lg leading-8' : 'block text-lg leading-8 text-warning'}>
                                {healthy ? 'All nodes online' : 'A node is offline'}
                            </span>
                        }
                        detail={healthy ? undefined : 'Servers on that node cannot be reached right now.'}
                        icon={<Activity size={16} aria-hidden />}
                    />
                </div>
            )}
        </div>
    );
}

function ServersCard({ servers, isAdmin }: { servers: Resource<Service[]>; isAdmin: boolean }) {
    const list = servers.data ?? [];

    return (
        <Card flush className="lg:col-span-2">
            <CardHeader
                title="Your servers"
                className="mb-0 px-5 pb-4 pt-5 sm:px-6"
                actions={
                    list.length > 0 && (
                        <Link to="/services" className="text-sm font-medium text-primary-text hover:underline">
                            {list.length > SERVERS_SHOWN ? `View all ${list.length}` : 'View all'}
                        </Link>
                    )
                }
            />

            <RetryAlert resource={servers} className="mx-5 mb-4 sm:mx-6" />

            {servers.loading ? (
                <div className="space-y-3 px-5 pb-5 sm:px-6" role="status" aria-label="Loading servers">
                    {[0, 1, 2].map((index) => (
                        <Skeleton key={index} className="h-12" />
                    ))}
                </div>
            ) : list.length > 0 ? (
                <ul className="divide-y divide-border border-t border-border">
                    {list.slice(0, SERVERS_SHOWN).map((service) => (
                        <ServiceCard key={service.uuid} service={service} compact />
                    ))}
                </ul>
            ) : (
                !servers.error && <NoServers isAdmin={isAdmin} />
            )}
        </Card>
    );
}

function NewsCard({ news, isAdmin }: { news: Resource<NewsItem[]>; isAdmin: boolean }) {
    const list = news.data ?? [];

    return (
        <Card>
            <CardHeader title="Announcements" />

            <RetryAlert resource={news} className="mb-4" />

            {news.loading ? (
                <div className="space-y-3" role="status" aria-label="Loading announcements">
                    <Skeleton className="h-4 w-2/3" />
                    <Skeleton className="h-4" />
                    <Skeleton className="h-4 w-5/6" />
                </div>
            ) : list.length > 0 ? (
                <ul className="-my-4 divide-y divide-border">
                    {list.slice(0, NEWS_SHOWN).map((item) => (
                        <li key={item.id} className="py-4">
                            <div className="flex flex-wrap items-center gap-x-2.5 gap-y-1">
                                {item.type && <Badge tone={newsTones[item.type.toLowerCase()] ?? 'neutral'}>{item.type}</Badge>}
                                <time
                                    dateTime={item.created_at}
                                    title={formatDateTime(item.created_at)}
                                    className="text-xs text-muted"
                                >
                                    {formatRelative(item.created_at)}
                                </time>
                            </div>
                            <h3 className="mt-2 break-words text-sm font-medium text-foreground">{item.title}</h3>
                            <p className="mt-1 whitespace-pre-line break-words text-sm text-muted">{item.content}</p>
                        </li>
                    ))}
                </ul>
            ) : (
                !news.error && (
                    <EmptyState
                        className="py-8"
                        icon={<Megaphone size={18} aria-hidden />}
                        title="No announcements"
                        description={
                            isAdmin ? (
                                <>
                                    Post one from{' '}
                                    <Link to="/admin/news" className="font-medium text-primary-text hover:underline">
                                        Announcements
                                    </Link>{' '}
                                    to tell everyone about updates or maintenance.
                                </>
                            ) : (
                                'Updates and maintenance notices from your administrator will appear here.'
                            )
                        }
                    />
                )
            )}
        </Card>
    );
}

export default function Dashboard() {
    const { user } = useAuth();
    const isAdmin = Boolean(user?.is_admin);

    // Three independent requests: one failing never blanks the others.
    const overview = useResource<Overview>('/services/overview', 'Could not load your server summary.', 10_000);
    const servers = useResource<Service[]>('/services', 'Could not load your servers.', 10_000);
    const news = useResource<NewsItem[]>('/news', 'Could not load announcements.', 60_000);

    return (
        <>
            <PageHeader
                title={user?.username ? `Welcome back, ${user.username}` : 'Welcome back'}
                description="Your servers at a glance, and the latest announcements."
            />

            <div className="space-y-6">
                <OverviewTiles overview={overview} />

                <div className="grid grid-cols-1 items-start gap-6 lg:grid-cols-3">
                    <ServersCard servers={servers} isAdmin={isAdmin} />
                    <NewsCard news={news} isAdmin={isAdmin} />
                </div>
            </div>
        </>
    );
}
