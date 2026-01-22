import { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import api from '../../lib/api';
import { Layers, Plus, ExternalLink, HardDrive, Cpu, Trash2, Search, Globe, Edit, X, Activity, AlertCircle } from 'lucide-react';
import clsx from 'clsx';

export default function AdminServicesPage() {
    const navigate = useNavigate();
    const [services, setServices] = useState<any[]>([]);
    const [loading, setLoading] = useState(true);
    const [searchTerm, setSearchTerm] = useState('');
    const [editingService, setEditingService] = useState<any>(null);

    const fetchServices = async () => {
        try {
            const res = await api.get('/admin/services');
            setServices(res.data);
        } catch (err) {
            console.error("Failed to fetch services", err);
        } finally {
            setLoading(false);
        }
    };

    useEffect(() => {
        fetchServices();
    }, []);

    const filteredServices = services.filter(service =>
        service.name.toLowerCase().includes(searchTerm.toLowerCase()) ||
        service.uuid.toLowerCase().includes(searchTerm.toLowerCase()) ||
        service.user?.email.toLowerCase().includes(searchTerm.toLowerCase()) ||
        service.user?.username?.toLowerCase().includes(searchTerm.toLowerCase())
    );

    const deleteService = async (id: number, name: string) => {
        if (!confirm(`Are you sure you want to delete "${name}"? This will permanently remove all files and data belonging to this service.`)) return;
        try {
            await api.delete(`/admin/services/${id}`);
            fetchServices();
        } catch (err) {
            alert("Failed to delete service. Please check node connectivity.");
        }
    };

    const handleEdit = (service: any) => {
        setEditingService({ ...service });
    };

    const saveEdit = async () => {
        if (!editingService) return;
        try {
            await api.put(`/admin/services/${editingService.id}`, editingService);
            setEditingService(null);
            fetchServices();
        } catch (err) {
            alert("Failed to update service.");
        }
    };

    if (loading) return <div className="p-12 text-center animate-pulse text-muted font-bold tracking-widest mt-20 uppercase">Loading Services...</div>;

    return (
        <div className="space-y-8 animation-enter">
            <div className="flex flex-col md:flex-row md:items-center justify-between gap-6 bg-secondary/20 p-8 rounded-3xl border border-border/50">
                <div>
                    <h1 className="text-3xl font-bold tracking-tight flex items-center gap-3">
                        <Layers className="text-primary" size={28} /> Manage Services
                    </h1>
                    <p className="text-muted font-medium text-sm mt-1">Found {services.length} total service deployments in the system.</p>
                </div>

                <div className="flex items-center gap-4">
                    <div className="relative group">
                        <Search className="absolute left-4 top-1/2 -translate-y-1/2 text-muted group-focus-within:text-primary transition-colors" size={18} />
                        <input
                            type="text"
                            placeholder="Search by name, ID, or user..."
                            value={searchTerm}
                            onChange={(e) => setSearchTerm(e.target.value)}
                            className="bg-background border border-border/50 pl-11 pr-5 py-2.5 rounded-xl text-sm focus:outline-none focus:ring-2 focus:ring-primary/20 focus:border-primary transition-all w-80 font-medium"
                        />
                    </div>
                    <button
                        onClick={() => navigate('/admin/services/create')}
                        className="flex items-center gap-2 bg-primary text-white px-6 py-2.5 rounded-xl font-bold text-sm hover:bg-primary/90 transition-all shadow-lg shadow-primary/20"
                    >
                        <Plus size={18} />
                        Create New
                    </button>
                </div>
            </div>

            <div className="panel-card !p-0 border-border/40 shadow-sm overflow-hidden">
                <div className="overflow-x-auto">
                    <table className="w-full text-left border-collapse">
                        <thead>
                            <tr className="bg-secondary/40 border-b border-border/50">
                                <th className="px-6 py-4 text-[11px] font-bold uppercase tracking-wider text-muted">Service & Detail</th>
                                <th className="px-6 py-4 text-[11px] font-bold uppercase tracking-wider text-muted">Owner</th>
                                <th className="px-6 py-4 text-[11px] font-bold uppercase tracking-wider text-muted">Node & Address</th>
                                <th className="px-6 py-4 text-[11px] font-bold uppercase tracking-wider text-muted">Resources</th>
                                <th className="px-6 py-4 text-[11px] font-bold uppercase tracking-wider text-muted text-right">Status</th>
                            </tr>
                        </thead>
                        <tbody className="divide-y divide-border/30">
                            {filteredServices.map(service => (
                                <tr key={service.id} className="hover:bg-primary/[0.02] transition-colors group">
                                    <td className="px-6 py-5">
                                        <div className="flex items-center gap-4">
                                            <div className="w-10 h-10 rounded-xl bg-secondary flex items-center justify-center text-muted group-hover:bg-primary group-hover:text-white transition-all shadow-inner">
                                                <Layers size={20} />
                                            </div>
                                            <div>
                                                <div className="font-bold text-sm group-hover:text-primary transition-colors">{service.name}</div>
                                                <div className="text-[10px] text-muted font-bold mt-1 uppercase tracking-widest flex items-center gap-1.5 opacity-60">
                                                    <span>{service.uuid.split('-')[0]}</span>
                                                    <span className="opacity-30">|</span>
                                                    <span>{service.egg?.name || 'Standard'}</span>
                                                </div>
                                            </div>
                                        </div>
                                    </td>
                                    <td className="px-6 py-5">
                                        <div className="flex items-center gap-3">
                                            <div className="w-7 h-7 rounded-lg bg-secondary border border-border flex items-center justify-center text-[10px] font-bold text-muted">
                                                {(service.user?.email || 'U').charAt(0).toUpperCase()}
                                            </div>
                                            <div className="flex flex-col">
                                                <span className="text-xs font-bold">{service.user?.username || 'System'}</span>
                                                <span className="text-[10px] text-muted font-medium">{service.user?.email}</span>
                                            </div>
                                        </div>
                                    </td>
                                    <td className="px-6 py-5">
                                        <div className="flex flex-col gap-1">
                                            <div className="flex items-center gap-1.5 text-xs font-bold">
                                                <Globe size={12} className="text-primary/70" /> {service.node?.name || 'Unknown'}
                                            </div>
                                            <div className="flex items-center gap-1.5 text-[10px] text-muted font-medium tracking-wide">
                                                {service.node?.address}:{service.port}
                                            </div>
                                        </div>
                                    </td>
                                    <td className="px-6 py-5">
                                        <div className="flex flex-col gap-2">
                                            <div className="flex items-center gap-1.5 text-[10px] font-bold text-muted">
                                                <HardDrive size={10} className="text-purple-500" />
                                                <span>{service.memory} MB</span>
                                            </div>
                                            <div className="flex items-center gap-1.5 text-[10px] font-bold text-muted">
                                                <Cpu size={10} className="text-blue-500" />
                                                <span>{service.cpu}% CPU</span>
                                            </div>
                                        </div>
                                    </td>
                                    <td className="px-6 py-5 text-right">
                                        <div className="flex items-center justify-end gap-5">
                                            <div className={clsx(
                                                "flex items-center gap-2 px-3 py-1 rounded-full text-[10px] font-bold uppercase tracking-wider border",
                                                service.is_suspended
                                                    ? "bg-red-500/10 text-red-500 border-red-500/20"
                                                    : "bg-emerald-500/10 text-emerald-500 border-emerald-500/20"
                                            )}>
                                                <div className={clsx("w-1 h-1 rounded-full", service.is_suspended ? "bg-red-500" : "bg-emerald-500")} />
                                                {service.is_suspended ? 'Suspended' : 'Healthy'}
                                            </div>
                                            <div className="flex items-center gap-2 opacity-0 group-hover:opacity-100 transition-all duration-200">
                                                <button
                                                    onClick={() => handleEdit(service)}
                                                    className="p-2.5 bg-background hover:bg-primary hover:text-white rounded-lg border border-border shadow-sm transition-all"
                                                    title="Edit Service"
                                                >
                                                    <Edit size={16} />
                                                </button>
                                                <button
                                                    onClick={() => navigate(`/services/${service.uuid}`)}
                                                    className="p-2.5 bg-background hover:bg-primary hover:text-white rounded-lg border border-border shadow-sm transition-all"
                                                    title="View Console"
                                                >
                                                    <ExternalLink size={16} />
                                                </button>
                                                <button
                                                    onClick={() => deleteService(service.id, service.name)}
                                                    className="p-2.5 bg-background hover:bg-red-500 hover:text-white rounded-lg border border-border shadow-sm transition-all"
                                                    title="Delete"
                                                >
                                                    <Trash2 size={16} />
                                                </button>
                                            </div>
                                        </div>
                                    </td>
                                </tr>
                            ))}
                        </tbody>
                    </table>
                </div>
            </div>

            {/* Edit Modal */}
            {editingService && (
                <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/60 backdrop-blur-sm animate-in fade-in duration-200">
                    <div className="bg-background border border-border/60 rounded-3xl w-full max-w-xl shadow-2xl overflow-hidden scale-in">
                        <div className="p-8 border-b border-border/50 flex items-center justify-between bg-secondary/20">
                            <div className="flex items-center gap-3">
                                <div className="w-10 h-10 rounded-xl bg-primary/10 text-primary flex items-center justify-center">
                                    <Activity size={20} />
                                </div>
                                <div>
                                    <h3 className="font-bold text-xl">Adjust Service</h3>
                                    <p className="text-xs text-muted font-medium">Modify resource limits for {editingService.name}</p>
                                </div>
                            </div>
                            <button onClick={() => setEditingService(null)} className="p-2 hover:bg-secondary rounded-xl transition-colors">
                                <X size={20} className="text-muted" />
                            </button>
                        </div>
                        <div className="p-8 space-y-6">
                            <div className="space-y-2">
                                <label className="text-[10px] font-bold text-muted uppercase tracking-widest pl-1">Display Name</label>
                                <input
                                    className="input-field"
                                    value={editingService.name}
                                    onChange={e => setEditingService({ ...editingService, name: e.target.value })}
                                />
                            </div>

                            <div className="grid grid-cols-3 gap-6">
                                <div className="space-y-2">
                                    <label className="text-[10px] font-bold text-muted uppercase tracking-widest pl-1">Memory (MB)</label>
                                    <input
                                        type="number"
                                        className="input-field"
                                        value={editingService.memory}
                                        onChange={e => setEditingService({ ...editingService, memory: parseInt(e.target.value) })}
                                    />
                                </div>
                                <div className="space-y-2">
                                    <label className="text-[10px] font-bold text-muted uppercase tracking-widest pl-1">CPU Limit (%)</label>
                                    <input
                                        type="number"
                                        className="input-field"
                                        value={editingService.cpu}
                                        onChange={e => setEditingService({ ...editingService, cpu: parseInt(e.target.value) })}
                                    />
                                </div>
                                <div className="space-y-2">
                                    <label className="text-[10px] font-bold text-muted uppercase tracking-widest pl-1">Disk (MB)</label>
                                    <input
                                        type="number"
                                        className="input-field"
                                        value={editingService.disk}
                                        onChange={e => setEditingService({ ...editingService, disk: parseInt(e.target.value) })}
                                    />
                                </div>
                            </div>

                            <div className="space-y-2">
                                <label className="text-[10px] font-bold text-muted uppercase tracking-widest pl-1">Container Image</label>
                                <select
                                    className="input-field"
                                    value={editingService.docker_image}
                                    onChange={e => setEditingService({ ...editingService, docker_image: e.target.value })}
                                >
                                    {(() => {
                                        try {
                                            const images = JSON.parse(editingService.egg?.docker_images || '[]');
                                            if (images.length === 0 && editingService.docker_image) {
                                                return <option value={editingService.docker_image}>{editingService.docker_image}</option>;
                                            }
                                            return images.map((img: string) => (
                                                <option key={img} value={img}>{img}</option>
                                            ));
                                        } catch (e) {
                                            return <option value={editingService.docker_image}>{editingService.docker_image}</option>;
                                        }
                                    })()}
                                </select>
                            </div>

                            <div className="p-4 rounded-xl bg-blue-500/5 border border-blue-500/20 flex gap-3">
                                <AlertCircle className="text-blue-500 shrink-0 mt-0.5" size={16} />
                                <div className="text-[10px] text-blue-500/80 font-medium leading-relaxed">
                                    Changes to limits will take effect after a service restart. Currently running services will not be throttled until rebooted.
                                </div>
                            </div>
                        </div>
                        <div className="p-8 bg-secondary/10 flex items-center justify-end gap-3">
                            <button
                                onClick={() => setEditingService(null)}
                                className="px-6 py-2.5 rounded-xl font-bold text-sm text-muted hover:text-foreground transition-colors"
                            >
                                Cancel
                            </button>
                            <button
                                onClick={saveEdit}
                                className="px-8 py-2.5 bg-primary text-white rounded-xl font-bold text-sm shadow-lg shadow-primary/20 hover:bg-primary/90 transition-all"
                            >
                                Apply Changes
                            </button>
                        </div>
                    </div>
                </div>
            )}
        </div>
    );
}
