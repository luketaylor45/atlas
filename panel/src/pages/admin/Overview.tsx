import { useState, useEffect } from 'react';
import api from '../../lib/api';
import { Users, Layers, Activity, HardDrive, ShieldCheck, AlertCircle, AlertTriangle } from 'lucide-react';
import clsx from 'clsx';
import { formatDistanceToNow } from 'date-fns';

interface Stats {
    users: number;
    services: number;
    nodes_total: number;
    nodes_healthy: number;
    total_memory: number;
    total_disk: number;
    total_cpu: number;
    nodes: any[];
    logs: any[];
    system_health: string;
}

const StatCard = ({ icon: Icon, label, value, detail, color, warning }: { icon: any, label: string, value: string | number, detail: string, color: string, warning?: boolean }) => (
    <div className={clsx("panel-card group relative overflow-hidden", warning && "border-amber-500/30")}>
        {warning && <div className="absolute top-0 right-0 p-2 bg-amber-500/10 text-amber-500 rounded-bl-xl"><AlertCircle size={14} /></div>}
        <div className="flex items-start justify-between">
            <div>
                <p className="text-[11px] font-bold uppercase tracking-wider text-muted mb-2">{label}</p>
                <h3 className="text-3xl font-bold tracking-tight mb-1">{value}</h3>
                <p className="text-xs text-muted font-medium">{detail}</p>
            </div>
            <div className={clsx("p-3 rounded-xl bg-secondary transition-colors", color)}>
                <Icon size={20} className={clsx("text-white")} />
            </div>
        </div>
    </div>
);

export default function AdminOverviewPage() {
    const [stats, setStats] = useState<Stats | null>(null);
    const [loading, setLoading] = useState(true);

    useEffect(() => {
        const fetchStats = async () => {
            try {
                const res = await api.get('/admin/overview');
                setStats(res.data);
            } catch (err) {
                console.error("Failed to fetch statistics", err);
            } finally {
                setLoading(false);
            }
        };
        fetchStats();
    }, []);

    if (loading) return <div className="p-12 text-center animate-pulse text-muted font-bold tracking-widest mt-20 uppercase">Loading Statistics...</div>;

    const allNodesHealthy = stats?.nodes_total === stats?.nodes_healthy;

    return (
        <div className="space-y-10 animation-enter">
            <div className="flex flex-col md:flex-row md:items-center justify-between gap-6">
                <div>
                    <h1 className="text-3xl font-bold tracking-tight mb-2 text-glow">Platform Pulse</h1>
                    <p className="text-muted text-sm font-medium italic">Omnipresent view of your compute distribution and user activity.</p>
                </div>
                {!allNodesHealthy ? (
                    <div className="bg-amber-500/10 border border-amber-500/20 px-5 py-2.5 rounded-xl flex items-center gap-3 animate-pulse">
                        <AlertTriangle className="text-amber-500" size={18} />
                        <span className="text-xs font-black text-amber-500 uppercase tracking-widest">{stats?.nodes_total! - stats?.nodes_healthy!} NODE(S) OFFLINE</span>
                    </div>
                ) : (
                    <div className="bg-emerald-500/10 border border-emerald-500/20 px-5 py-2.5 rounded-xl flex items-center gap-3 shadow-[0_0_20px_-5px_rgba(16,185,129,0.2)]">
                        <ShieldCheck className="text-emerald-500" size={18} />
                        <span className="text-xs font-black text-emerald-500 uppercase tracking-widest">All Systems Operational</span>
                    </div>
                )}
            </div>

            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">
                <StatCard icon={Users} label="Total Users" value={stats?.users || 0} detail="Active Identities" color="bg-indigo-600 shadow-indigo-600/20" />
                <StatCard icon={Layers} label="Active Services" value={stats?.services || 0} detail="Workload Instances" color="bg-rose-600 shadow-rose-600/20" />
                <StatCard icon={Activity} label="System Capacity" value={`${stats?.nodes_healthy}/${stats?.nodes_total}`} detail="Healthy Nodes" color="bg-cyan-600 shadow-cyan-600/20" warning={!allNodesHealthy} />
                <StatCard icon={HardDrive} label="Aggregated RAM" value={`${((stats?.total_memory || 0) / 1024).toFixed(1)} GB`} detail="Cluster Allocation" color="bg-orange-600 shadow-orange-600/20" />
            </div>

            <div className="grid grid-cols-1 lg:grid-cols-3 gap-8">
                <div className="lg:col-span-2 space-y-6">
                    <div className="panel-card h-full p-8 flex flex-col justify-between overflow-hidden relative group">
                        {/* Fake visualization grid */}
                        <div className="absolute inset-x-0 bottom-0 h-48 bg-gradient-to-t from-primary/5 to-transparent pointer-none" />
                        <div className="flex items-center justify-between mb-10 relative">
                            <div>
                                <h3 className="text-xl font-black tracking-tight mb-1">Global Load Distribution</h3>
                                <p className="text-xs text-muted font-medium uppercase tracking-widest">Real-time resource utilization across nodes</p>
                            </div>
                            <div className="flex items-center gap-2">
                                <span className="w-2 h-2 rounded-full bg-primary animate-ping" />
                                <span className="text-[10px] font-black uppercase tracking-widest text-primary">Live Pulse</span>
                            </div>
                        </div>

                        <div className="grid grid-cols-2 md:grid-cols-4 lg:grid-cols-6 items-end gap-3 min-h-[160px] relative px-4">
                            {stats?.nodes?.map((node, i) => {
                                const load = node.is_online ? (node.used_ram && node.total_ram ? (node.used_ram / node.total_ram) * 100 : 0) : 0;
                                return (
                                    <div key={i} className="relative group/bar flex-1 h-full flex flex-col justify-end">
                                        <div className="absolute bottom-full left-1/2 -translate-x-1/2 mb-2 bg-zinc-900 text-[10px] px-2 py-1 rounded opacity-0 group-hover/bar:opacity-100 transition-opacity whitespace-nowrap font-bold border border-border z-10">
                                            {node.name}: {load.toFixed(1)}% Load
                                        </div>
                                        <div
                                            className={clsx(
                                                "w-full rounded-t-lg transition-all duration-700",
                                                node.is_online ? "bg-primary/20 group-hover/bar:bg-primary/40" : "bg-red-500/10"
                                            )}
                                            style={{ height: `${Math.max(load, node.is_online ? 5 : 2)}%` }}
                                        >
                                            <div className={clsx("w-full h-1 rounded-t-lg", node.is_online ? "bg-primary" : "bg-red-500")} />
                                        </div>
                                        <div className="text-[8px] text-center mt-2 font-bold truncate text-muted group-hover/bar:text-primary transition-colors">{node.name}</div>
                                    </div>
                                );
                            })}
                        </div>

                        <div className="mt-10 pt-8 border-t border-border/50 flex justify-between items-center relative">
                            <div className="flex items-center gap-6">
                                <div className="flex items-center gap-2">
                                    <div className="w-2 h-2 rounded-full bg-primary" />
                                    <span className="text-[10px] font-black text-muted uppercase">Allocated</span>
                                </div>
                                <div className="flex items-center gap-2">
                                    <div className="w-2 h-2 rounded-full bg-zinc-700" />
                                    <span className="text-[10px] font-black text-muted uppercase">Physical Capacity</span>
                                </div>
                            </div>
                        </div>
                    </div>
                </div>

                <div className="space-y-6">
                    <div className="panel-card p-6 bg-secondary/10">
                        <h3 className="text-sm font-black uppercase tracking-[0.2em] mb-8 flex items-center gap-3 text-primary">
                            <AlertCircle size={18} />
                            System Event Log
                        </h3>
                        <div className="space-y-5">
                            {stats?.logs?.length === 0 ? (
                                <p className="text-[10px] text-muted text-center py-8 font-bold border-dashed border border-border/40 rounded-xl">No Recent Events</p>
                            ) : (
                                stats?.logs?.map((log, i) => (
                                    <div key={i} className="flex gap-4 group">
                                        <div className={clsx(
                                            "w-1 rounded-full transition-colors",
                                            log.action === 'create' ? "bg-emerald-500" : log.action === 'delete' ? "bg-red-500" : "bg-blue-500"
                                        )} />
                                        <div>
                                            <p className="text-xs font-black tracking-tight group-hover:text-primary transition-colors">{log.description}</p>
                                            <p className="text-[10px] text-zinc-500 mt-1 font-bold uppercase tracking-widest font-mono italic">
                                                {formatDistanceToNow(new Date(log.created_at), { addSuffix: true })}
                                            </p>
                                        </div>
                                    </div>
                                ))
                            )}
                        </div>
                        <button className="w-full mt-8 py-3 rounded-xl border border-border/50 text-[10px] font-black uppercase tracking-widest hover:bg-primary hover:text-white transition-all">View All Activity</button>
                    </div>
                </div>
            </div>
        </div>
    );
}
