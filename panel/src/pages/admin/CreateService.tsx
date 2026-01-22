import { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import api from '../../lib/api';
import {
    Check, ChevronRight,
    Server, Activity,
    Globe,
    CheckCircle2, AlertTriangle
} from 'lucide-react';
import clsx from 'clsx';

export default function CreateServicePage() {
    const navigate = useNavigate();
    const [step, setStep] = useState(1);
    const [loading, setLoading] = useState(false);

    // Data lists
    const [nodes, setNodes] = useState<any[]>([]);
    const [nests, setNests] = useState<any[]>([]);
    const [users, setUsers] = useState<any[]>([]);

    // Selections
    const [selectedEgg, setSelectedEgg] = useState<any>(null);
    const [selectedNode, setSelectedNode] = useState<any>(null);
    const [selectedUser, setSelectedUser] = useState<any>(null);
    const [name, setName] = useState('');
    const [ram, setRam] = useState(1024);
    const [cpu, setCpu] = useState(100);
    const [disk, setDisk] = useState(5120);
    const [port, setPort] = useState(25565);
    const [eggVars, setEggVars] = useState<Record<string, string>>({});
    const [selectedImage, setSelectedImage] = useState('');

    useEffect(() => {
        const fetchData = async () => {
            const [nRes, nestsRes, uRes] = await Promise.all([
                api.get('/admin/nodes'),
                api.get('/admin/nests'),
                api.get('/admin/users')
            ]);
            setNodes(nRes.data);
            setNests(nestsRes.data);
            setUsers(uRes.data);
        };
        fetchData();
    }, []);

    useEffect(() => {
        if (selectedEgg) {
            const defaults: Record<string, string> = {};
            selectedEgg.variables?.forEach((v: any) => {
                defaults[v.environment_variable] = v.default_value;
            });
            setEggVars(defaults);

            // Set default image
            try {
                const images = JSON.parse(selectedEgg.docker_images);
                if (images.length > 0) setSelectedImage(images[0]);
            } catch (e) {
                setSelectedImage('');
            }
        }
    }, [selectedEgg]);

    const handleCreate = async () => {
        setLoading(true);
        try {
            await api.post('/admin/services', {
                name,
                node_id: selectedNode.id,
                egg_id: selectedEgg.id,
                user_id: selectedUser.id,
                memory: ram,
                cpu: cpu,
                disk: disk,
                port: port,
                environment: JSON.stringify(eggVars),
                docker_image: selectedImage
            });
            navigate('/admin/services');
        } catch (err: any) {
            console.error(err);
            const errorMsg = err.response?.data?.error || "Failed to create service. Please check daemon logs.";
            alert(`Error: ${errorMsg}`);
        } finally {
            setLoading(false);
        }
    };


    return (
        <div className="max-w-6xl mx-auto space-y-12 pb-20 animation-enter">
            {/* Header */}
            <div className="flex items-center justify-between">
                <div>
                    <h1 className="text-4xl font-bold tracking-tight">Create Service</h1>
                    <p className="text-muted text-lg mt-2">Provision a new service instance on your infrastructure.</p>
                </div>
                <div className="flex items-center gap-3">
                    {[1, 2, 3].map(i => (
                        <div key={i} className="flex items-center gap-3">
                            <div className={clsx(
                                "w-10 h-10 rounded-xl flex items-center justify-center font-bold text-sm transition-all shadow-sm",
                                step === i ? "bg-primary text-white shadow-primary/20 scale-110" :
                                    step > i ? "bg-emerald-500 text-white shadow-emerald-500/20" : "bg-secondary text-muted"
                            )}>
                                {step > i ? <Check size={18} /> : i}
                            </div>
                            {i < 3 && <div className={clsx("w-8 h-0.5 rounded-full transition-colors", step > i ? "bg-emerald-500" : "bg-border")} />}
                        </div>
                    ))}
                </div>
            </div>

            {/* Step 1: Template Selection */}
            {step === 1 && (
                <div className="space-y-10">
                    <div className="grid grid-cols-1 md:grid-cols-2 gap-8">
                        {nests.map(nest => (
                            <div key={nest.id} className="space-y-6">
                                <div className="flex items-center gap-2 px-4 py-1.5 bg-secondary/50 rounded-lg w-fit border border-border/50">
                                    <span className="text-xs font-bold uppercase tracking-widest text-muted">{nest.name} Category</span>
                                </div>
                                <div className="space-y-4">
                                    {nest.eggs?.map((egg: any) => (
                                        <div
                                            key={egg.id}
                                            onClick={() => setSelectedEgg(egg)}
                                            className={clsx(
                                                "panel-card p-6 cursor-pointer transition-all border-2",
                                                selectedEgg?.id === egg.id ? "border-primary bg-primary/5 shadow-lg shadow-primary/5 scale-[1.02]" : "border-border/40 hover:border-primary/30"
                                            )}
                                        >
                                            <div className="flex justify-between items-start mb-4">
                                                <h4 className="font-bold text-xl">{egg.name}</h4>
                                                {selectedEgg?.id === egg.id && <CheckCircle2 className="text-primary" size={24} />}
                                            </div>
                                            <p className="text-sm text-muted font-medium leading-relaxed">{egg.description || "Deploy this template with standard configuration on any node."}</p>
                                        </div>
                                    ))}
                                </div>
                            </div>
                        ))}
                    </div>

                    <div className="flex justify-end pt-8">
                        <button
                            disabled={!selectedEgg}
                            onClick={() => setStep(2)}
                            className="btn-primary group !px-10 !py-4 text-base font-bold shadow-2xl"
                        >
                            Continue to Allocation <ChevronRight size={20} className="group-hover:translate-x-1 transition-transform" />
                        </button>
                    </div>
                </div>
            )}

            {/* Step 2: Configuration & Allocation */}
            {step === 2 && (
                <div className="grid grid-cols-1 lg:grid-cols-3 gap-12 items-start">
                    <div className="lg:col-span-2 space-y-10">
                        {/* Basic Info */}
                        <div className="panel-card p-8 border-border/60">
                            <h3 className="text-xl font-bold mb-8 flex items-center gap-3"><Globe className="text-primary" /> Service Identity</h3>
                            <div className="grid grid-cols-1 md:grid-cols-2 gap-8">
                                <div className="space-y-3">
                                    <label className="text-[10px] font-bold text-muted uppercase tracking-widest">Service Name</label>
                                    <input
                                        type="text"
                                        placeholder="e.g. Minecraft Server #1"
                                        className="input-field !text-base"
                                        value={name}
                                        onChange={(e) => setName(e.target.value)}
                                    />
                                </div>
                                <div className="space-y-3">
                                    <label className="text-[10px] font-bold text-muted uppercase tracking-widest">Owner User</label>
                                    <select
                                        className="input-field !text-base appearance-none"
                                        onChange={(e) => setSelectedUser(users.find(u => u.id === parseInt(e.target.value)))}
                                        value={selectedUser?.id || ""}
                                    >
                                        <option value="">Select an owner account...</option>
                                        {users.map(u => (
                                            <option key={u.id} value={u.id}>{u.email} ({u.username})</option>
                                        ))}
                                    </select>
                                </div>
                            </div>
                        </div>

                        {/* Node Selection */}
                        <div className="panel-card p-8 border-border/60">
                            <h3 className="text-xl font-bold mb-8 flex items-center gap-3"><Server className="text-primary" /> Hosting Node</h3>
                            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                                {nodes.map(node => (
                                    <div
                                        key={node.id}
                                        onClick={() => setSelectedNode(node)}
                                        className={clsx(
                                            "p-4 rounded-2xl border-2 transition-all cursor-pointer flex items-center justify-between",
                                            selectedNode?.id === node.id ? "border-primary bg-primary/5" : "border-border/40 hover:border-primary/20"
                                        )}
                                    >
                                        <div className="flex items-center gap-4">
                                            <div className={clsx("w-10 h-10 rounded-xl flex items-center justify-center", node.is_online ? "bg-emerald-500/10 text-emerald-500" : "bg-red-500/10 text-red-500")}>
                                                <Server size={20} />
                                            </div>
                                            <div>
                                                <div className="font-bold text-sm">{node.name}</div>
                                                <div className="text-[10px] text-muted font-bold uppercase tracking-wider">{node.address}</div>
                                                <div className="text-[9px] text-muted font-medium mt-1">
                                                    RAM: {node.total_ram || 0} MB | Disk: {node.total_disk || 0} MB
                                                </div>
                                            </div>
                                        </div>
                                        {selectedNode?.id === node.id && <Check className="text-primary" size={20} />}
                                    </div>
                                ))}
                            </div>
                        </div>

                        {/* Image Selection */}
                        <div className="panel-card p-8 border-border/60">
                            <h3 className="text-xl font-bold mb-8 flex items-center gap-3"><Activity className="text-primary" /> Container Image</h3>
                            <div className="space-y-3">
                                <label className="text-[10px] font-bold text-muted uppercase tracking-widest">Select Runtime Version</label>
                                <select
                                    className="input-field !text-base"
                                    value={selectedImage}
                                    onChange={(e) => setSelectedImage(e.target.value)}
                                >
                                    {(() => {
                                        try {
                                            const images = JSON.parse(selectedEgg.docker_images);
                                            return images.map((img: string) => (
                                                <option key={img} value={img}>{img}</option>
                                            ));
                                        } catch (e) {
                                            return <option value={selectedImage}>{selectedImage}</option>;
                                        }
                                    })()}
                                </select>
                                <p className="text-[10px] text-muted font-medium">This image will be used to run your service container.</p>
                            </div>
                        </div>

                        {/* Resource Allocation & Variables */}
                        <div className="grid grid-cols-1 md:grid-cols-2 gap-8">
                            <div className="panel-card p-8 border-border/60 h-fit">
                                <h3 className="text-xl font-bold mb-8 flex items-center gap-3"><Activity className="text-primary" /> Resource Allocation</h3>
                                <div className="space-y-8">
                                    <div className="space-y-3">
                                        <div className="flex justify-between items-end">
                                            <label className="text-[10px] font-bold text-muted uppercase tracking-widest leading-none">Memory Usage (MB)</label>
                                            <span className="text-sm font-bold text-primary">{ram / 1024} GB</span>
                                        </div>
                                        <input
                                            type="range"
                                            min="512"
                                            max="32768"
                                            step="512"
                                            value={ram}
                                            onChange={(e) => setRam(parseInt(e.target.value))}
                                            className="w-full accent-primary h-1.5 bg-secondary rounded-full appearance-none transition-all"
                                        />
                                    </div>
                                    <div className="space-y-3">
                                        <div className="flex justify-between items-end">
                                            <label className="text-[10px] font-bold text-muted uppercase tracking-widest leading-none">CPU Limit (%)</label>
                                            <span className="text-sm font-bold text-primary">{cpu}%</span>
                                        </div>
                                        <input
                                            type="range"
                                            min="50"
                                            max="800"
                                            step="50"
                                            value={cpu}
                                            onChange={(e) => setCpu(parseInt(e.target.value))}
                                            className="w-full accent-primary h-1.5 bg-secondary rounded-full appearance-none transition-all"
                                        />
                                    </div>
                                    <div className="grid grid-cols-2 gap-4">
                                        <div className="space-y-3">
                                            <label className="text-[10px] font-bold text-muted uppercase tracking-widest block">Storage (MB)</label>
                                            <input
                                                type="number"
                                                className="input-field"
                                                value={disk}
                                                onChange={(e) => setDisk(parseInt(e.target.value))}
                                            />
                                        </div>
                                        <div className="space-y-3">
                                            <label className="text-[10px] font-bold text-muted uppercase tracking-widest block">Port</label>
                                            <input
                                                type="number"
                                                className="input-field"
                                                value={port}
                                                onChange={(e) => setPort(parseInt(e.target.value))}
                                            />
                                        </div>
                                    </div>
                                </div>
                            </div>

                            <div className="panel-card p-8 border-border/60">
                                <h3 className="text-xl font-bold mb-8 flex items-center gap-3"><Activity className="text-primary" /> Template Variables</h3>
                                <div className="space-y-6">
                                    {selectedEgg.variables?.length > 0 ? (
                                        selectedEgg.variables.map((v: any) => (
                                            <div key={v.id} className="space-y-3">
                                                <div className="flex justify-between">
                                                    <label className="text-[10px] font-bold text-muted uppercase tracking-widest">{v.name}</label>
                                                    {!v.user_editable && <span className="text-[8px] font-bold text-primary/50 uppercase tracking-widest">Read Only</span>}
                                                </div>
                                                <input
                                                    type={v.input_type === 'password' ? 'password' : 'text'}
                                                    className="input-field !py-3 !text-sm"
                                                    value={eggVars[v.environment_variable] || ""}
                                                    disabled={!v.user_editable}
                                                    onChange={(e) => setEggVars({ ...eggVars, [v.environment_variable]: e.target.value })}
                                                    placeholder={v.default_value}
                                                />
                                                <p className="text-[10px] text-muted leading-tight">{v.description}</p>
                                            </div>
                                        ))
                                    ) : (
                                        <div className="py-10 text-center space-y-3 opacity-50">
                                            <div className="w-12 h-12 rounded-2xl bg-secondary mx-auto flex items-center justify-center">
                                                <Activity size={20} className="text-muted" />
                                            </div>
                                            <p className="text-xs font-bold text-muted uppercase tracking-widest">No custom variables</p>
                                        </div>
                                    )}
                                </div>
                            </div>
                        </div>
                    </div>

                    {/* Summary Sidebar */}
                    <div className="space-y-6 lg:sticky lg:top-8">
                        <div className="panel-card p-8 bg-primary/5 border-primary/20 shadow-2xl overflow-hidden relative group">
                            <div className="absolute -right-10 -top-10 w-40 h-40 bg-primary/10 rounded-full blur-3xl" />
                            <h3 className="text-sm font-bold text-primary uppercase tracking-[0.2em] mb-8 flex items-center gap-2">
                                <Activity size={16} /> Summary Details
                            </h3>
                            <div className="space-y-6">
                                <div className="flex flex-col gap-1">
                                    <span className="text-[10px] font-bold text-muted uppercase tracking-widest">Selected Template</span>
                                    <span className="font-bold text-lg">{selectedEgg?.name || "None"}</span>
                                </div>
                                <div className="flex flex-col gap-1">
                                    <span className="text-[10px] font-bold text-muted uppercase tracking-widest">Target Node</span>
                                    <span className="font-bold text-lg">{selectedNode?.name || "None"}</span>
                                </div>
                                <div className="h-px bg-primary/10" />
                                <div className="grid grid-cols-2 gap-4">
                                    <div className="flex flex-col gap-1">
                                        <span className="text-[10px] font-bold text-muted uppercase tracking-widest">RAM</span>
                                        <span className="font-bold">{ram} MB</span>
                                    </div>
                                    <div className="flex flex-col gap-1">
                                        <span className="text-[10px] font-bold text-muted uppercase tracking-widest">CPU</span>
                                        <span className="font-bold">{cpu}%</span>
                                    </div>
                                </div>
                            </div>
                        </div>

                        <div className="space-y-3">
                            <button
                                onClick={handleCreate}
                                disabled={loading || !name || !selectedNode || !selectedUser}
                                className="w-full btn-primary !py-5 !rounded-2xl shadow-xl shadow-primary/20 flex items-center justify-center gap-3 font-bold text-lg disabled:opacity-50"
                            >
                                {loading ? "Deploying..." : "Finalize & Launch"}
                                {!loading && <Check size={24} />}
                            </button>
                            <button
                                onClick={() => setStep(1)}
                                className="w-full py-4 text-sm font-bold text-muted hover:text-foreground transition-all uppercase tracking-widest"
                            >
                                Change Template
                            </button>
                        </div>

                        {!selectedNode && (
                            <div className="p-4 rounded-xl bg-amber-500/10 border border-amber-500/20 flex gap-3">
                                <AlertTriangle size={20} className="text-amber-500 shrink-0" />
                                <p className="text-xs text-amber-500/80 font-medium leading-relaxed">Please select a hosting node before proceeding with deployment.</p>
                            </div>
                        )}
                    </div>
                </div>
            )}
        </div>
    );
}
