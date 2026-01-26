import { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import api from '../../lib/api';
import {
    Check, ChevronRight,
    Server, Activity,
    Globe,
    AlertTriangle
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
    const [selectedNest, setSelectedNest] = useState<any>(null);
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
                <div className="space-y-12">
                    <div className="text-center space-y-4 max-w-2xl mx-auto">
                        <h2 className="text-3xl font-bold">Select a Service Type</h2>
                        <p className="text-muted font-medium">Choose the platform or game you want to deploy. You'll be able to select the specific version next.</p>
                    </div>

                    <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-8">
                        {nests.filter(n => !n.parent_id).map(category => (
                            <div
                                key={category.id}
                                onClick={() => {
                                    setSelectedNest(category);
                                    setSelectedEgg(null); // Clear egg when switching categories
                                }}
                                className={clsx(
                                    "panel-card p-8 cursor-pointer transition-all border-2 flex flex-col gap-6 items-center text-center group",
                                    selectedNest?.id === category.id ? "border-primary bg-primary/5 shadow-xl scale-[1.03]" : "border-border/40 hover:border-primary/40 hover:-translate-y-1"
                                )}
                            >
                                <div className={clsx(
                                    "w-20 h-20 rounded-3xl flex items-center justify-center transition-colors shadow-lg",
                                    selectedNest?.id === category.id ? "bg-primary text-white" : "bg-secondary text-muted group-hover:bg-primary/10 group-hover:text-primary"
                                )}>
                                    <Globe size={40} />
                                </div>
                                <div>
                                    <h4 className="font-bold text-2xl mb-2">{category.name}</h4>
                                    <p className="text-sm text-muted font-medium line-clamp-2">{category.description}</p>
                                </div>

                                {selectedNest?.id === category.id && (
                                    <div className="absolute top-4 right-4">
                                        <div className="bg-primary text-white p-1 rounded-full"><Check size={16} /></div>
                                    </div>
                                )}
                            </div>
                        ))}
                    </div>

                    {/* Show Sub-Nests (Games) when category selected */}
                    {selectedNest && selectedNest.sub_nests && selectedNest.sub_nests.length > 0 && (
                        <div className="space-y-6 max-w-5xl mx-auto animation-enter">
                            <div className="flex items-center gap-3">
                                <div className="h-px bg-border flex-1" />
                                <span className="text-xs font-bold uppercase tracking-widest text-muted">Select Game from {selectedNest.name}</span>
                                <div className="h-px bg-border flex-1" />
                            </div>

                            <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
                                {selectedNest.sub_nests.map((game: any) => {
                                    const isGameSelected = selectedEgg && game.eggs?.some((e: any) => e.id === selectedEgg.id);
                                    return (
                                        <div
                                            key={game.id}
                                            onClick={() => {
                                                if (game.eggs && game.eggs.length > 0) {
                                                    setSelectedEgg(game.eggs[0]);
                                                }
                                            }}
                                            className={clsx(
                                                "panel-card p-6 cursor-pointer transition-all border-2 flex flex-col gap-4 items-center text-center",
                                                isGameSelected ? "border-primary bg-primary/5 ring-2 ring-primary/20" : "border-border/40 hover:border-primary/20"
                                            )}
                                        >
                                            <div className={clsx(
                                                "w-16 h-16 rounded-2xl flex items-center justify-center transition-colors",
                                                isGameSelected ? "bg-primary text-white" : "bg-secondary text-muted"
                                            )}>
                                                <Server size={32} />
                                            </div>
                                            <div>
                                                <h5 className="font-bold text-lg">{game.name}</h5>
                                                <p className="text-[10px] text-muted font-medium mt-1">{game.description}</p>
                                            </div>
                                        </div>
                                    );
                                })}
                            </div>
                        </div>
                    )}

                    {/* Show Egg Versions when a game is selected */}
                    {selectedEgg && (() => {
                        // Find which game this egg belongs to
                        const parentGame = selectedNest?.sub_nests?.find((g: any) =>
                            g.eggs?.some((e: any) => e.id === selectedEgg.id)
                        );

                        if (!parentGame || !parentGame.eggs || parentGame.eggs.length === 0) return null;

                        return (
                            <div className="space-y-6 max-w-4xl mx-auto animation-enter">
                                <div className="flex items-center gap-3">
                                    <div className="h-px bg-border flex-1" />
                                    <span className="text-xs font-bold uppercase tracking-widest text-muted">Select version of {parentGame.name}</span>
                                    <div className="h-px bg-border flex-1" />
                                </div>

                                <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                                    {parentGame.eggs.map((egg: any) => (
                                        <div
                                            key={egg.id}
                                            onClick={() => setSelectedEgg(egg)}
                                            className={clsx(
                                                "panel-card p-5 cursor-pointer transition-all border-2 flex justify-between items-center",
                                                selectedEgg?.id === egg.id ? "border-primary bg-primary/5 ring-1 ring-primary/20" : "border-border/40 hover:border-primary/20"
                                            )}
                                        >
                                            <div>
                                                <h5 className="font-bold">{egg.name}</h5>
                                                <p className="text-[11px] text-muted font-medium mt-1">{egg.description}</p>
                                            </div>
                                            <div className={clsx(
                                                "w-6 h-6 rounded-full border-2 flex items-center justify-center",
                                                selectedEgg?.id === egg.id ? "bg-primary border-primary" : "border-border"
                                            )}>
                                                {selectedEgg?.id === egg.id && <Check size={14} className="text-white" />}
                                            </div>
                                        </div>
                                    ))}
                                </div>
                            </div>
                        );
                    })()}

                    <div className="flex justify-end pt-8">
                        <button
                            disabled={!selectedEgg}
                            onClick={() => setStep(2)}
                            className="bg-primary px-10 py-3.5 rounded-2xl text-white font-bold flex items-center gap-3 enabled:hover:bg-primary/90 transition-all shadow-xl shadow-primary/20 disabled:opacity-50"
                        >
                            Next Step: Identification
                            <ChevronRight size={20} />
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
