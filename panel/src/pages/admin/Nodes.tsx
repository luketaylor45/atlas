import { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import api from '../../lib/api';
import { Server, HardDrive, Plus, Globe, CheckCircle, XCircle, Trash2, Settings, X, AlertTriangle } from 'lucide-react';
import clsx from 'clsx';

interface Node {
    id: number;
    name: string;
    address: string;
    port: string;
    is_online: boolean;
    location?: string;
    total_ram?: number;
    total_disk?: number;
}

export default function NodesPage() {
    const navigate = useNavigate();
    const [nodes, setNodes] = useState<Node[]>([]);
    const [loading, setLoading] = useState(true);
    const [editingNode, setEditingNode] = useState<Node | null>(null);

    const fetchNodes = () => {
        api.get<Node[]>('/admin/nodes')
            .then(res => setNodes(res.data))
            .catch(err => console.error(err))
            .finally(() => setLoading(false));
    };

    useEffect(() => {
        fetchNodes();
    }, []);

    const handleDelete = async (id: number) => {
        if (!confirm("Are you sure you want to delete this node? All services on this node will be disconnected from the controller.")) return;
        try {
            await api.delete(`/admin/nodes/${id}`);
            fetchNodes();
        } catch (err) {
            alert("Failed to delete node.");
        }
    };

    const handleEdit = (node: Node) => {
        setEditingNode({ ...node });
    };

    const saveEdit = async () => {
        if (!editingNode) return;
        try {
            await api.put(`/admin/nodes/${editingNode.id}`, editingNode);
            setEditingNode(null);
            fetchNodes();
        } catch (err) {
            alert("Failed to update node.");
        }
    };

    if (loading) return <div className="p-12 text-center animate-pulse text-muted font-bold tracking-widest mt-20 uppercase">Loading Nodes...</div>;

    return (
        <div className="space-y-8 animation-enter">
            <div className="flex items-center justify-between mb-2">
                <div>
                    <h2 className="text-2xl font-bold tracking-tight">Active Infrastructure</h2>
                    <p className="text-muted text-sm font-medium">Monitoring {nodes.length} compute nodes across your network.</p>
                </div>
                <button
                    onClick={() => navigate('/admin/nodes/create')}
                    className="flex items-center gap-2 bg-primary text-white px-5 py-2.5 rounded-xl font-bold text-sm hover:bg-primary/90 transition-all shadow-lg shadow-primary/20"
                >
                    <Plus size={18} />
                    Add Node
                </button>
            </div>

            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
                {nodes.map((node) => (
                    <div key={node.id} className="panel-card p-6 group relative border-border/50 hover:border-primary/30 transition-all">
                        <div className="flex justify-between items-start mb-6">
                            <div className="flex items-center gap-4">
                                <div className={clsx(
                                    "w-12 h-12 rounded-2xl flex items-center justify-center transition-colors shadow-inner",
                                    node.is_online ? "bg-emerald-500/10 text-emerald-500" : "bg-red-500/10 text-red-500"
                                )}>
                                    <Server size={24} />
                                </div>
                                <div>
                                    <h3 className="font-bold text-lg leading-tight group-hover:text-primary transition-colors">{node.name}</h3>
                                    <div className="flex items-center gap-1.5 text-xs text-muted font-medium mt-1">
                                        <Globe size={12} /> {node.address}:{node.port}
                                    </div>
                                </div>
                            </div>
                            <div className={clsx(
                                "flex items-center gap-1.5 px-3 py-1 rounded-full text-[10px] font-bold uppercase tracking-wider border",
                                node.is_online ? "bg-emerald-500/10 text-emerald-500 border-emerald-500/20" : "bg-red-500/10 text-red-500 border-red-500/20"
                            )}>
                                {node.is_online ? <CheckCircle size={10} /> : <XCircle size={10} />}
                                {node.is_online ? 'Online' : 'Offline'}
                            </div>
                        </div>

                        <div className="grid grid-cols-2 gap-4">
                            <div className="bg-secondary/40 p-4 rounded-xl border border-border/30">
                                <div className="flex items-center gap-2 text-muted text-[10px] font-bold uppercase tracking-widest mb-2">
                                    <HardDrive size={12} className="text-purple-500" /> RAM
                                </div>
                                <span className="text-xl font-bold tracking-tight">{node.total_ram || 0} MB</span>
                            </div>
                            <div className="bg-secondary/40 p-4 rounded-xl border border-border/30">
                                <div className="flex items-center gap-2 text-muted text-[10px] font-bold uppercase tracking-widest mb-2">
                                    <Server size={12} className="text-blue-500" /> Disk
                                </div>
                                <span className="text-xl font-bold tracking-tight">{node.total_disk || 0} MB</span>
                            </div>
                        </div>

                        <div className="mt-8 pt-4 border-t border-border/30 flex items-center justify-between">
                            <div className="text-[10px] font-bold text-muted uppercase tracking-widest flex items-center gap-2">
                                Last Heartbeat <span className="text-foreground">Just now</span>
                            </div>
                            <div className="flex items-center gap-2">
                                <button
                                    onClick={() => handleEdit(node)}
                                    className="p-2 hover:bg-primary/10 text-muted hover:text-primary rounded-lg transition-colors border border-transparent hover:border-primary/20"
                                    title="Edit Node"
                                >
                                    <Settings size={16} />
                                </button>
                                <button
                                    onClick={() => handleDelete(node.id)}
                                    className="p-2 hover:bg-red-500/10 text-muted hover:text-red-500 rounded-lg transition-colors border border-transparent hover:border-red-500/20"
                                    title="Delete Node"
                                >
                                    <Trash2 size={16} />
                                </button>
                            </div>
                        </div>
                    </div>
                ))}

                <button
                    onClick={() => navigate('/admin/nodes/create')}
                    className="panel-card p-8 flex flex-col items-center justify-center gap-4 border-dashed border-2 border-border/60 hover:border-primary/50 hover:bg-primary/[0.02] transition-all group min-h-[220px]"
                >
                    <div className="w-14 h-14 rounded-full bg-secondary group-hover:bg-primary group-hover:text-white transition-all flex items-center justify-center shadow-inner">
                        <Plus size={32} />
                    </div>
                    <div className="text-center">
                        <span className="block font-bold text-lg">Deploy New Node</span>
                        <span className="text-xs text-muted font-medium mt-1">Add manual compute resources to your cluster</span>
                    </div>
                </button>
            </div>

            {/* Edit Modal */}
            {editingNode && (
                <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/60 backdrop-blur-sm animate-in fade-in duration-200">
                    <div className="bg-background border border-border/60 rounded-3xl w-full max-w-xl shadow-2xl overflow-hidden scale-in">
                        <div className="p-8 border-b border-border/50 flex items-center justify-between bg-secondary/20">
                            <div className="flex items-center gap-3">
                                <div className="w-10 h-10 rounded-xl bg-primary/10 text-primary flex items-center justify-center">
                                    <Settings size={20} />
                                </div>
                                <div>
                                    <h3 className="font-bold text-xl">Modify Node</h3>
                                    <p className="text-xs text-muted font-medium">Update infrastructure details for {editingNode.name}</p>
                                </div>
                            </div>
                            <button onClick={() => setEditingNode(null)} className="p-2 hover:bg-secondary rounded-xl transition-colors">
                                <X size={20} className="text-muted" />
                            </button>
                        </div>
                        <div className="p-8 space-y-6">
                            <div className="grid grid-cols-2 gap-6">
                                <div className="space-y-2">
                                    <label className="text-[10px] font-bold text-muted uppercase tracking-widest pl-1">Node Name</label>
                                    <input
                                        className="input-field"
                                        value={editingNode.name}
                                        onChange={e => setEditingNode({ ...editingNode, name: e.target.value })}
                                    />
                                </div>
                                <div className="space-y-2">
                                    <label className="text-[10px] font-bold text-muted uppercase tracking-widest pl-1">Location</label>
                                    <input
                                        className="input-field"
                                        value={editingNode.location || ''}
                                        onChange={e => setEditingNode({ ...editingNode, location: e.target.value })}
                                        placeholder="e.g. London, UK"
                                    />
                                </div>
                            </div>
                            <div className="grid grid-cols-2 gap-6">
                                <div className="space-y-2">
                                    <label className="text-[10px] font-bold text-muted uppercase tracking-widest pl-1">IP Address / Domain</label>
                                    <input
                                        className="input-field"
                                        value={editingNode.address}
                                        onChange={e => setEditingNode({ ...editingNode, address: e.target.value })}
                                    />
                                </div>
                                <div className="space-y-2">
                                    <label className="text-[10px] font-bold text-muted uppercase tracking-widest pl-1">API Port</label>
                                    <input
                                        className="input-field"
                                        value={editingNode.port}
                                        onChange={e => setEditingNode({ ...editingNode, port: e.target.value })}
                                    />
                                </div>
                            </div>
                            <div className="grid grid-cols-2 gap-6">
                                <div className="space-y-2">
                                    <label className="text-[10px] font-bold text-muted uppercase tracking-widest pl-1">Total RAM (MB)</label>
                                    <input
                                        type="number"
                                        className="input-field"
                                        value={editingNode.total_ram || 0}
                                        onChange={e => setEditingNode({ ...editingNode, total_ram: parseInt(e.target.value) })}
                                    />
                                </div>
                                <div className="space-y-2">
                                    <label className="text-[10px] font-bold text-muted uppercase tracking-widest pl-1">Total Disk (MB)</label>
                                    <input
                                        type="number"
                                        className="input-field"
                                        value={editingNode.total_disk || 0}
                                        onChange={e => setEditingNode({ ...editingNode, total_disk: parseInt(e.target.value) })}
                                    />
                                </div>
                            </div>
                            <div className="p-4 rounded-xl bg-amber-500/5 border border-amber-500/20 flex gap-3">
                                <AlertTriangle className="text-amber-500 shrink-0 mt-0.5" size={16} />
                                <div className="text-[10px] text-amber-500/80 font-medium leading-relaxed">
                                    If you change the address or port, ensure the node's daemon is correctly configured and reachable at the new location.
                                </div>
                            </div>
                        </div>
                        <div className="p-8 bg-secondary/10 flex items-center justify-end gap-3">
                            <button
                                onClick={() => setEditingNode(null)}
                                className="px-6 py-2.5 rounded-xl font-bold text-sm text-muted hover:text-foreground transition-colors"
                            >
                                Cancel
                            </button>
                            <button
                                onClick={saveEdit}
                                className="px-8 py-2.5 bg-primary text-white rounded-xl font-bold text-sm shadow-lg shadow-primary/20 hover:bg-primary/90 transition-all"
                            >
                                Save Changes
                            </button>
                        </div>
                    </div>
                </div>
            )}
        </div>
    );
}
