import { useState, useEffect } from 'react';
import api from '../../lib/api';
import { Users, Layers, Activity, HardDrive, ShieldCheck, AlertCircle } from 'lucide-react';
import clsx from 'clsx';

interface Stats {
    users: number;
    services: number;
    nodes: number;
    total_memory: number;
    total_disk: number;
}

const StatCard = ({ icon: Icon, label, value, detail, color }: { icon: any, label: string, value: string | number, detail: string, color: string }) => (
    <div className="panel-card group relative">
        <div className="flex items-start justify-between">
            <div>
                <p className="text-[11px] font-bold uppercase tracking-wider text-muted mb-2">{label}</p>
                <h3 className="text-3xl font-bold tracking-tight mb-1">{value}</h3>
                <p className="text-xs text-muted font-medium">{detail}</p>
            </div>
            <div className={clsx("p-3 rounded-xl bg-secondary transition-colors", color.replace('bg-', 'text-'))}>
                <Icon size={20} />
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

    return (
        <div className="space-y-10 animation-enter">
            <div className="flex flex-col md:flex-row md:items-center justify-between gap-6">
                <div>
                    <h1 className="text-3xl font-bold tracking-tight mb-2">System Overview</h1>
                    <p className="text-muted text-sm font-medium">Global infrastructure statistics and system status.</p>
                </div>
                <div className="bg-emerald-500/10 border border-emerald-500/20 px-5 py-2.5 rounded-xl flex items-center gap-3">
                    <ShieldCheck className="text-emerald-500" size={18} />
                    <span className="text-xs font-bold text-emerald-500 uppercase tracking-wider">All Systems Operational</span>
                </div>
            </div>

            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">
                <StatCard icon={Users} label="Total Users" value={stats?.users || 0} detail="Registered accounts" color="bg-primary" />
                <StatCard icon={Layers} label="Active Services" value={stats?.services || 0} detail="Running instances" color="bg-purple-500" />
                <StatCard icon={Activity} label="Managed Nodes" value={stats?.nodes || 0} detail="Compute resources" color="bg-blue-500" />
                <StatCard icon={HardDrive} label="Total Allocation" value={`${((stats?.total_memory || 0) / 1024).toFixed(1)} GB`} detail="Memory consumed" color="bg-amber-500" />
            </div>

            <div className="grid grid-cols-1 lg:grid-cols-3 gap-8">
                <div className="lg:col-span-2 panel-card p-10 flex flex-col justify-center items-center text-center">
                    <div className="p-4 rounded-full bg-secondary mb-6">
                        <Activity size={48} className="text-primary" />
                    </div>
                    <h3 className="text-xl font-bold mb-2">Usage Visualization</h3>
                    <p className="text-muted text-sm max-w-sm">Detailed resource usage charts and network traffic analysis tools are coming soon to provide deeper insights into your infrastructure.</p>
                </div>

                <div className="panel-card p-6">
                    <h3 className="text-sm font-bold uppercase tracking-widest mb-6 flex items-center gap-2">
                        <AlertCircle size={16} className="text-primary" />
                        Recent System Events
                    </h3>
                    <div className="space-y-4">
                        {[1, 2, 3].map(i => (
                            <div key={i} className="flex gap-4 p-4 rounded-xl bg-secondary/50 border border-border/50 group hover:border-primary/30 transition-all">
                                <div>
                                    <p className="text-sm font-bold">New Service Created</p>
                                    <p className="text-xs text-muted mt-1 font-medium">Node {i} successfully initialized a new service instance.</p>
                                    <p className="text-[10px] text-primary mt-2 font-bold uppercase tracking-wider">{i * 5} minutes ago</p>
                                </div>
                            </div>
                        ))}
                    </div>
                </div>
            </div>
        </div>
    );
}
