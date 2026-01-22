import { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import api from '../../lib/api';
import { Layers, Activity, Globe, HardDrive, Cpu, ExternalLink } from 'lucide-react';
import clsx from 'clsx';

export default function MyServices() {
    const navigate = useNavigate();
    const [services, setServices] = useState<any[]>([]);
    const [loading, setLoading] = useState(true);

    useEffect(() => {
        const fetchServices = async () => {
            try {
                const res = await api.get('/services');
                setServices(res.data);
            } catch (err) {
                console.error("Failed to fetch user services", err);
            } finally {
                setLoading(false);
            }
        };
        fetchServices();
        const interval = setInterval(fetchServices, 10000);
        return () => clearInterval(interval);
    }, []);

    return (
        <div className="space-y-8 max-w-7xl mx-auto animation-enter">
            <div className="flex justify-between items-end">
                <div>
                    <h1 className="text-3xl font-bold tracking-tight mb-2">My Services</h1>
                    <p className="text-muted">Monitoring {services.length} active service instances across the Atlas cloud.</p>
                </div>
            </div>

            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-8 pb-10">
                {loading ? (
                    <div className="col-span-full py-20 text-center text-muted animate-pulse font-bold tracking-widest uppercase">SYNCHRONIZING CLUSTER...</div>
                ) : services.length === 0 ? (
                    <div className="col-span-full py-32 text-center panel-card bg-secondary/10 border-dashed">
                        <Layers size={64} className="mx-auto text-muted mb-6 opacity-20" />
                        <h3 className="text-xl font-bold mb-2">No Active Services</h3>
                        <p className="text-muted text-sm max-w-xs mx-auto">Contact your administrator or use the provisioning tool to deploy your first service.</p>
                    </div>
                ) : services.map(service => (
                    <div
                        key={service.id}
                        onClick={() => navigate(`/services/${service.uuid}`)}
                        className="panel-card panel-card-hover group relative overflow-hidden flex flex-col h-full"
                    >
                        {/* Background Gradient Detail */}
                        <div className="absolute -right-10 -top-10 w-40 h-40 bg-primary/5 rounded-full blur-3xl group-hover:bg-primary/10 transition-all duration-500" />

                        <div className="flex items-start justify-between mb-8">
                            <div className="icon-box group-hover:bg-primary group-hover:text-white group-hover:scale-110 duration-300">
                                <Layers size={24} />
                            </div>
                            <div className={clsx(
                                "flex items-center gap-2 px-3 py-1 rounded-full text-[10px] font-black uppercase tracking-[0.15em] border",
                                service.status === 'installing' ? "bg-primary/10 text-primary border-primary/20 animate-pulse" :
                                    service.status === 'running' ? "bg-emerald-500/10 text-emerald-500 border-emerald-500/20" :
                                        "bg-zinc-500/10 text-zinc-400 border-zinc-500/20"
                            )}>
                                <span className={clsx("w-1.5 h-1.5 rounded-full",
                                    service.status === 'installing' ? "bg-primary" :
                                        service.status === 'running' ? "bg-emerald-500" : "bg-zinc-500"
                                )} />
                                {service.status}
                            </div>
                        </div>

                        <div className="flex-1 space-y-6">
                            <div>
                                <h3 className="text-2xl font-black group-hover:text-primary transition-colors tracking-tight line-clamp-1">{service.name}</h3>
                                <div className="flex items-center gap-2 mt-2">
                                    <Globe size={12} className="text-muted" />
                                    <p className="text-xs text-muted font-bold font-mono tracking-tight lowercase">
                                        {service.node?.address}:{service.port}
                                    </p>
                                </div>
                            </div>

                            <div className="grid grid-cols-2 gap-4 pt-6 border-t border-border/50">
                                <div className="space-y-1">
                                    <div className="text-[10px] font-black text-muted uppercase tracking-widest pl-0.5">Allocation</div>
                                    <div className="flex items-center gap-2">
                                        <HardDrive size={14} className="text-primary/70" />
                                        <span className="text-sm font-bold">{service.memory}<span className="text-[10px] text-muted ml-0.5">MB</span></span>
                                    </div>
                                </div>
                                <div className="space-y-1 text-right">
                                    <div className="text-[10px] font-black text-muted uppercase tracking-widest pr-0.5">Service Type</div>
                                    <div className="flex items-center gap-2 justify-end">
                                        <Cpu size={14} className="text-primary/70" />
                                        <span className="text-sm font-bold truncate max-w-[120px]">{service.egg?.name}</span>
                                    </div>
                                </div>
                            </div>
                        </div>

                        <div className="mt-6 pt-4 border-t border-border/30 flex items-center justify-between text-[10px] font-black text-muted/60 uppercase tracking-widest">
                            <span className="flex items-center gap-1"><Activity size={10} /> Node: {service.node?.name || 'Local'}</span>
                            <span className="flex items-center gap-1 group-hover:text-primary transition-colors">Manage <ExternalLink size={10} /></span>
                        </div>
                    </div>
                ))}
            </div>
        </div>
    );
}
