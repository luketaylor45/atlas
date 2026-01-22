import { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import api from '../../lib/api';
import {
    Layers, ShieldCheck,
    ArrowRight, Zap, Bell, Clock,
    CheckCircle
} from 'lucide-react';
import { useAuth } from '../../context/AuthContext';

export default function UserDashboard() {
    const { user } = useAuth();
    const navigate = useNavigate();
    const [services, setServices] = useState<any[]>([]);
    const [overview, setOverview] = useState<any>({ total_services: 0, running_services: 0, health: 'OPTIMAL' });
    const [loading, setLoading] = useState(true);

    useEffect(() => {
        const fetchData = async () => {
            try {
                const [sRes, oRes] = await Promise.all([
                    api.get('/services'),
                    api.get('/services/overview')
                ]);
                setServices(sRes.data);
                setOverview(oRes.data);
            } catch (err) {
                console.error("Failed to fetch dashboard data", err);
            } finally {
                setLoading(false);
            }
        };
        fetchData();
    }, []);

    return (
        <div className="space-y-10 animate-in">
            {/* Welcome Header */}
            <div className="flex flex-col md:flex-row md:items-center justify-between gap-6">
                <div>
                    <h1 className="text-4xl font-bold tracking-tight mb-2">Welcome back, {user?.username || user?.email.split('@')[0]}</h1>
                    <p className="text-muted text-lg">Here's an overview of your services and system status.</p>
                </div>
                <div className="bg-emerald-500/10 border border-emerald-500/20 px-5 py-2.5 rounded-2xl flex items-center gap-3">
                    <CheckCircle className="text-emerald-500" size={20} />
                    <span className="text-sm font-bold text-emerald-500 uppercase tracking-wider">System Optimal</span>
                </div>
            </div>

            {/* Quick Stats */}
            <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
                <div className="panel-card bg-card border-border shadow-sm p-6">
                    <div className="flex items-center gap-4">
                        <div className="p-3 rounded-xl bg-primary/10 text-primary">
                            <Layers size={24} />
                        </div>
                        <div>
                            <p className="text-xs font-bold text-muted uppercase tracking-widest mb-1">Total Services</p>
                            <h3 className="text-3xl font-bold tracking-tight">{overview.total_services}</h3>
                        </div>
                    </div>
                </div>

                <div className="panel-card bg-card border-border shadow-sm p-6">
                    <div className="flex items-center gap-4">
                        <div className="p-3 rounded-xl bg-emerald-500/10 text-emerald-500">
                            <Zap size={24} />
                        </div>
                        <div>
                            <p className="text-xs font-bold text-muted uppercase tracking-widest mb-1">Active Services</p>
                            <h3 className="text-3xl font-bold tracking-tight">{overview.running_services}</h3>
                        </div>
                    </div>
                </div>

                <div className="panel-card bg-card border-border shadow-sm p-6">
                    <div className="flex items-center gap-4">
                        <div className="p-3 rounded-xl bg-blue-500/10 text-blue-500">
                            <ShieldCheck size={24} />
                        </div>
                        <div>
                            <p className="text-xs font-bold text-muted uppercase tracking-widest mb-1">Node Status</p>
                            <h3 className="text-3xl font-bold tracking-tight">Stable</h3>
                        </div>
                    </div>
                </div>
            </div>

            <div className="grid grid-cols-1 lg:grid-cols-3 gap-8">
                {/* Recent Services */}
                <div className="lg:col-span-2 space-y-6">
                    <div className="flex items-center justify-between">
                        <h2 className="text-xl font-bold flex items-center gap-2">
                            <Clock size={20} className="text-primary" />
                            Recent Activity
                        </h2>
                        <button
                            onClick={() => navigate('/services')}
                            className="text-sm font-bold text-primary hover:underline flex items-center gap-1 uppercase tracking-wider"
                        >
                            View All <ArrowRight size={14} />
                        </button>
                    </div>

                    <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                        {loading ? (
                            Array(4).fill(0).map((_, i) => (
                                <div key={i} className="h-32 bg-secondary/50 rounded-2xl animate-pulse border border-border/40" />
                            ))
                        ) : services.length === 0 ? (
                            <div className="col-span-2 panel-card bg-secondary/10 border-dashed border-border/60 py-16 text-center">
                                <p className="text-muted font-bold mb-6">No services found in your account.</p>
                                <button
                                    onClick={() => navigate('/services')}
                                    className="px-8 py-3 bg-primary text-white rounded-xl font-bold shadow-lg shadow-primary/20 hover:scale-105 transition-transform"
                                >
                                    Deploy First Service
                                </button>
                            </div>
                        ) : (
                            services.slice(0, 4).map(service => (
                                <div
                                    key={service.uuid}
                                    onClick={() => navigate(`/services/${service.uuid}`)}
                                    className="panel-card bg-card border-border hover:border-primary/40 transition-all cursor-pointer group p-6"
                                >
                                    <div className="flex items-center justify-between mb-4">
                                        <div className="p-2.5 rounded-lg bg-secondary text-primary group-hover:bg-primary group-hover:text-white transition-all shadow-inner">
                                            <Layers size={20} />
                                        </div>
                                        <div className={`flex items-center gap-1.5 px-2 py-0.5 rounded-full text-[9px] font-bold uppercase tracking-wider border ${service.status === 'running' ? 'bg-emerald-500/10 text-emerald-500 border-emerald-500/20' : 'bg-muted/10 text-muted border-muted/20'}`}>
                                            <div className={`w-1 h-1 rounded-full ${service.status === 'running' ? 'bg-emerald-500' : 'bg-muted'}`} />
                                            {service.status}
                                        </div>
                                    </div>
                                    <h4 className="font-bold text-lg truncate group-hover:text-primary transition-colors">{service.name}</h4>
                                    <p className="text-[10px] text-muted font-bold uppercase tracking-widest mt-1.5">{service.egg?.name}</p>
                                </div>
                            ))
                        )}
                    </div>
                </div>

                {/* System Announcements */}
                <div className="space-y-6">
                    <h2 className="text-xl font-bold flex items-center gap-2">
                        <Bell size={20} className="text-primary" />
                        Latest News
                    </h2>
                    <div className="panel-card p-6 space-y-6 border-border/60 bg-secondary/10">
                        <div className="space-y-3">
                            <span className="text-[9px] font-bold text-primary uppercase tracking-widest bg-primary/10 px-2 py-0.5 rounded border border-primary/20">Release</span>
                            <h4 className="text-sm font-bold leading-tight">Atlas Node v2.1 Performance Update</h4>
                            <p className="text-xs text-muted leading-relaxed font-medium">Infrastructure upgrades are complete for all major regions, improving deployment speed by 40%.</p>
                        </div>
                        <div className="h-px bg-border/40" />
                        <div className="space-y-3">
                            <span className="text-[9px] font-bold text-blue-500 uppercase tracking-widest bg-blue-500/10 px-2 py-0.5 rounded border border-blue-500/20">Status</span>
                            <h4 className="text-sm font-bold leading-tight">Scheduled Optimization</h4>
                            <p className="text-xs text-muted leading-relaxed font-medium">We will be performing a global database optimization on Feb 1st. No downtime is expected.</p>
                        </div>
                    </div>
                </div>
            </div>
        </div>
    );
}
