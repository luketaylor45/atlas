import { useState, useEffect, useRef } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import api from '../../lib/api';
import {
    Play, Square, RefreshCcw, Skull,
    Terminal, Activity,
    ChevronRight, ArrowLeft, Settings, FolderClosed, Rocket,
    Globe, AlertTriangle, Layers
} from 'lucide-react';
import clsx from 'clsx';
import FileManager from './FileManager';

type Tab = 'console' | 'files' | 'startup' | 'settings';

interface LogLine {
    time: string;
    message: string;
}

// Helper to strip ANSI escape codes
const stripAnsi = (str: string) => {
    return str.replace(/[\u001b\u009b][[()#;?]*(?:[0-9]{1,4}(?:;[0-9]{0,4})*)?[0-9A-ORZcf-nqry=><]/g, '');
};

export default function ServiceConsolePage() {
    const { uuid } = useParams();
    const navigate = useNavigate();
    const [service, setService] = useState<any>(null);
    const [loading, setLoading] = useState(true);
    const [actionLoading, setActionLoading] = useState(false);
    const [logs, setLogs] = useState<LogLine[]>([]);
    const [stats, setStats] = useState<any>({ cpu: 0, memory: 0 });
    const [activeTab, setActiveTab] = useState<Tab>('console');
    const [showReinstallConfirm, setShowReinstallConfirm] = useState(false);
    const [wsVersion, setWsVersion] = useState(0); // Used to force WS reconnect
    const consoleRef = useRef<HTMLDivElement>(null);
    const wsRef = useRef<WebSocket | null>(null);

    const fetchDetails = async () => {
        try {
            const res = await api.get(`/services/${uuid}`);
            setService(res.data);
        } catch (err) {
            console.error("Failed to fetch service details", err);
        } finally {
            setLoading(false);
        }
    };

    // Initial fetch and status polling
    useEffect(() => {
        fetchDetails();
        const interval = setInterval(fetchDetails, 5000);
        return () => clearInterval(interval);
    }, [uuid]);

    // WebSocket Logic
    useEffect(() => {
        if (!service?.node || activeTab !== 'console' || service.status === 'installing') return;

        let reconnectTimer: any;
        let isStopped = false;

        const connect = () => {
            if (isStopped) return;

            const nodeAddr = service.node.address === 'localhost' || service.node.address === '127.0.0.1'
                ? `${window.location.hostname}:8081`
                : `${service.node.address}:${service.node.port}`;

            console.log(`[Atlas] Connecting to console: ws://${nodeAddr}/api/servers/${uuid}/console`);
            const ws = new WebSocket(`ws://${nodeAddr}/api/servers/${uuid}/console`);
            wsRef.current = ws;

            ws.onmessage = (event) => {
                const rawData = event.data;
                if (!rawData) return;

                setLogs(prev => {
                    let logTime = new Date().toLocaleTimeString([], { hour12: false, hour: '2-digit', minute: '2-digit', second: '2-digit' });
                    let logMsg = rawData;

                    // Docker timestamps: 2024-03-21T12:00:00.123456789Z message
                    const tsMatch = rawData.match(/^(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?Z)\s?(.*)$/);
                    if (tsMatch) {
                        const date = new Date(tsMatch[1]);
                        if (!isNaN(date.getTime())) {
                            logTime = date.toLocaleTimeString([], { hour12: false, hour: '2-digit', minute: '2-digit', second: '2-digit' });
                            logMsg = tsMatch[2];
                        }
                    }

                    // Strip ANSI codes from the message
                    logMsg = stripAnsi(logMsg);

                    const newLog = { time: logTime, message: logMsg };
                    return [...prev, newLog].slice(-500);
                });
            };

            ws.onclose = (event) => {
                if (!isStopped) {
                    console.log("[Atlas] Console stream disconnected. Retrying in 2s...");
                    reconnectTimer = setTimeout(connect, 2000);
                }
            };

            ws.onerror = (err) => {
                console.error("[Atlas] Console WS error", err);
                ws.close();
            };
        };

        connect();

        return () => {
            isStopped = true;
            clearTimeout(reconnectTimer);
            if (wsRef.current) {
                wsRef.current.close();
                wsRef.current = null;
            }
        };
    }, [uuid, service?.node?.address, service?.node?.port, activeTab, service?.status, wsVersion]);

    // Fetch stats polling
    useEffect(() => {
        if (!service || service.status !== 'running') {
            setStats({ cpu: 0, memory: 0 });
            return;
        }

        const fetchStats = async () => {
            const nodeAddr = service.node?.address === 'localhost' || service.node?.address === '127.0.0.1'
                ? `http://${window.location.hostname}:8081`
                : `http://${service.node?.address}:${service.node?.port}`;

            try {
                const res = await fetch(`${nodeAddr}/api/servers/${uuid}/stats`);
                if (res.ok) {
                    const data = await res.json();
                    setStats({
                        cpu: data.cpu,
                        memory: data.memory
                    });
                }
            } catch (err) { }
        };

        const interval = setInterval(fetchStats, 2000);
        return () => clearInterval(interval);
    }, [service?.status, uuid, service?.node?.address, service?.node?.port]);

    // Internal logs for installation or offline
    useEffect(() => {
        if (service?.status === 'installing' && logs.length === 0) {
            setLogs([{
                time: new Date().toLocaleTimeString(),
                message: "[Atlas] Initializing installation sequence..."
            }]);
        }
    }, [service?.status]);

    // Auto-scroll logic
    useEffect(() => {
        if (consoleRef.current) {
            consoleRef.current.scrollTop = consoleRef.current.scrollHeight;
        }
    }, [logs, activeTab, service?.status]);

    const handlePowerAction = async (action: string) => {
        setActionLoading(true);

        // Immediate Visual Feedback
        const time = new Date().toLocaleTimeString([], { hour12: false, hour: '2-digit', minute: '2-digit', second: '2-digit' });
        const actionLabels: Record<string, string> = {
            'start': 'STARTING SERVER...',
            'stop': 'STOPPING SERVER...',
            'restart': 'RESTARTING SERVER...',
            'kill': 'TERMINATING SERVER PROCESS...'
        };
        setLogs(prev => [...prev, { time, message: `[Atlas] ${actionLabels[action] || action.toUpperCase()}` }]);

        // Ensure the new log line is scrolled into view immediately
        setTimeout(() => {
            if (consoleRef.current) consoleRef.current.scrollTop = consoleRef.current.scrollHeight;
        }, 50);

        try {
            await api.post(`/services/${uuid}/power`, { action });
            // Small delay then trigger WS reconnect
            setTimeout(() => setWsVersion(v => v + 1), 500);
        } catch (err) {
            setLogs(prev => [...prev, { time, message: `[Error] Failed to ${action} server.` }]);
        } finally {
            setActionLoading(false);
            fetchDetails();
        }
    };

    const handleSendCommand = async (command: string) => {
        if (!command) return;
        try {
            await api.post(`/services/${uuid}/command`, { command });
        } catch (err) {
            const time = new Date().toLocaleTimeString([], { hour12: false, hour: '2-digit', minute: '2-digit', second: '2-digit' });
            setLogs(prev => [...prev, { time, message: `[Error] Failed to send command: ${command}` }]);
        }
    };

    const handleReinstall = async () => {
        setActionLoading(true);
        setShowReinstallConfirm(false);
        try {
            await api.post(`/services/${uuid}/reinstall`);
            setLogs([]);
        } catch (err) {
            console.error("Failed to trigger reinstall", err);
        } finally {
            setActionLoading(false);
            fetchDetails();
        }
    };

    const handleUpdateEnvironment = async (env: any) => {
        try {
            await api.post(`/services/${uuid}/environment`, {
                environment: JSON.stringify(env)
            });
            setService({ ...service, environment: JSON.stringify(env) });
        } catch (err) {
            console.error("Failed to update environment", err);
        }
    };

    if (loading) return <div className="p-12 text-center text-muted animate-pulse font-bold tracking-widest mt-20">Connecting...</div>;
    if (!service) return <div className="p-12 text-center text-red-500 font-bold">Server not found.</div>;

    const envData = service.environment ? JSON.parse(service.environment) : (service.egg?.environment ? JSON.parse(service.egg.environment) : {});

    // Helper for status formatting
    const getStatusInfo = (status: string) => {
        const s = status?.toLowerCase() || 'unknown';
        if (s === 'running') return { label: 'Connected', color: 'bg-emerald-500', pill: 'bg-emerald-500/10 text-emerald-500 border-emerald-500/20' };
        if (s === 'starting') return { label: 'Starting', color: 'bg-blue-500', pill: 'bg-blue-500/10 text-blue-500 border-blue-500/20' };
        if (s === 'stopping') return { label: 'Stopping', color: 'bg-amber-500', pill: 'bg-amber-500/10 text-amber-500 border-amber-500/20' };
        if (s === 'installing') return { label: 'Installing', color: 'bg-primary', pill: 'bg-primary/10 text-primary border-primary/20' };
        if (s === 'offline' || s === '' || s === 'unknown') return { label: 'Offline', color: 'bg-zinc-500', pill: 'bg-zinc-500/10 text-zinc-500 border-zinc-500/20' };
        return { label: 'Error', color: 'bg-red-500', pill: 'bg-red-500/10 text-red-500 border-red-500/20' };
    };

    const statusInfo = getStatusInfo(service.status);

    return (
        <div className="space-y-8 max-w-7xl mx-auto animation-enter relative">
            {/* Installation Overlay */}
            {service.status === 'installing' && (
                <div className="fixed inset-0 z-[100] bg-background flex flex-col items-center justify-center p-8 animate-in text-foreground">
                    <div className="max-w-4xl w-full space-y-12">
                        <div className="text-center space-y-4">
                            <h2 className="text-4xl font-bold tracking-tight">Installing Server</h2>
                            <p className="text-muted font-medium">
                                Setting up "{service.name}" on {service.node?.name}.
                            </p>
                        </div>

                        {/* Progress Tracker */}
                        <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
                            <div className={clsx("panel-card border-border p-6 transition-all", service.installation_progress >= 10 ? "shadow-md ring-1 ring-primary/20" : "opacity-50")}>
                                <div className="flex items-center gap-3 mb-4">
                                    <div className={clsx("w-8 h-8 rounded-lg flex items-center justify-center font-bold", service.installation_progress >= 10 ? "bg-primary text-white" : "bg-secondary text-muted")}>1</div>
                                    <span className="font-bold text-sm uppercase">Bootstrap</span>
                                </div>
                                <p className="text-xs text-muted font-medium">Preparing the environment and checking for dependencies.</p>
                            </div>
                            <div className={clsx("panel-card border-border p-6 transition-all", service.installation_progress >= 30 ? "shadow-md ring-1 ring-primary/20" : "opacity-50")}>
                                <div className="flex items-center gap-3 mb-4">
                                    <div className={clsx("w-8 h-8 rounded-lg flex items-center justify-center font-bold", service.installation_progress >= 30 ? "bg-primary text-white" : "bg-secondary text-muted")}>2</div>
                                    <span className="font-bold text-sm uppercase">Downloads</span>
                                </div>
                                <p className="text-xs text-muted font-medium">Downloading required application files and assets.</p>
                            </div>
                            <div className={clsx("panel-card border-border p-6 transition-all", service.installation_progress >= 90 ? "shadow-md ring-1 ring-primary/20" : "opacity-50")}>
                                <div className="flex items-center gap-3 mb-4">
                                    <div className={clsx("w-8 h-8 rounded-lg flex items-center justify-center font-bold", service.installation_progress >= 90 ? "bg-primary text-white" : "bg-secondary text-muted")}>3</div>
                                    <span className="font-bold text-sm uppercase">Finalizing</span>
                                </div>
                                <p className="text-xs text-muted font-medium">Verifying files and preparing the server for its first boot.</p>
                            </div>
                        </div>

                        {/* Console Logs */}
                        <div className="panel-card !p-0 bg-black border-border overflow-hidden h-[300px]">
                            <div className="px-4 py-2 bg-zinc-900 border-b border-zinc-800 flex items-center justify-between">
                                <span className="text-[10px] font-bold uppercase tracking-widest text-zinc-500">Installation Logs</span>
                                <div className="flex items-center gap-2">
                                    <span className="text-[10px] text-zinc-600 uppercase font-mono">{service.installation_stage || 'Waiting...'}</span>
                                    <div className="w-1.5 h-1.5 rounded-full bg-primary animate-pulse" />
                                </div>
                            </div>
                            <div ref={consoleRef} className="p-6 font-mono text-xs overflow-y-auto h-[calc(100%-35px)] space-y-1 text-zinc-400">
                                {logs.map((log, i) => (
                                    <div key={i} className="flex gap-4">
                                        <span className="text-[10px] text-zinc-700 w-8 shrink-0">{i + 1}</span>
                                        <span className="break-all">{log.message}</span>
                                    </div>
                                ))}
                            </div>
                        </div>

                        {/* Progress Bar */}
                        <div className="space-y-4">
                            <div className="flex justify-between text-sm font-bold">
                                <span className="text-muted uppercase tracking-widest">Global Progress</span>
                                <span className="text-primary">{service.installation_progress}%</span>
                            </div>
                            <div className="w-full h-2 bg-secondary rounded-full overflow-hidden">
                                <div
                                    className="h-full bg-primary transition-all duration-1000"
                                    style={{ width: `${service.installation_progress}%` }}
                                />
                            </div>
                        </div>
                    </div>
                </div>
            )}

            {/* Reinstall modal */}
            {showReinstallConfirm && (
                <div className="fixed inset-0 z-[110] flex items-center justify-center p-6 bg-black/60 backdrop-blur-sm animate-in">
                    <div className="panel-card max-w-md w-full border-red-500/30 shadow-xl">
                        <div className="flex items-center gap-4 text-red-500 mb-6">
                            <AlertTriangle size={36} />
                            <h2 className="text-2xl font-bold tracking-tight">Confirm Reinstall</h2>
                        </div>
                        <p className="text-muted font-medium text-sm mb-8">
                            Are you sure you want to reinstall "{service.name}"? All files and data will be permanently deleted.
                        </p>
                        <div className="flex gap-3">
                            <button
                                onClick={() => setShowReinstallConfirm(false)}
                                className="flex-1 px-6 py-3 rounded-xl font-bold bg-secondary hover:bg-secondary/80 transition-all text-sm"
                            >
                                Cancel
                            </button>
                            <button
                                onClick={handleReinstall}
                                className="flex-1 px-6 py-3 rounded-xl font-bold bg-red-600 text-white hover:bg-red-700 transition-all text-sm shadow-lg shadow-red-600/20"
                            >
                                Reinstall
                            </button>
                        </div>
                    </div>
                </div>
            )}

            {/* Main Header */}
            <div className="flex flex-col md:flex-row md:items-center justify-between gap-6 mb-8">
                <div className="flex items-center gap-5">
                    <button
                        onClick={() => navigate('/services')}
                        className="p-3 hover:bg-secondary rounded-xl text-muted transition-all border border-border"
                    >
                        <ArrowLeft size={20} />
                    </button>
                    <div>
                        <div className="flex items-center gap-3">
                            <h1 className="text-3xl font-bold tracking-tight">{service.name}</h1>
                            <div className={clsx(
                                "px-3 py-1 rounded-full text-[10px] font-bold uppercase tracking-wider border flex items-center gap-2",
                                statusInfo.pill
                            )}>
                                <div className={clsx("w-1.5 h-1.5 rounded-full", statusInfo.color)} />
                                {statusInfo.label}
                            </div>
                        </div>
                        <div className="flex items-center gap-4 text-xs font-semibold text-muted mt-2 uppercase tracking-tight">
                            <span className="flex items-center gap-1.5 uppercase font-bold text-[10px] tracking-widest"><Globe size={14} /> {service.node?.address}</span>
                            <span className="flex items-center gap-1.5 uppercase font-bold text-[10px] tracking-widest"><Layers size={14} /> {service.egg?.name}</span>
                        </div>
                    </div>
                </div>

                <div className="flex items-center gap-2 p-1.5 bg-secondary/50 rounded-2xl border border-border w-fit">
                    <button
                        onClick={() => handlePowerAction('start')}
                        disabled={actionLoading || service.status === 'running' || service.status === 'starting' || service.status === 'installing'}
                        className="flex items-center gap-2 px-4 py-2.5 rounded-xl hover:bg-white text-emerald-500 transition-all disabled:opacity-30 text-xs font-bold uppercase tracking-wider"
                    >
                        <Play size={18} fill="currentColor" />
                        <span>Start</span>
                    </button>
                    <button
                        onClick={() => handlePowerAction('restart')}
                        disabled={actionLoading || service.status === 'installing'}
                        className="flex items-center gap-2 px-4 py-2.5 rounded-xl hover:bg-white text-blue-500 transition-all disabled:opacity-30 text-xs font-bold uppercase tracking-wider"
                    >
                        <RefreshCcw size={18} />
                        <span>Restart</span>
                    </button>
                    <button
                        onClick={() => handlePowerAction('stop')}
                        disabled={actionLoading || service.status === 'offline' || service.status === 'stopping' || service.status === 'installing'}
                        className="flex items-center gap-2 px-4 py-2.5 rounded-xl hover:bg-white text-amber-500 transition-all disabled:opacity-30 text-xs font-bold uppercase tracking-wider"
                    >
                        <Square size={18} fill="currentColor" />
                        <span>Stop</span>
                    </button>
                    <button
                        onClick={() => handlePowerAction('kill')}
                        disabled={actionLoading || service.status === 'installing'}
                        className="flex items-center gap-2 px-4 py-2.5 rounded-xl hover:bg-white text-red-500 transition-all disabled:opacity-30 text-xs font-bold uppercase tracking-wider"
                    >
                        <Skull size={18} />
                        <span>Kill</span>
                    </button>
                </div>
            </div>

            {/* Navigation */}
            <div className="flex items-center gap-1 border-b border-border mb-8 overflow-x-auto no-scrollbar">
                {[
                    { id: 'console', label: 'Console', icon: Terminal },
                    { id: 'files', label: 'File Manager', icon: FolderClosed },
                    { id: 'startup', label: 'Startup', icon: Rocket },
                    { id: 'settings', label: 'Settings', icon: Settings },
                ].map(tab => (
                    <button
                        key={tab.id}
                        onClick={() => setActiveTab(tab.id as Tab)}
                        className={clsx(
                            "flex items-center gap-2.5 px-6 py-4 text-sm font-bold transition-all relative",
                            activeTab === tab.id ? "text-primary" : "text-muted hover:text-foreground"
                        )}
                    >
                        <tab.icon size={18} className={clsx(activeTab === tab.id ? "text-primary" : "text-muted opacity-50")} />
                        {tab.label}
                        {activeTab === tab.id && (
                            <div className="absolute bottom-0 left-0 right-0 h-0.5 bg-primary" />
                        )}
                    </button>
                ))}
            </div>

            <div className="grid grid-cols-1 lg:grid-cols-4 gap-8">
                <div className="lg:col-span-3">
                    {activeTab === 'console' && (
                        <div className="flex flex-col h-[600px] border border-zinc-800 bg-zinc-950 rounded-2xl overflow-hidden shadow-2xl">
                            <div className="px-6 py-3 bg-zinc-900/50 border-b border-zinc-800 flex items-center justify-between">
                                <span className="text-[10px] font-black uppercase tracking-widest text-zinc-500">Terminal Output</span>
                                <div className="flex items-center gap-2">
                                    <div className={clsx("w-1.5 h-1.5 rounded-full", statusInfo.color)} />
                                    <span className="text-[10px] text-zinc-600 uppercase font-bold tracking-widest">
                                        {statusInfo.label}
                                    </span>
                                </div>
                            </div>
                            <div
                                ref={consoleRef}
                                className="flex-1 p-6 font-mono text-sm overflow-y-auto space-y-1 selection:bg-primary/30 custom-scrollbar scroll-smooth"
                                style={{ scrollBehavior: 'auto' }}
                            >
                                {logs.map((log, i) => (
                                    <div key={i} className="flex gap-4 group/line">
                                        <span className="text-[10px] text-zinc-700 font-mono w-14 shrink-0 pt-0.5 text-right select-none">
                                            {log.time}
                                        </span>
                                        <span className={clsx(
                                            "leading-relaxed break-all",
                                            log.message.includes('Error') || log.message.includes('failed') ? "text-red-400" :
                                                log.message.includes('[Atlas]') ? "text-primary font-bold" :
                                                    log.message.startsWith('>') ? "text-zinc-500" : "text-zinc-300"
                                        )}>
                                            {log.message}
                                        </span>
                                    </div>
                                ))}
                                {logs.length === 0 && (
                                    <div className="flex flex-col items-center justify-center h-full text-zinc-600 space-y-3">
                                        <Terminal size={40} className="opacity-20" />
                                        <p className="font-mono italic text-sm text-center px-8">
                                            {service.status === 'offline' ? '[Atlas] Terminal offline. Waiting for stream...' : 'Awaiting output...'}
                                        </p>
                                    </div>
                                )}
                            </div>
                            <div className="p-4 bg-zinc-900/50 border-t border-zinc-800 flex items-center gap-4">
                                <ChevronRight size={18} className="text-primary" />
                                <input
                                    type="text"
                                    placeholder="Type a command..."
                                    className="bg-transparent border-none outline-none flex-1 text-sm font-mono text-zinc-200"
                                    onKeyDown={(e) => {
                                        if (e.key === 'Enter') {
                                            const input = (e.target as HTMLInputElement).value;
                                            if (!input) return;
                                            setLogs(prev => [...prev, { time: new Date().toLocaleTimeString([], { hour12: false, hour: '2-digit', minute: '2-digit', second: '2-digit' }), message: `> ${input}` }]);
                                            handleSendCommand(input);
                                            (e.target as HTMLInputElement).value = '';
                                        }
                                    }}
                                />
                            </div>
                        </div>
                    )}

                    {activeTab === 'files' && (
                        <FileManager />
                    )}

                    {activeTab === 'startup' && (
                        <div className="space-y-6">
                            <div className="panel-card p-8 border-border">
                                <h3 className="text-lg font-bold mb-4">Startup Configuration</h3>
                                <div className="p-4 bg-secondary rounded-xl font-mono text-xs text-muted break-all border border-border">
                                    {service.egg?.startup_command}
                                </div>
                            </div>
                            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                                {Object.keys(envData).map(key => (
                                    <div key={key} className="panel-card p-6 border-border">
                                        <label className="text-[10px] font-bold text-muted uppercase tracking-widest block mb-4">{key}</label>
                                        <input
                                            type="text"
                                            defaultValue={envData[key]}
                                            onBlur={(e) => {
                                                const newVal = e.target.value;
                                                const newEnv = { ...envData, [key]: newVal };
                                                handleUpdateEnvironment(newEnv);
                                            }}
                                            className="input-field"
                                        />
                                    </div>
                                ))}
                            </div>
                        </div>
                    )}

                    {activeTab === 'settings' && (
                        <div className="panel-card p-8 border-red-500/20">
                            <h3 className="text-xl font-bold text-red-500 mb-6 font-bold tracking-tight">Danger Zone</h3>
                            <div className="flex items-center justify-between p-6 bg-transparent border border-red-500/20 rounded-2xl">
                                <div>
                                    <h4 className="font-bold">Reinstall Server</h4>
                                    <p className="text-xs text-muted mt-1 font-medium">This will wipe all existing files and start the installation process again.</p>
                                </div>
                                <button
                                    onClick={() => setShowReinstallConfirm(true)}
                                    className="px-6 py-2 rounded-xl bg-red-600 text-white font-bold text-xs hover:bg-red-700 transition-all shadow-md"
                                >
                                    Reinstall
                                </button>
                            </div>
                        </div>
                    )}
                </div>

                <div className="space-y-6">
                    <div className="panel-card p-6 bg-primary/5 border-primary/20">
                        <h3 className="text-xs font-bold text-primary uppercase tracking-widest mb-6 flex items-center gap-2">
                            <Activity size={16} /> Current Usage
                        </h3>
                        <div className="space-y-8">
                            <div className="space-y-2">
                                <div className="flex justify-between text-[11px] font-bold uppercase tracking-tight">
                                    <span className="text-muted">CPU</span>
                                    <span>{stats.cpu}%</span>
                                </div>
                                <div className="h-1.5 bg-secondary rounded-full overflow-hidden">
                                    <div className="h-full bg-primary transition-all duration-1000" style={{ width: `${Math.min(stats.cpu, 100)}%` }} />
                                </div>
                            </div>
                            <div className="space-y-2">
                                <div className="flex justify-between text-[11px] font-bold uppercase tracking-tight">
                                    <span className="text-muted">Memory</span>
                                    <span>{stats.memory}MB / {service.memory}MB</span>
                                </div>
                                <div className="h-1.5 bg-secondary rounded-full overflow-hidden">
                                    <div className="h-full bg-primary transition-all duration-1000" style={{ width: `${Math.min((stats.memory / service.memory) * 100, 100)}%` }} />
                                </div>
                            </div>
                        </div>
                    </div>

                    <div className="panel-card p-6 border-border">
                        <h3 className="text-xs font-bold text-muted uppercase tracking-widest mb-4">Location Details</h3>
                        <div className="space-y-4">
                            <div className="flex justify-between">
                                <span className="text-[10px] text-muted font-bold uppercase">Node</span>
                                <span className="text-xs font-bold uppercase">NODE-{service.node?.id}</span>
                            </div>
                            <div className="flex justify-between">
                                <span className="text-[10px] text-muted font-bold uppercase">Public IP</span>
                                <span className="text-xs font-bold">{service.node?.address || 'N/A'}</span>
                            </div>
                            <div className="flex justify-between">
                                <span className="text-[10px] text-muted font-bold uppercase">Network Port</span>
                                <span className="text-xs font-bold">{service.port}</span>
                            </div>
                        </div>
                    </div>
                </div>
            </div>
        </div>
    );
}
