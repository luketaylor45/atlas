import { useState, useEffect } from 'react';
import { useParams } from 'react-router-dom';
import api from '../../lib/api';
import { Activity, Download, User, Clock, MapPin, Monitor, Zap, FileText, Settings, Key, Users as UsersIcon } from 'lucide-react';
import clsx from 'clsx';

interface ActivityLog {
    id: number;
    service_id: number;
    user_id: number;
    action: string;
    resource: string;
    description: string;
    ip_address: string;
    user_agent: string;
    metadata: string;
    created_at: string;
    user?: {
        username: string;
    };
}

export default function ServiceActivityTab() {
    const { uuid } = useParams();
    const [logs, setLogs] = useState<ActivityLog[]>([]);
    const [loading, setLoading] = useState(true);
    const [filter, setFilter] = useState('all');

    useEffect(() => {
        fetchActivityLogs();
        // Refresh every 10 seconds
        const interval = setInterval(fetchActivityLogs, 10000);
        return () => clearInterval(interval);
    }, [uuid]);

    const fetchActivityLogs = async () => {
        try {
            const res = await api.get(`/services/${uuid}/logs`);
            setLogs(res.data.logs || []);
        } catch (err) {
            console.error('Failed to fetch activity logs', err);
            setLogs([]);
        } finally {
            setLoading(false);
        }
    };

    const actionIcons: Record<string, any> = {
        power: Zap,
        sftp_login: Key,
        file_edit: FileText,
        file_upload: FileText,
        file_delete: FileText,
        startup_change: Settings,
        user_added: UsersIcon,
        user_removed: UsersIcon,
        user_updated: UsersIcon,
    };

    const actionColors: Record<string, string> = {
        power: 'text-blue-500 bg-blue-500/10',
        sftp_login: 'text-purple-500 bg-purple-500/10',
        file_edit: 'text-amber-500 bg-amber-500/10',
        file_upload: 'text-emerald-500 bg-emerald-500/10',
        file_delete: 'text-red-500 bg-red-500/10',
        startup_change: 'text-primary bg-primary/10',
        user_added: 'text-green-500 bg-green-500/10',
        user_removed: 'text-red-500 bg-red-500/10',
        user_updated: 'text-blue-500 bg-blue-500/10',
    };

    const getActionIcon = (action: string) => {
        return actionIcons[action] || Activity;
    };

    const getActionColor = (action: string) => {
        return actionColors[action] || 'text-muted bg-secondary';
    };

    const filteredLogs = filter === 'all' ? logs : logs.filter(log => log.action === filter);

    const exportLogs = () => {
        const csv = [
            ['Time', 'User', 'Action', 'Resource', 'Description', 'IP Address'],
            ...filteredLogs.map(log => [
                new Date(log.created_at).toLocaleString(),
                log.user?.username || `User #${log.user_id}`,
                log.action,
                log.resource || '-',
                log.description,
                log.ip_address
            ])
        ].map(row => row.join(',')).join('\n');

        const blob = new Blob([csv], { type: 'text/csv' });
        const url = window.URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.href = url;
        a.download = `activity-log-${uuid}-${Date.now()}.csv`;
        a.click();
    };

    if (loading) return <div className="p-12 text-center text-muted animate-pulse">Loading activity logs...</div>;

    return (
        <div className="space-y-8">
            {/* Header */}
            <div className="flex flex-col md:flex-row md:items-center justify-between gap-6">
                <div>
                    <h3 className="text-2xl font-bold tracking-tight flex items-center gap-3">
                        <Activity className="text-primary" size={28} /> Activity Log
                    </h3>
                    <p className="text-muted text-sm mt-1">Complete audit trail of all service actions</p>
                </div>
                <div className="flex items-center gap-3">
                    <select
                        value={filter}
                        onChange={(e) => setFilter(e.target.value)}
                        className="input-field w-auto"
                    >
                        <option value="all">All Actions</option>
                        <option value="power">Power Actions</option>
                        <option value="sftp_login">SFTP Logins</option>
                        <option value="file_edit">File Changes</option>
                        <option value="startup_change">Startup Changes</option>
                        <option value="user_added">User Management</option>
                    </select>
                    <button
                        onClick={exportLogs}
                        disabled={filteredLogs.length === 0}
                        className="btn-primary flex items-center gap-2 disabled:opacity-50"
                    >
                        <Download size={18} /> Export CSV
                    </button>
                </div>
            </div>

            {/* Timeline */}
            {filteredLogs.length === 0 ? (
                <div className="panel-card p-12 text-center border-dashed border-2">
                    <Activity size={48} className="mx-auto text-muted/30 mb-4" />
                    <h4 className="font-bold text-lg mb-2">No Activity Yet</h4>
                    <p className="text-sm text-muted">Actions performed on this service will appear here</p>
                    <p className="text-xs text-muted mt-4">Tracks: Power actions, SFTP logins, file operations, and more</p>
                </div>
            ) : (
                <div className="space-y-4">
                    {filteredLogs.map((log) => {
                        const Icon = getActionIcon(log.action);
                        const colorClass = getActionColor(log.action);
                        return (
                            <div key={log.id} className="relative">
                                <div className="panel-card p-6 hover:border-primary/30 transition-all">
                                    <div className="flex gap-6">
                                        {/* Icon */}
                                        <div className={clsx(
                                            "w-14 h-14 rounded-2xl flex items-center justify-center shrink-0",
                                            colorClass
                                        )}>
                                            <Icon size={24} />
                                        </div>

                                        {/* Content */}
                                        <div className="flex-1 space-y-3">
                                            <div className="flex items-start justify-between gap-4">
                                                <div>
                                                    <h4 className="font-bold text-lg">{log.description}</h4>
                                                    {log.resource && (
                                                        <p className="text-sm text-muted font-mono mt-1">{log.resource}</p>
                                                    )}
                                                </div>
                                                <div className="text-right shrink-0">
                                                    <div className="text-xs font-bold text-primary uppercase tracking-wider">
                                                        {log.action.replace('_', ' ')}
                                                    </div>
                                                </div>
                                            </div>

                                            <div className="grid grid-cols-2 md:grid-cols-4 gap-4 pt-3 border-t border-border/50">
                                                <div className="flex items-center gap-2 text-xs">
                                                    <User size={14} className="text-muted" />
                                                    <span className="font-medium">{log.user?.username || `User #${log.user_id}`}</span>
                                                </div>
                                                <div className="flex items-center gap-2 text-xs text-muted">
                                                    <Clock size={14} />
                                                    <span>{new Date(log.created_at).toLocaleString()}</span>
                                                </div>
                                                <div className="flex items-center gap-2 text-xs text-muted">
                                                    <MapPin size={14} />
                                                    <span className="font-mono">
                                                        {log.ip_address === '::1' || log.ip_address === '127.0.0.1' ? 'Cluster Internal' : log.ip_address}
                                                    </span>
                                                </div>
                                                <div className="flex items-center gap-2 text-xs text-muted">
                                                    <Monitor size={14} />
                                                    <span className="truncate" title={log.user_agent}>
                                                        {log.user_agent.substring(0, 20)}...
                                                    </span>
                                                </div>
                                            </div>

                                            {log.metadata && (
                                                <details className="text-xs">
                                                    <summary className="cursor-pointer text-muted hover:text-foreground font-medium">
                                                        View Metadata
                                                    </summary>
                                                    <pre className="mt-2 p-3 bg-black/30 rounded-lg font-mono text-[10px] overflow-x-auto">
                                                        {JSON.stringify(JSON.parse(log.metadata), null, 2)}
                                                    </pre>
                                                </details>
                                            )}
                                        </div>
                                    </div>
                                </div>
                            </div>
                        );
                    })}
                </div>
            )}

            {/* Stats Summary */}
            <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
                {['power', 'sftp_login', 'file_edit', 'startup_change'].map(action => {
                    const count = logs.filter(log => log.action === action).length;
                    const Icon = getActionIcon(action);
                    const colorClass = getActionColor(action);

                    return (
                        <div key={action} className="panel-card p-4 border-border/50">
                            <div className="flex items-center gap-3 mb-2">
                                <div className={clsx("p-2 rounded-lg", colorClass)}>
                                    <Icon size={16} />
                                </div>
                                <div className="text-2xl font-bold">{count}</div>
                            </div>
                            <div className="text-xs text-muted uppercase tracking-wider font-bold">
                                {action.replace('_', ' ')}
                            </div>
                        </div>
                    );
                })}
            </div>
        </div>
    );
}
