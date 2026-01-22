import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import api from '../../lib/api';
import { Server, Cpu, MapPin, Globe } from 'lucide-react';
import clsx from 'clsx';

export default function CreateNodePage() {
    const navigate = useNavigate();
    const [loading, setLoading] = useState(false);
    const [error, setError] = useState('');

    const [formData, setFormData] = useState({
        name: '',
        address: '',
        port: '8081',
        sftp_port: '2022',
        total_ram: 1024,
        total_disk: 10240,
        location: 'Unknown'
    });

    const regions = [
        { code: 'US', name: 'North America', icon: '🌎' },
        { code: 'EU', name: 'Europe', icon: '🌍' },
        { code: 'AS', name: 'Asia', icon: '🌏' },
    ];

    const [successData, setSuccessData] = useState<{ name: string, token: string } | null>(null);

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault();
        setLoading(true);
        setError('');

        try {
            const res = await api.post('/admin/nodes', formData);
            setSuccessData({ name: res.data.node.name, token: res.data.token });
        } catch (err: any) {
            setError(err.response?.data?.error || 'Failed to create node');
        } finally {
            setLoading(false);
        }
    };

    if (successData) {
        return (
            <div className="max-w-2xl mx-auto my-12 panel-card p-8 text-center">
                <div className="w-16 h-16 bg-emerald-500/10 text-emerald-500 rounded-full flex items-center justify-center mx-auto mb-6">
                    <Server size={32} />
                </div>
                <h2 className="text-2xl font-bold mb-2">Node Deployed Successfully!</h2>
                <p className="text-muted mb-8">Save this token immediately. You will need it to configure your Daemon.</p>

                <div className="bg-black/30 p-4 rounded-xl font-mono text-lg mb-8 break-all border border-white/5 select-all">
                    {/* Access token from response. If hidden in model, API needs to return it manually */}
                    {successData.token || "Token Hidden - Check logs if testing"}
                </div>

                <button
                    onClick={() => navigate('/admin/nodes')}
                    className="px-8 py-3 bg-primary text-white rounded-xl font-medium hover:bg-primary/90"
                >
                    Return to Node List
                </button>
            </div>
        );
    }

    return (
        <div className="max-w-4xl mx-auto">
            <div className="mb-8 flex items-center justify-between">
                <div>
                    <h1 className="text-3xl font-bold tracking-tight mb-2">Deploy New Node</h1>
                    <p className="text-muted">Connect a new daemon to your Atlas cluster.</p>
                </div>
            </div>

            {error && (
                <div className="bg-red-500/10 border border-red-500/20 text-red-500 p-4 rounded-xl mb-6">
                    {error}
                </div>
            )}

            <form onSubmit={handleSubmit} className="grid grid-cols-1 lg:grid-cols-3 gap-8">
                {/* Left Column: Details */}
                <div className="lg:col-span-2 space-y-6">
                    {/* Connection Info */}
                    <div className="panel-card p-6">
                        <h3 className="text-lg font-semibold mb-4 flex items-center gap-2">
                            <Server size={18} className="text-primary" />
                            Connection Details
                        </h3>

                        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                            <div className="space-y-2">
                                <label className="text-xs font-medium uppercase tracking-wide text-muted">Node Name</label>
                                <input
                                    required
                                    className="input-field"
                                    placeholder="e.g. Phoenix-01"
                                    value={formData.name}
                                    onChange={e => setFormData({ ...formData, name: e.target.value })}
                                />
                            </div>
                            <div className="space-y-2">
                                <label className="text-xs font-medium uppercase tracking-wide text-muted">FQDN / IP Address</label>
                                <input
                                    required
                                    className="input-field"
                                    placeholder="node.example.com"
                                    value={formData.address}
                                    onChange={e => setFormData({ ...formData, address: e.target.value })}
                                />
                            </div>
                            <div className="space-y-2">
                                <label className="text-xs font-medium uppercase tracking-wide text-muted">Daemon Port</label>
                                <input
                                    required
                                    className="input-field"
                                    value={formData.port}
                                    onChange={e => setFormData({ ...formData, port: e.target.value })}
                                />
                            </div>
                            <div className="space-y-2">
                                <label className="text-xs font-medium uppercase tracking-wide text-muted">SFTP Port</label>
                                <input
                                    required
                                    className="input-field"
                                    value={formData.sftp_port}
                                    onChange={e => setFormData({ ...formData, sftp_port: e.target.value })}
                                />
                            </div>
                        </div>
                    </div>

                    {/* Resources */}
                    <div className="panel-card p-6">
                        <h3 className="text-lg font-semibold mb-4 flex items-center gap-2">
                            <Cpu size={18} className="text-purple-500" />
                            Resource Allocation
                        </h3>

                        <div className="grid grid-cols-1 gap-6">
                            <div className="space-y-4">
                                <div className="flex justify-between">
                                    <label className="text-xs font-medium uppercase tracking-wide text-muted">Total RAM (MB)</label>
                                    <span className="font-mono text-primary">{formData.total_ram} MB</span>
                                </div>
                                <input
                                    type="range"
                                    min="1024"
                                    max="65536"
                                    step="1024"
                                    className="w-full h-2 bg-secondary rounded-lg appearance-none cursor-pointer accent-primary"
                                    value={formData.total_ram}
                                    onChange={e => setFormData({ ...formData, total_ram: parseInt(e.target.value) })}
                                />
                            </div>

                            <div className="space-y-4">
                                <div className="flex justify-between">
                                    <label className="text-xs font-medium uppercase tracking-wide text-muted">Total Disk (MB)</label>
                                    <span className="font-mono text-purple-500">{formData.total_disk} MB</span>
                                </div>
                                <input
                                    type="range"
                                    min="10240"
                                    max="1048576"
                                    step="10240"
                                    className="w-full h-2 bg-secondary rounded-lg appearance-none cursor-pointer accent-purple-500"
                                    value={formData.total_disk}
                                    onChange={e => setFormData({ ...formData, total_disk: parseInt(e.target.value) })}
                                />
                            </div>
                        </div>
                    </div>
                </div>

                {/* Right Column: Location & Submit */}
                <div className="space-y-6">
                    <div className="panel-card p-6">
                        <h3 className="text-lg font-semibold mb-4 flex items-center gap-2">
                            <MapPin size={18} className="text-emerald-500" />
                            Location
                        </h3>

                        <div className="grid grid-cols-1 gap-3">
                            {regions.map(r => (
                                <button
                                    key={r.code}
                                    type="button"
                                    onClick={() => setFormData({ ...formData, location: r.name })}
                                    className={clsx(
                                        "flex items-center gap-3 p-3 rounded-xl border transition-all text-left",
                                        formData.location === r.name
                                            ? "border-emerald-500 bg-emerald-500/10 text-emerald-500"
                                            : "border-border/50 bg-secondary/30 hover:bg-secondary/50 text-muted hover:text-foreground"
                                    )}
                                >
                                    <span className="text-xl">{r.icon}</span>
                                    <span className="font-medium text-sm">{r.name}</span>
                                </button>
                            ))}

                            <div className="relative mt-2">
                                <div className="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none">
                                    <Globe size={14} className="text-muted" />
                                </div>
                                <input
                                    className="input-field pl-9"
                                    placeholder="Custom Location..."
                                    value={formData.location}
                                    onChange={e => setFormData({ ...formData, location: e.target.value })}
                                />
                            </div>
                        </div>
                    </div>

                    <button
                        type="submit"
                        disabled={loading}
                        className="w-full py-4 rounded-xl bg-primary text-white font-bold text-lg hover:bg-primary/90 transition-all shadow-lg shadow-primary/25 disabled:opacity-50"
                    >
                        {loading ? 'Deploying...' : 'Deploy Node'}
                    </button>
                </div>
            </form>
        </div>
    );
}
