import { Link } from 'react-router-dom';
import {
    Activity,
    ArrowRight,
    Megaphone,
    MemoryStick,
    Play,
    Plus,
    Server,
    Sparkles,
    TriangleAlert,
    Wrench,
    type LucideIcon,
} from 'lucide-react';
import OrbitArt from '../../components/OrbitArt';
import ServerTile, { NoServers } from '../../components/ServiceCard';
import { Alert, Badge, Button, Card, CardHeader, EmptyState, Skeleton, StatTile, type Tone } from '../../components/ui';
import { useAuth } from '../../context/auth';
import { cn } from '../../lib/cn';
import { formatDateTime, formatMB, formatRelative } from '../../lib/format';
import type { NewsItem, Service } from '../../lib/types';
import { useResource, type Resource } from './useResource';

interface Overview {
    total_services: number;
    running_services: number;
    health: 'OPTIMAL' | 'DEGRADED' | string;
}

const SERVERS_SHOWN = 6;
const NEWS_SHOWN = 5;

const newsKinds: Record<string, { tone: Tone; icon: LucideIcon }> = {
    release: { tone: 'primary', icon: Sparkles },
    status: { tone: 'info', icon: Activity },
    maintenance: { tone: 'warning', icon: Wrench },
};

const newsIconTones: Record<Tone, string> = {
    neutral: 'bg-secondary text-muted',
    primary: 'bg-primary/12 text-primary-text',
    success: 'bg-success/12 text-success',
    warning: 'bg-warning/12 text-warning',
    danger: 'bg-danger/12 text-danger',
    info: 'bg-info/12 text-info',
};

const linkButton =
    'inline-flex h-9 items-center justify-center gap-2 whitespace-nowrap rounded-control px-4 text-sm font-medium transition-colors';

function greeting(date: Date): string {
    const hour = date.getHours();
    if (hour < 5) return 'Good evening';
    if (hour < 12) return 'Good morning';
    if (hour < 18) return 'Good afternoon';
    return 'Good evening';
}

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

/** The greeting, a one-line summary and the main actions, over the orbit art. */
function Hero({ name, overview, isAdmin }: { name?: string; overview: Overview | null; isAdmin: boolean }) {
    const now = new Date();
    const total = Number(overview?.total_services) || 0;
    const running = Number(overview?.running_services) || 0;

    let summary = 'Your servers at a glance, and the latest announcements.';
    if (overview) {
        summary =
            total === 0
                ? 'You have no servers yet.'
                : `${running} of ${total} ${total === 1 ? 'server is' : 'servers are'} running` +
                  (overview.health === 'OPTIMAL' ? ', and their nodes are all online.' : '. A node hosting one of them is offline.');
    }

    return (
        <section
            aria-labelledby="dashboard-greeting"
            className="surface relative overflow-hidden rounded-card border border-border bg-card shadow-card"
        >
            <div aria-hidden className="hero-glow absolute inset-0" />
            <OrbitArt className="absolute -right-16 -top-24 hidden h-[22rem] w-[22rem] text-primary-text opacity-70 sm:block" />

            <div className="relative flex flex-wrap items-end justify-between gap-6 p-5 sm:p-8">
                <div className="min-w-0 max-w-xl">
                    <p className="text-sm font-medium text-primary-text">
                        {now.toLocaleDateString([], { weekday: 'long', day: 'numeric', month: 'long' })}
                    </p>
                    <h1 id="dashboard-greeting" className="page-title mt-1.5 break-words text-foreground">
                        {greeting(now)}
                        {name ? `, ${name}` : ''}
                    </h1>
                    <p className="mt-2 text-sm text-muted">{summary}</p>
                </div>

                <div className="flex flex-wrap gap-2">
                    <Link
                        to="/services"
                        className={cn(linkButton, 'border border-border-strong bg-card text-foreground hover:bg-secondary')}
                    >
                        All servers
                        <ArrowRight size={16} aria-hidden />
                    </Link>
                    {isAdmin && (
                        <Link
                            to="/admin/services/create"
                            className={cn(linkButton, 'bg-primary text-primary-foreground shadow-card hover:bg-primary-hover')}
                        >
                            <Plus size={16} aria-hidden />
                            New server
                        </Link>
                    )}
                </div>
            </div>
        </section>
    );
}

function OverviewTiles({ overview, servers }: { overview: Resource<Overview>; servers: Resource<Service[]> }) {
    if (overview.loading) {
        return (
            <div className="grid grid-cols-2 gap-4 lg:grid-cols-4">
                {[0, 1, 2, 3].map((index) => (
                    <Skeleton key={index} className="h-[108px] rounded-card" />
                ))}
            </div>
        );
    }

    const data = overview.data;
    const healthy = data?.health === 'OPTIMAL';
    const list = servers.data;
    const memory = list?.reduce((sum, service) => sum + (Number(service.memory) || 0), 0) ?? null;
    const problems = list?.filter((service) => service.status === 'installation_failed').length ?? 0;

    return (
        <div className="space-y-4">
            <RetryAlert resource={overview} />
            {data && (
                <div className="grid grid-cols-2 gap-4 lg:grid-cols-4">
                    <StatTile
                        label="Servers"
                        value={data.total_services}
                        detail={problems > 0 ? <span className="text-danger">{problems} failed to install</span> : 'Owned or shared with you'}
                        icon={<Server size={16} aria-hidden />}
                    />
                    <StatTile
                        label="Running"
                        value={data.running_services}
                        detail={`of ${data.total_services}`}
                        tone="success"
                        icon={<Play size={16} aria-hidden />}
                    />
                    <StatTile
                        label="Memory"
                        value={memory === null ? '—' : formatMB(memory)}
                        detail="Allocated to your servers"
                        tone="info"
                        icon={<MemoryStick size={16} aria-hidden />}
                    />
                    <StatTile
                        label="Health"
                        value={
                            <span className={cn('block text-lg leading-8', !healthy && 'text-warning')}>
                                {healthy ? 'All online' : 'Node offline'}
                            </span>
                        }
                        detail={healthy ? 'Every node your servers use is reachable' : 'Servers on it cannot be reached right now.'}
                        tone={healthy ? 'success' : 'warning'}
                        icon={healthy ? <Activity size={16} aria-hidden /> : <TriangleAlert size={16} aria-hidden />}
                    />
                </div>
            )}
        </div>
    );
}

function ServersSection({ servers, isAdmin }: { servers: Resource<Service[]>; isAdmin: boolean }) {
    const list = servers.data ?? [];

    return (
        <section aria-labelledby="dashboard-servers" className="min-w-0 xl:col-span-2">
            <div className="mb-4 flex items-center justify-between gap-3">
                <h2 id="dashboard-servers" className="text-base font-semibold tracking-tight text-foreground">
                    Your servers
                </h2>
                {list.length > 0 && (
                    <Link to="/services" className="inline-flex items-center gap-1 text-sm font-medium text-primary-text hover:underline">
                        {list.length > SERVERS_SHOWN ? `View all ${list.length}` : 'View all'}
                        <ArrowRight size={14} aria-hidden />
                    </Link>
                )}
            </div>

            <RetryAlert resource={servers} className="mb-4" />

            {servers.loading ? (
                <div className="grid grid-cols-1 gap-4 sm:grid-cols-2" role="status" aria-label="Loading servers">
                    {[0, 1].map((index) => (
                        <Skeleton key={index} className="h-64 rounded-card" />
                    ))}
                </div>
            ) : list.length > 0 ? (
                <ul className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                    {list.slice(0, SERVERS_SHOWN).map((service) => (
                        <ServerTile key={service.uuid} service={service} />
                    ))}
                </ul>
            ) : (
                !servers.error && (
                    <Card flush>
                        <NoServers isAdmin={isAdmin} />
                    </Card>
                )
            )}
        </section>
    );
}

function NewsFeed({ news, isAdmin }: { news: Resource<NewsItem[]>; isAdmin: boolean }) {
    const list = news.data ?? [];

    return (
        <Card className="min-w-0">
            <CardHeader
                title="Announcements"
                icon={<Megaphone size={18} aria-hidden />}
                actions={isAdmin && list.length > 0 && (
                    <Link to="/admin/news" className="text-sm font-medium text-primary-text hover:underline">
                        Manage
                    </Link>
                )}
            />

            <RetryAlert resource={news} className="mb-4" />

            {news.loading ? (
                <div className="space-y-3" role="status" aria-label="Loading announcements">
                    <Skeleton className="h-4 w-2/3" />
                    <Skeleton className="h-4" />
                    <Skeleton className="h-4 w-5/6" />
                </div>
            ) : list.length > 0 ? (
                // A timeline: a rail down the left with an icon per announcement.
                <ol className="relative space-y-6 before:absolute before:bottom-3 before:left-4 before:top-3 before:w-px before:-translate-x-1/2 before:bg-border">
                    {list.slice(0, NEWS_SHOWN).map((item) => {
                        const kind = newsKinds[(item.type || '').toLowerCase()] ?? { tone: 'neutral' as Tone, icon: Megaphone };
                        const Icon = kind.icon;
                        return (
                            <li key={item.id} className="relative flex gap-3.5">
                                <span
                                    className={cn(
                                        'relative flex h-8 w-8 shrink-0 items-center justify-center rounded-full ring-4 ring-card',
                                        newsIconTones[kind.tone],
                                    )}
                                    aria-hidden
                                >
                                    <Icon size={15} />
                                </span>
                                <div className="min-w-0 flex-1 pt-1">
                                    <div className="flex flex-wrap items-center gap-x-2.5 gap-y-1">
                                        {item.type && <Badge tone={kind.tone}>{item.type}</Badge>}
                                        <time dateTime={item.created_at} title={formatDateTime(item.created_at)} className="text-xs text-muted">
                                            {formatRelative(item.created_at)}
                                        </time>
                                    </div>
                                    <h3 className="mt-2 break-words text-sm font-semibold text-foreground">{item.title}</h3>
                                    <p className="mt-1 whitespace-pre-line break-words text-sm leading-relaxed text-muted">{item.content}</p>
                                </div>
                            </li>
                        );
                    })}
                </ol>
            ) : (
                !news.error && (
                    <EmptyState
                        className="py-8"
                        icon={<Megaphone size={18} aria-hidden />}
                        title="All quiet"
                        description={
                            isAdmin ? (
                                <>
                                    Post an update from{' '}
                                    <Link to="/admin/news" className="font-medium text-primary-text hover:underline">
                                        Announcements
                                    </Link>{' '}
                                    to tell everyone about changes or maintenance.
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
        <div className="space-y-6 sm:space-y-8">
            <Hero name={user?.username} overview={overview.data} isAdmin={isAdmin} />

            <OverviewTiles overview={overview} servers={servers} />

            <div className="grid grid-cols-1 items-start gap-6 xl:grid-cols-3">
                <ServersSection servers={servers} isAdmin={isAdmin} />
                <NewsFeed news={news} isAdmin={isAdmin} />
            </div>
        </div>
    );
}
