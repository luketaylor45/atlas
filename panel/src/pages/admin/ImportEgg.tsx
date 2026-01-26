import { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import api from '../../lib/api';
import { Download, AlertCircle, CheckCircle2, ChevronLeft, Terminal, Plus } from 'lucide-react';

export default function ImportEggPage() {
    const navigate = useNavigate();
    const [nests, setNests] = useState<any[]>([]);
    const [selectedNest, setSelectedNest] = useState<string>('');
    const [content, setContent] = useState<string>('');
    const [loading, setLoading] = useState(false);
    const [error, setError] = useState<string | null>(null);
    const [nestsError, setNestsError] = useState<string | null>(null);

    // Create Nest state
    const [showCreateNest, setShowCreateNest] = useState(false);
    const [newNestName, setNewNestName] = useState('');
    const [newNestDesc, setNewNestDesc] = useState('');
    const [creatingNest, setCreatingNest] = useState(false);

    const fetchNests = () => {
        api.get('/admin/nests')
            .then(res => {
                console.log('Nests response:', res.data);
                setNests(res.data || []);
                if (res.data && res.data.length > 0) setSelectedNest(res.data[0].id.toString());
            })
            .catch(err => {
                console.error('Failed to fetch nests:', err);
                setNestsError(err.response?.data?.error || 'Failed to load nests');
            });
    };

    useEffect(() => { fetchNests(); }, []);

    const handleCreateNest = async () => {
        if (!newNestName.trim()) return;
        setCreatingNest(true);
        try {
            const res = await api.post('/admin/nests', { name: newNestName, description: newNestDesc });
            setShowCreateNest(false);
            setNewNestName('');
            setNewNestDesc('');
            fetchNests();
            setSelectedNest(res.data.id.toString());
        } catch (err: any) {
            setError(err.response?.data?.error || 'Failed to create nest');
        } finally {
            setCreatingNest(false);
        }
    };

    const handleImport = async () => {
        if (!content.trim()) {
            setError("Please provide the egg JSON content.");
            return;
        }

        setLoading(true);
        setError(null);
        try {
            await api.post('/admin/eggs/import', {
                nest_id: parseInt(selectedNest),
                content: content
            });
            // Success!
            navigate('/admin/services/create');
        } catch (err: any) {
            console.error(err);
            setError(err.response?.data?.error || "Failed to import egg. Ensure the JSON is valid Pterodactyl format.");
        } finally {
            setLoading(false);
        }
    };

    const populatePreset = (type: string) => {
        const presets: Record<string, any> = {
            gmod: {
                name: "Garry's Mod",
                description: "The sandbox physics game by Facepunch Studios.",
                docker_images: { "ghcr.io/pterodactyl/games:source": "ghcr.io/pterodactyl/games:source" },
                startup: "./srcds_run -game garrysmod -console -port {{SERVER_PORT}} +maxplayers {{MAX_PLAYERS}} +map {{SERVER_MAP}} +sv_setsteamaccount {{STEAM_ACC}} +gamemode {{GAMEMODE}}",
                config: { stop: "quit", startup: '{"done": "gameserver Steam ID"}' },
                scripts: {
                    installation: {
                        container: "ghcr.io/pterodactyl/installers:debian",
                        entrypoint: "bash",
                        script: "#!/bin/bash\ncd /tmp\ncurl -sSL -o steamcmd.tar.gz https://steamcdn-a.akamaihd.net/client/installer/steamcmd_linux.tar.gz\nmkdir -p /mnt/server/steamcmd\ntar -xzvf steamcmd.tar.gz -C /mnt/server/steamcmd\ncd /mnt/server/steamcmd\n./steamcmd.sh +force_install_dir /mnt/server +login anonymous +app_update 4020 validate +quit || exit 1\nmkdir -p /mnt/server/.steam/sdk32\ncp -v linux32/steamclient.so /mnt/server/.steam/sdk32/steamclient.so"
                    }
                }
            },
            minecraft: {
                name: "Minecraft (Paper)",
                description: "High-performance Minecraft server using PaperMC.",
                docker_images: { "ghcr.io/pterodactyl/yolks:java_17": "ghcr.io/pterodactyl/yolks:java_17" },
                startup: "java -Xms128M -Xmx{{SERVER_MEMORY}}M -jar {{SERVER_JARFILE}}",
                config: { stop: "stop" },
                scripts: {
                    installation: {
                        container: "eclipse-temurin:17-jdk",
                        entrypoint: "bash",
                        script: "#!/bin/bash\ncd /mnt/server\nPAPER_VERSION=${PAPER_VERSION:-1.20.4}\ncurl -o server.jar -L \"https://papermc.io/api/v2/projects/paper/versions/${PAPER_VERSION}/builds/latest/downloads/paper-${PAPER_VERSION}.jar\"\necho \"eula=true\" > eula.txt"
                    }
                }
            }
        };

        if (presets[type]) {
            setContent(JSON.stringify(presets[type], null, 4));
        }
    };

    return (
        <div className="max-w-7xl mx-auto space-y-12 pb-20 animation-enter">
            {/* Header */}
            <div className="flex items-center justify-between">
                <div className="space-y-2">
                    <button
                        onClick={() => navigate(-1)}
                        className="flex items-center gap-2 text-muted hover:text-primary transition-colors text-sm font-bold uppercase tracking-widest"
                    >
                        <ChevronLeft size={16} /> Back
                    </button>
                    <h1 className="text-4xl font-bold tracking-tight">Import Service Egg</h1>
                    <p className="text-muted text-lg mt-2">Compatible with Pterodactyl and Pelican egg formats.</p>
                </div>
                <div className="w-16 h-16 rounded-2xl bg-primary/10 flex items-center justify-center text-primary">
                    <Download size={32} />
                </div>
            </div>

            <div className="grid grid-cols-1 lg:grid-cols-4 gap-8">
                <div className="lg:col-span-1 space-y-6">
                    {/* Presets Card */}
                    <div className="panel-card p-6 border-border/60">
                        <h4 className="text-xs font-bold uppercase tracking-widest text-muted mb-4 px-1">Quick Presets</h4>
                        <div className="flex flex-col gap-2">
                            {[
                                { id: 'gmod', name: "Garry's Mod", icon: '🎮' },
                                { id: 'minecraft', name: 'Minecraft (Paper)', icon: '🧊' }
                            ].map(preset => (
                                <button
                                    key={preset.id}
                                    onClick={() => populatePreset(preset.id)}
                                    className="flex items-center gap-3 p-3 rounded-xl bg-secondary/40 hover:bg-primary/10 hover:text-primary transition-all text-left text-sm font-bold border border-border/40 group"
                                >
                                    <span className="text-xl group-hover:scale-110 transition-transform">{preset.icon}</span>
                                    {preset.name}
                                </button>
                            ))}
                        </div>
                        <p className="text-[10px] text-muted font-medium mt-6 px-1 italic">Selecting a preset will overwrite the editor content.</p>
                    </div>

                    {/* Target Nest Card */}
                    <div className="panel-card p-6 border-border/60 space-y-6">
                        <div className="space-y-3">
                            <div className="flex justify-between items-center">
                                <label className="text-[10px] font-bold text-muted uppercase tracking-widest leading-none">Target Nest</label>
                                <button
                                    onClick={() => setShowCreateNest(!showCreateNest)}
                                    className="text-[10px] font-bold text-primary uppercase tracking-widest hover:underline"
                                >
                                    {showCreateNest ? 'Cancel' : '+ New'}
                                </button>
                            </div>

                            {showCreateNest ? (
                                <div className="space-y-3 p-4 bg-secondary/30 rounded-xl border border-border/50">
                                    <input
                                        type="text"
                                        className="input-field !py-2 !text-sm"
                                        placeholder="Nest Name"
                                        value={newNestName}
                                        onChange={(e) => setNewNestName(e.target.value)}
                                    />
                                    <button
                                        onClick={handleCreateNest}
                                        disabled={creatingNest || !newNestName.trim()}
                                        className="w-full btn-primary !py-2 !text-sm flex items-center justify-center gap-2 disabled:opacity-50"
                                    >
                                        <Plus size={14} /> {creatingNest ? 'Creating...' : 'Create Nest'}
                                    </button>
                                </div>
                            ) : nestsError ? (
                                <div className="text-xs text-red-500">{nestsError}</div>
                            ) : nests.length === 0 ? (
                                <div className="text-xs text-amber-500">No nests found. Create one above!</div>
                            ) : (
                                <select
                                    className="input-field !text-base appearance-none"
                                    value={selectedNest}
                                    onChange={(e) => setSelectedNest(e.target.value)}
                                >
                                    {nests.map(nest => (
                                        <option key={nest.id} value={nest.id}>{nest.name}</option>
                                    ))}
                                </select>
                            )}
                        </div>

                        {error && (
                            <div className="p-4 rounded-xl bg-red-500/10 border border-red-500/20 flex gap-3">
                                <AlertCircle size={20} className="text-red-500 shrink-0" />
                                <p className="text-xs text-red-500 font-medium leading-relaxed">{error}</p>
                            </div>
                        )}

                        <button
                            onClick={handleImport}
                            disabled={loading || !content}
                            className="w-full btn-primary !py-4 flex items-center justify-center gap-3 font-bold shadow-xl shadow-primary/20 disabled:opacity-50"
                        >
                            {loading ? "Importing..." : "Process Import"}
                            {!loading && <CheckCircle2 size={20} />}
                        </button>
                    </div>
                </div>

                <div className="lg:col-span-3">
                    <div className="panel-card p-8 border-border/60">
                        <div className="flex items-center justify-between mb-6">
                            <h3 className="text-xl font-bold flex items-center gap-3"><Terminal size={20} className="text-primary" /> Egg Configuration (JSON)</h3>
                            <span className="text-[10px] font-bold text-muted uppercase bg-secondary px-2 py-1 rounded">V2 Format Compatible</span>
                        </div>
                        <textarea
                            className="w-full h-[600px] bg-black/40 border border-border/60 rounded-xl p-4 font-mono text-xs focus:outline-none focus:border-primary/50 transition-colors resize-none overflow-y-auto scrollbar-thin leading-relaxed"
                            placeholder='{ "name": "Minecraft", ... }'
                            value={content}
                            onChange={(e) => setContent(e.target.value)}
                        />
                    </div>
                </div>
            </div>
        </div>
    );
}
