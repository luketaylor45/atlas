import { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import api from '../../lib/api';
import { Egg, Package, Trash2, Edit, Plus, FolderPlus, Search, X, Settings, Info, ChevronRight } from 'lucide-react';

export default function AdminEggsPage() {
    const navigate = useNavigate();
    const [eggs, setEggs] = useState<any[]>([]);
    const [nests, setNests] = useState<any[]>([]);
    const [loading, setLoading] = useState(true);
    const [searchTerm, setSearchTerm] = useState('');
    const [editingEgg, setEditingEgg] = useState<any>(null);
    const [editingNest, setEditingNest] = useState<any>(null);
    const [showCreateNestModal, setShowCreateNestModal] = useState(false);
    const [newNest, setNewNest] = useState({ name: '', description: '', parent_id: null as number | null });

    const fetchData = async () => {
        try {
            const [eggsRes, nestsRes] = await Promise.all([
                api.get('/admin/eggs'),
                api.get('/admin/nests')
            ]);
            setEggs(eggsRes.data);
            setNests(nestsRes.data);
        } catch (err) {
            console.error("Failed to fetch data", err);
        } finally {
            setLoading(false);
        }
    };

    useEffect(() => {
        fetchData();
    }, []);

    const deleteEgg = async (id: number) => {
        if (!confirm("Are you sure? Deleting this egg will prevent new services from being created with it.")) return;
        try {
            await api.delete(`/admin/eggs/${id}`);
            fetchData();
        } catch (err) {
            alert("Failed to delete egg.");
        }
    };

    const deleteNest = async (id: number) => {
        if (!confirm("Are you sure? Deleting a nest will delete all eggs inside it!")) return;
        try {
            await api.delete(`/admin/nests/${id}`);
            fetchData();
        } catch (err) {
            alert("Failed to delete nest.");
        }
    };

    const saveEgg = async () => {
        try {
            await api.put(`/admin/eggs/${editingEgg.id}`, editingEgg);
            setEditingEgg(null);
            fetchData();
        } catch (err) {
            alert("Failed to update egg.");
        }
    };

    const saveNest = async () => {
        try {
            await api.put(`/admin/nests/${editingNest.id}`, editingNest);
            setEditingNest(null);
            fetchData();
        } catch (err) {
            alert("Failed to update nest.");
        }
    };

    const createNest = async () => {
        try {
            await api.post('/admin/nests', newNest);
            setShowCreateNestModal(false);
            setNewNest({ name: '', description: '', parent_id: null });
            fetchData();
        } catch (err) {
            alert("Failed to create nest.");
        }
    };

    if (loading) return <div className="p-12 text-center animate-pulse text-muted font-bold tracking-widest mt-20 uppercase">Loading Templates...</div>;

    return (
        <div className="space-y-10 animation-enter">
            <div className="flex flex-col md:flex-row md:items-center justify-between gap-6">
                <div>
                    <h1 className="text-3xl font-bold tracking-tight flex items-center gap-3 text-foreground">
                        <Package className="text-primary" size={28} /> Eggs & Nests
                    </h1>
                    <p className="text-muted font-medium text-sm mt-1">Manage your service templates and categories configuration.</p>
                </div>

                <div className="flex items-center gap-3">
                    <button
                        onClick={() => setShowCreateNestModal(true)}
                        className="flex items-center gap-2 bg-secondary text-foreground px-5 py-2.5 rounded-xl font-bold text-sm hover:bg-secondary/80 transition-all border border-border/50"
                    >
                        <FolderPlus size={18} />
                        New Category
                    </button>
                    <button
                        onClick={() => navigate('/admin/eggs/import')}
                        className="flex items-center gap-2 bg-primary text-white px-5 py-2.5 rounded-xl font-bold text-sm hover:brightness-110 transition-all shadow-lg shadow-primary/20"
                    >
                        <Plus size={18} />
                        Import Egg
                    </button>
                </div>
            </div>

            {/* Nests Section */}
            <div className="space-y-6">
                <div className="flex items-center justify-between">
                    <h2 className="text-lg font-bold tracking-tight flex items-center gap-2 uppercase text-muted tracking-widest">
                        <FolderPlus size={18} className="text-primary/50" /> Categories (Nests)
                    </h2>
                </div>

                <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-8">
                    {nests.filter(n => !n.parent_id).map(nest => (
                        <div key={nest.id} className="space-y-4">
                            {/* Root Category Header */}
                            <div className="panel-card p-6 flex flex-col justify-between group bg-primary/5 border-primary/20 hover:border-primary/40 transition-all rounded-[2rem]">
                                <div>
                                    <div className="flex items-center justify-between mb-4">
                                        <div className="w-12 h-12 rounded-2xl bg-primary/10 text-primary flex items-center justify-center">
                                            <Package size={24} />
                                        </div>
                                        <div className="flex items-center gap-1">
                                            <button onClick={() => setEditingNest(nest)} className="p-2 hover:bg-secondary rounded-lg text-muted transition-colors">
                                                <Edit size={16} />
                                            </button>
                                            <button onClick={() => deleteNest(nest.id)} className="p-2 hover:bg-red-500/10 hover:text-red-500 rounded-lg text-muted transition-colors">
                                                <Trash2 size={16} />
                                            </button>
                                        </div>
                                    </div>
                                    <h3 className="font-bold text-xl group-hover:text-primary transition-colors tracking-tight">{nest.name}</h3>
                                    <div className="flex items-center gap-2 mt-1">
                                        <span className="text-[10px] bg-primary/10 text-primary px-2 py-0.5 rounded-full font-bold uppercase tracking-widest">
                                            {eggs.filter(e => e.nest_id === nest.id || nests.filter(sn => sn.parent_id === nest.id).some(sn => sn.id === e.nest_id)).length} Templates
                                        </span>
                                    </div>
                                    <p className="text-xs text-muted font-medium mt-2 line-clamp-2">{nest.description || 'Main category managed via file system.'}</p>
                                </div>
                            </div>

                            {/* Sub-Categories & Eggs */}
                            <div className="pl-6 space-y-4 border-l-2 border-border/30 ml-6">
                                {nests.filter(sub => sub.parent_id === nest.id).map(sub => (
                                    <div key={sub.id} className="space-y-2">
                                        <div className="flex items-center justify-between group">
                                            <div className="flex items-center gap-2">
                                                <div className="w-1.5 h-1.5 rounded-full bg-primary" />
                                                <span className="font-bold text-sm text-foreground/80 uppercase tracking-widest">{sub.name}</span>
                                            </div>
                                            <div className="flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-all">
                                                <button onClick={() => setEditingNest(sub)} className="text-muted hover:text-foreground p-1"><Edit size={12} /></button>
                                                <button onClick={() => deleteNest(sub.id)} className="text-muted hover:text-red-500 p-1"><Trash2 size={12} /></button>
                                            </div>
                                        </div>

                                        {/* List Eggs in this Sub-Nest */}
                                        <div className="grid grid-cols-1 gap-2">
                                            {eggs.filter(egg => egg.nest_id === sub.id).map(egg => (
                                                <div key={egg.id} className="panel-card !p-3 !bg-background border-border/40 hover:border-primary/30 cursor-pointer group/egg flex items-center justify-between" onClick={() => setEditingEgg(egg)}>
                                                    <div className="flex items-center gap-2">
                                                        <Egg size={12} className="text-orange-500" />
                                                        <span className="text-[11px] font-bold text-muted group-hover/egg:text-primary transition-colors">{egg.name}</span>
                                                    </div>
                                                    <ChevronRight size={12} className="text-muted/30 group-hover/egg:text-primary group-hover/egg:translate-x-1 transition-all" />
                                                </div>
                                            ))}
                                            {eggs.filter(egg => egg.nest_id === sub.id).length === 0 && (
                                                <div className="text-[9px] font-bold text-muted/40 italic uppercase tracking-wider pl-4">No templates found</div>
                                            )}
                                        </div>
                                    </div>
                                ))}
                                {nests.filter(sub => sub.parent_id === nest.id).length === 0 && (
                                    <div className="py-2 text-[10px] font-bold text-muted/30 uppercase tracking-widest pl-4">No sub-categories</div>
                                )}
                            </div>
                        </div>
                    ))}
                </div>
            </div>

            {/* Eggs Section */}
            <div className="space-y-6">
                <div className="flex items-center justify-between bg-primary/[0.03] p-6 rounded-3xl border border-primary/10">
                    <h2 className="text-lg font-bold tracking-tight flex items-center gap-2 uppercase text-primary/70 tracking-widest">
                        <Egg size={18} /> Service Templates (Eggs)
                    </h2>
                    <div className="relative">
                        <Search className="absolute left-3 top-1/2 -translate-y-1/2 text-muted" size={16} />
                        <input
                            placeholder="Filter eggs..."
                            value={searchTerm}
                            onChange={e => setSearchTerm(e.target.value)}
                            className="bg-background border border-border/50 pl-10 pr-4 py-2 rounded-xl text-sm focus:outline-none focus:ring-2 focus:ring-primary/20 w-64 font-medium transition-all"
                        />
                    </div>
                </div>

                <div className="panel-card !p-0 overflow-hidden border-border/50">
                    <table className="w-full text-left">
                        <thead>
                            <tr className="bg-secondary/40 border-b border-border/50">
                                <th className="px-6 py-4 text-[11px] font-bold uppercase tracking-wider text-muted">Egg Name</th>
                                <th className="px-6 py-4 text-[11px] font-bold uppercase tracking-wider text-muted">Category</th>
                                <th className="px-6 py-4 text-[11px] font-bold uppercase tracking-wider text-muted">Startup Command</th>
                                <th className="px-6 py-4 text-[11px] font-bold uppercase tracking-wider text-muted text-right">Actions</th>
                            </tr>
                        </thead>
                        <tbody className="divide-y divide-border/30">
                            {eggs.filter(e => e.name.toLowerCase().includes(searchTerm.toLowerCase())).map(egg => (
                                <tr key={egg.id} className="hover:bg-primary/[0.01] transition-colors group">
                                    <td className="px-6 py-4">
                                        <div className="flex items-center gap-3">
                                            <div className="w-8 h-8 rounded-lg bg-orange-500/10 text-orange-500 flex items-center justify-center">
                                                <Egg size={16} />
                                            </div>
                                            <div>
                                                <div className="font-bold text-sm text-foreground">{egg.name}</div>
                                                <div className="text-[10px] text-muted font-medium truncate max-w-[200px]">{egg.uuid}</div>
                                            </div>
                                        </div>
                                    </td>
                                    <td className="px-6 py-4">
                                        <span className="px-2.5 py-1 rounded-lg bg-secondary text-[10px] font-bold text-muted uppercase border border-border/50">
                                            {egg.nest?.name || 'Uncategorized'}
                                        </span>
                                    </td>
                                    <td className="px-6 py-4">
                                        <code className="text-[10px] bg-secondary/80 px-2 py-1 rounded text-primary font-mono truncate max-w-[300px] block">
                                            {egg.startup_command}
                                        </code>
                                    </td>
                                    <td className="px-6 py-4 text-right">
                                        <div className="flex items-center justify-end gap-2 opacity-0 group-hover:opacity-100 transition-all">
                                            <button onClick={() => setEditingEgg(egg)} className="p-2 hover:bg-primary/10 hover:text-primary rounded-lg text-muted transition-colors">
                                                <Settings size={16} />
                                            </button>
                                            <button onClick={() => deleteEgg(egg.id)} className="p-2 hover:bg-red-500/10 hover:text-red-500 rounded-lg text-muted transition-colors">
                                                <Trash2 size={16} />
                                            </button>
                                        </div>
                                    </td>
                                </tr>
                            ))}
                        </tbody>
                    </table>
                </div>
            </div>

            {/* Editing Nest Modal */}
            {editingNest && (
                <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/60 backdrop-blur-sm animate-in fade-in">
                    <div className="bg-background border border-border/60 rounded-3xl w-full max-w-lg shadow-2xl scale-in">
                        <div className="p-6 border-b border-border/50 flex items-center justify-between">
                            <h3 className="font-bold text-lg">Edit Category</h3>
                            <button onClick={() => setEditingNest(null)} className="p-2 hover:bg-secondary rounded-xl"><X size={20} /></button>
                        </div>
                        <div className="p-6 space-y-4">
                            <div className="space-y-1.5">
                                <label className="text-[10px] font-bold text-muted uppercase tracking-widest pl-1">Nest Name</label>
                                <input className="input-field" value={editingNest.name} onChange={e => setEditingNest({ ...editingNest, name: e.target.value })} />
                            </div>
                            <div className="space-y-1.5">
                                <label className="text-[10px] font-bold text-muted uppercase tracking-widest pl-1">Parent Category</label>
                                <select
                                    className="input-field"
                                    value={editingNest.parent_id || ''}
                                    onChange={e => setEditingNest({ ...editingNest, parent_id: e.target.value === '' ? null : parseInt(e.target.value) })}
                                >
                                    <option value="">None (Top Level)</option>
                                    {nests.filter(n => !n.parent_id && n.id !== editingNest.id).map(n => (
                                        <option key={n.id} value={n.id}>{n.name}</option>
                                    ))}
                                </select>
                            </div>
                            <div className="space-y-1.5">
                                <label className="text-[10px] font-bold text-muted uppercase tracking-widest pl-1">Description</label>
                                <textarea className="input-field min-h-[100px]" value={editingNest.description} onChange={e => setEditingNest({ ...editingNest, description: e.target.value })} />
                            </div>
                        </div>
                        <div className="p-6 bg-secondary/10 flex justify-end gap-3">
                            <button onClick={saveNest} className="px-6 py-2 bg-primary text-white rounded-xl font-bold text-sm shadow-lg shadow-primary/20">Save Changes</button>
                        </div>
                    </div>
                </div>
            )}

            {/* Editing Egg Modal */}
            {editingEgg && (
                <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/60 backdrop-blur-sm animate-in fade-in">
                    <div className="bg-background border border-border/60 rounded-3xl w-full max-w-2xl shadow-2xl max-h-[90vh] overflow-y-auto scale-in">
                        <div className="p-8 border-b border-border/50 flex items-center justify-between sticky top-0 bg-background z-10">
                            <h3 className="font-bold text-xl">Modify Egg Template</h3>
                            <button onClick={() => setEditingEgg(null)} className="p-2 hover:bg-secondary rounded-xl"><X size={24} /></button>
                        </div>
                        <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
                            <div className="space-y-4">
                                <div className="space-y-1.5">
                                    <label className="text-[10px] font-bold text-muted uppercase tracking-widest pl-1">Service Name</label>
                                    <input className="input-field" value={editingEgg.name} onChange={e => setEditingEgg({ ...editingEgg, name: e.target.value })} />
                                </div>
                                <div className="space-y-1.5">
                                    <label className="text-[10px] font-bold text-muted uppercase tracking-widest pl-1">Category (Nest)</label>
                                    <select
                                        className="input-field"
                                        value={editingEgg.nest_id}
                                        onChange={e => setEditingEgg({ ...editingEgg, nest_id: parseInt(e.target.value) })}
                                    >
                                        {nests.map(n => (
                                            <option key={n.id} value={n.id}>{n.name}</option>
                                        ))}
                                    </select>
                                </div>
                                <div className="space-y-1.5">
                                    <label className="text-[10px] font-bold text-muted uppercase tracking-widest pl-1">Startup Command</label>
                                    <textarea className="input-field font-mono text-xs min-h-[80px]" value={editingEgg.startup_command} onChange={e => setEditingEgg({ ...editingEgg, startup_command: e.target.value })} />
                                </div>
                            </div>

                            <div className="space-y-4">
                                <div className="space-y-1.5">
                                    <label className="text-[10px] font-bold text-muted uppercase tracking-widest pl-1">Description</label>
                                    <textarea className="input-field min-h-[50px] text-xs" value={editingEgg.description} onChange={e => setEditingEgg({ ...editingEgg, description: e.target.value })} />
                                </div>
                                <div className="space-y-1.5">
                                    <label className="text-[10px] font-bold text-muted uppercase tracking-widest pl-1">Docker Image(s) JSON</label>
                                    <input className="input-field font-mono text-xs" value={editingEgg.docker_images} onChange={e => setEditingEgg({ ...editingEgg, docker_images: e.target.value })} placeholder='["image1"]' />
                                </div>
                                <div className="space-y-1.5">
                                    <label className="text-[10px] font-bold text-muted uppercase tracking-widest pl-1">Stop Command</label>
                                    <input className="input-field" value={editingEgg.stop_command} onChange={e => setEditingEgg({ ...editingEgg, stop_command: e.target.value })} placeholder="e.g. quit" />
                                </div>
                            </div>
                        </div>

                        <div className="flex items-center gap-3 py-2">
                            <div className="h-px bg-border flex-1" />
                            <span className="text-[10px] font-bold text-muted uppercase tracking-widest">Installation & Lifecycle</span>
                            <div className="h-px bg-border flex-1" />
                        </div>

                        <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
                            <div className="space-y-4">
                                <div className="space-y-1.5">
                                    <label className="text-[10px] font-bold text-muted uppercase tracking-widest pl-1">Install Container Image</label>
                                    <input className="input-field font-mono text-xs" value={editingEgg.install_container} onChange={e => setEditingEgg({ ...editingEgg, install_container: e.target.value })} />
                                </div>
                                <div className="space-y-1.5">
                                    <label className="text-[10px] font-bold text-muted uppercase tracking-widest pl-1">Script Entry Point</label>
                                    <input className="input-field font-mono text-xs" value={editingEgg.script_entry} onChange={e => setEditingEgg({ ...editingEgg, script_entry: e.target.value })} placeholder="e.g. bash" />
                                </div>
                            </div>
                            <div className="space-y-4">
                                <div className="space-y-1.5">
                                    <label className="text-[10px] font-bold text-muted uppercase tracking-widest pl-1">Installation Script</label>
                                    <textarea className="input-field font-mono text-xs min-h-[120px]" value={editingEgg.install_script} onChange={e => setEditingEgg({ ...editingEgg, install_script: e.target.value })} />
                                </div>
                            </div>
                        </div>

                        <div className="space-y-1.5">
                            <label className="text-[10px] font-bold text-muted uppercase tracking-widest pl-1">Raw Config JSON</label>
                            <textarea className="input-field font-mono text-xs min-h-[100px]" value={editingEgg.config} onChange={e => setEditingEgg({ ...editingEgg, config: e.target.value })} placeholder="{}" />
                        </div>

                        <div className="p-4 rounded-xl bg-orange-500/5 border border-orange-500/20 flex gap-3 text-orange-500">
                            <Info size={18} className="shrink-0" />
                            <div className="text-[10px] font-medium leading-relaxed">
                                Templates define how your services start, stop, and install. Be extremely careful when modifying scripts as it may break existing and future deployments.
                            </div>
                        </div>
                        <div className="p-8 bg-secondary/10 flex justify-end gap-3 sticky bottom-0 z-10 border-t border-border/50">
                            <button onClick={() => setEditingEgg(null)} className="px-6 py-2 rounded-xl font-bold text-sm text-muted">Cancel</button>
                            <button onClick={saveEgg} className="px-8 py-2 bg-primary text-white rounded-xl font-bold text-sm shadow-lg shadow-primary/20">Update Template</button>
                        </div>
                    </div>
                </div>
            )}
            {/* Create Nest Modal */}
            {showCreateNestModal && (
                <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/60 backdrop-blur-sm animate-in fade-in">
                    <div className="bg-background border border-border/60 rounded-3xl w-full max-w-lg shadow-2xl scale-in">
                        <div className="p-6 border-b border-border/50 flex items-center justify-between">
                            <h3 className="font-bold text-lg">Create New Category</h3>
                            <button onClick={() => setShowCreateNestModal(false)} className="p-2 hover:bg-secondary rounded-xl"><X size={20} /></button>
                        </div>
                        <div className="p-6 space-y-4">
                            <div className="space-y-1.5">
                                <label className="text-[10px] font-bold text-muted uppercase tracking-widest pl-1">Nest Name</label>
                                <input
                                    className="input-field"
                                    placeholder="e.g. Garry's Mod"
                                    value={newNest.name}
                                    onChange={e => setNewNest({ ...newNest, name: e.target.value })}
                                />
                            </div>
                            <div className="space-y-1.5">
                                <label className="text-[10px] font-bold text-muted uppercase tracking-widest pl-1">Parent Category</label>
                                <select
                                    className="input-field"
                                    value={newNest.parent_id || ''}
                                    onChange={e => setNewNest({ ...newNest, parent_id: e.target.value === '' ? null : parseInt(e.target.value) })}
                                >
                                    <option value="">None (Top Level)</option>
                                    {nests.filter(n => !n.parent_id).map(n => (
                                        <option key={n.id} value={n.id}>{n.name}</option>
                                    ))}
                                </select>
                            </div>
                            <div className="space-y-1.5">
                                <label className="text-[10px] font-bold text-muted uppercase tracking-widest pl-1">Description</label>
                                <textarea
                                    className="input-field min-h-[100px]"
                                    placeholder="Describe the types of eggs this category will contain..."
                                    value={newNest.description}
                                    onChange={e => setNewNest({ ...newNest, description: e.target.value })}
                                />
                            </div>
                        </div>
                        <div className="p-6 bg-secondary/10 flex justify-end gap-3">
                            <button
                                onClick={createNest}
                                disabled={!newNest.name}
                                className="px-8 py-2 bg-primary text-white rounded-xl font-bold text-sm shadow-lg shadow-primary/20 disabled:opacity-50"
                            >
                                Create Category
                            </button>
                        </div>
                    </div>
                </div>
            )}
        </div>
    );
}
