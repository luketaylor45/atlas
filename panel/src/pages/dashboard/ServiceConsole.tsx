import { useState, useEffect, useEffectEvent, useRef } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import api from '../../lib/api';
import {
    Play, Square, RefreshCcw, Skull,
    Terminal, Activity, Save,
    ChevronRight, ArrowLeft, Settings, FolderClosed, Rocket,
    Globe, AlertTriangle, Layers, Users, Clock, ArrowDown, ArrowUp, CheckCircle2
} from 'lucide-react';
import clsx from 'clsx';
import FileManager from './FileManager';
import ServiceUsersTab from './ServiceUsersTab';
import ServiceActivityTab from './ServiceActivityTab';

type Tab = 'console' | 'files' | 'startup' | 'settings' | 'users' | 'activity';

interface LogLine {
    id: string;
    time: string;
    message: string;
}

let nextLocalLogID = 0;

const makeLogLine = (message: string, time = new Date().toLocaleTimeString([], {
    hour12: false,
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit'
})): LogLine => ({
    id: `local-${Date.now()}-${nextLocalLogID++}`,
    time,
    message
});

// Helper to strip ANSI escape codes
const stripAnsi = (str: string) => {
    return str.replace(/[\u001b\u009b][[()#;?]*(?:[0-9]{1,4}(?:;[0-9]{0,4})*)?[0-9A-ORZcf-nqry=><]/g, '');
};

const requestErrorMessage = (error: unknown, fallback: string) => {
    const responseMessage = (error as { response?: { data?: { error?: unknown } } })
        ?.response?.data?.error;
    if (typeof responseMessage === 'string' && responseMessage.trim()) {
        return responseMessage.trim();
    }
    if (error instanceof Error && error.message.trim()) {
        return `${fallback} ${error.message.trim()}`;
    }
    return fallback;
};

const isContainerUnavailableStatus = (status?: string) => (
    status === 'installing' || status === 'installation_failed'
);

export default function ServiceConsolePage() {
    const { uuid } = useParams();
    const navigate = useNavigate();
    const [service, setService] = useState<any>(null);
    const [permissions, setPermissions] = useState<any>(null);
    const [isOwner, setIsOwner] = useState(true);
    const [loading, setLoading] = useState(true);
    const [actionLoading, setActionLoading] = useState(false);
    const [logs, setLogs] = useState<LogLine[]>([]);
    const [stats, setStats] = useState<any>({ cpu: 0, memory: 0, network: { rx: 0, tx: 0 } });
    const [activeTab, setActiveTab] = useState<Tab>('console');
    const [showReinstallConfirm, setShowReinstallConfirm] = useState(false);
    const [wsVersion, setWsVersion] = useState(0); // Used to force WS reconnect
    const [startupCommand, setStartupCommand] = useState('');
    const [startupEnv, setStartupEnv] = useState<Record<string, string>>({});
    const [startupSaving, setStartupSaving] = useState(false);
    const [startupDirty, setStartupDirty] = useState(false);
    const [startupMessage, setStartupMessage] = useState('');
    const [startupMessageType, setStartupMessageType] = useState<'idle' | 'info' | 'success' | 'error'>('idle');
    const [lastStartupSaveAt, setLastStartupSaveAt] = useState<string | null>(null);
    const [followOutput, setFollowOutput] = useState(true);
    const consoleRef = useRef<HTMLDivElement>(null);
    const installationLogRef = useRef<HTMLDivElement>(null);
    const wsRef = useRef<WebSocket | null>(null);
    const lastInstallationStageRef = useRef('');

    const fetchDetails = async () => {
        try {
            const res = await api.get(`/services/${uuid}`);
            // New structure: { service: {}, permissions: {}, is_owner: true/false }
            setService(res.data.service);
            setPermissions(res.data.permissions || null);
            setIsOwner(res.data.is_owner);
        } catch (err) {
            console.error("Failed to fetch service details", err);
        } finally {
            setLoading(false);
        }
    };
    const pollDetails = useEffectEvent(fetchDetails);

    // Initial fetch and status polling
    useEffect(() => {
        pollDetails();
        const interval = setInterval(pollDetails, service?.status === 'installing' ? 1000 : 5000);
        return () => clearInterval(interval);
    }, [uuid, service?.status]);

    useEffect(() => {
        setStartupDirty(false);
        setStartupMessage('');
        setStartupMessageType('idle');
        setLastStartupSaveAt(null);
    }, [uuid]);

    const canViewConsole = isOwner || Boolean(permissions?.can_view_console);

    // Console stream. The socket is relayed by Core and authorised with a
    // short-lived ticket, so the browser never talks to a node directly.
    useEffect(() => {
        if (!service?.uuid || !canViewConsole || activeTab !== 'console' || isContainerUnavailableStatus(service.status)) return;

        let reconnectTimer: ReturnType<typeof setTimeout> | undefined;
        let isStopped = false;
        let attempt = 0;
        const shouldReconnect = ['running', 'starting', 'stopping'].includes(service.status);

        const scheduleReconnect = () => {
            if (isStopped || !shouldReconnect) return;
            attempt += 1;
            reconnectTimer = setTimeout(connect, Math.min(1000 * 2 ** attempt, 15000));
        };

        const connect = async () => {
            if (isStopped) return;

            let ticket = '';
            try {
                const res = await api.post(`/services/${uuid}/console/ticket`);
                ticket = res.data.ticket;
            } catch {
                scheduleReconnect();
                return;
            }
            if (isStopped) return;

            const wsBase = String(api.defaults.baseURL || '').replace(/^http/i, 'ws');
            const ws = new WebSocket(`${wsBase}/services/${uuid}/console/ws?ticket=${encodeURIComponent(ticket)}`);
            wsRef.current = ws;

            ws.onopen = () => { attempt = 0; };

            ws.onmessage = (event) => {
                const rawData = String(event.data || '');
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

                    // A reconnect replays recent history; timestamped lines are
                    // unique, so they are used to drop the replayed copies.
                    const newLog = { id: tsMatch ? `docker-${rawData}` : makeLogLine('').id, time: logTime, message: logMsg };
                    if (tsMatch && prev.some(log => log.id === newLog.id)) {
                        return prev;
                    }
                    return [...prev, newLog].slice(-500);
                });
            };

            ws.onclose = () => {
                if (wsRef.current === ws) wsRef.current = null;
                scheduleReconnect();
            };

            ws.onerror = () => {
                ws.close();
            };
        };

        void connect();

        return () => {
            isStopped = true;
            clearTimeout(reconnectTimer);
            if (wsRef.current) {
                wsRef.current.close();
                wsRef.current = null;
            }
        };
    }, [uuid, service?.uuid, canViewConsole, activeTab, service?.status, wsVersion]);

    // Logs belong to one service; never carry them over to another.
    useEffect(() => {
        setLogs([]);
    }, [uuid]);

    // Live resource usage, proxied by Core.
    useEffect(() => {
        if (!service || service.status !== 'running' || !canViewConsole) {
            setStats({ cpu: 0, memory: 0, network: { rx: 0, tx: 0 } });
            return;
        }

        let cancelled = false;
        const fetchStats = async () => {
            if (document.hidden) return;
            try {
                const res = await api.get(`/services/${uuid}/stats`);
                if (cancelled) return;
                setStats({
                    cpu: res.data.cpu,
                    memory: res.data.memory,
                    network: res.data.network || { rx: 0, tx: 0 }
                });
            } catch {
                // Usage is best-effort; the next poll will try again.
            }
        };

        void fetchStats();
        const interval = setInterval(fetchStats, 3000);
        return () => {
            cancelled = true;
            clearInterval(interval);
        };
    }, [service?.status, uuid, canViewConsole]);

    // Internal logs for installation or offline
    useEffect(() => {
        if (isContainerUnavailableStatus(service?.status) && logs.length === 0) {
            const initialMessage = service.status === 'installation_failed'
                ? `[Atlas] ${service.installation_stage || 'Installation failed before the game container was created.'}`
                : "[Atlas] Initializing installation sequence...";
            setLogs([makeLogLine(initialMessage)]);
        }
    }, [service?.status, service?.installation_stage, logs.length]);

    useEffect(() => {
        if (!isContainerUnavailableStatus(service?.status)) {
            lastInstallationStageRef.current = '';
            return;
        }

        const stage = service.installation_stage?.trim();
        if (!stage || stage === lastInstallationStageRef.current) return;

        lastInstallationStageRef.current = stage;
        setLogs(prev => [...prev, makeLogLine(`[Atlas] ${stage}`)].slice(-500));
    }, [service?.status, service?.installation_stage]);

    useEffect(() => {
        if (isContainerUnavailableStatus(service?.status) && installationLogRef.current) {
            installationLogRef.current.scrollTop = installationLogRef.current.scrollHeight;
        }
    }, [logs, service?.status]);

    useEffect(() => {
        const consoleElement = consoleRef.current;
        if (!consoleElement || activeTab !== 'console' || !followOutput) return;

        const selection = window.getSelection();
        if (selection && !selection.isCollapsed && selection.anchorNode && consoleElement.contains(selection.anchorNode)) {
            setFollowOutput(false);
            return;
        }

        consoleElement.scrollTop = consoleElement.scrollHeight;
    }, [logs, activeTab, followOutput]);

    useEffect(() => {
        if (!service || startupDirty) return;

        const nextEnv = service.environment
            ? JSON.parse(service.environment)
            : (service.egg?.environment ? JSON.parse(service.egg.environment) : {});

        setStartupCommand(service.startup_command || service.egg?.startup_command || '');
        setStartupEnv(nextEnv);
    }, [service, startupDirty]);

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
        setFollowOutput(true);
        setLogs(prev => [...prev, makeLogLine(`[Atlas] ${actionLabels[action] || action.toUpperCase()}`, time)]);

        try {
            await api.post(`/services/${uuid}/power`, { action });
            // Small delay then trigger WS reconnect
            setTimeout(() => setWsVersion(v => v + 1), 500);
        } catch (err) {
            const message = requestErrorMessage(err, `Failed to ${action} server.`);
            setLogs(prev => [...prev, makeLogLine(`[Error] ${message}`, time)]);
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
            setLogs(prev => [...prev, makeLogLine(`[Error] Failed to send command: ${command}`, time)]);
        }
    };

    const handleReinstall = async () => {
        setActionLoading(true);
        setShowReinstallConfirm(false);
        try {
            await api.post(`/services/${uuid}/reinstall`);
            setLogs([]);
            setService((current: any) => current ? {
                ...current,
                status: 'installing',
                installation_stage: 'Reinstallation queued',
                installation_progress: 0
            } : current);
        } catch (err) {
            console.error("Failed to trigger reinstall", err);
        } finally {
            setActionLoading(false);
            fetchDetails();
        }
    };

    const handleSaveStartupConfiguration = async () => {
        setStartupSaving(true);
        setStartupMessage('Saving startup configuration...');
        setStartupMessageType('info');
        try {
            const res = await api.post(`/services/${uuid}/environment`, {
                startup_command: startupCommand,
                environment: JSON.stringify(startupEnv)
            });
            setService({
                ...service,
                startup_command: res.data.startup_command ?? startupCommand,
                environment: res.data.environment ?? JSON.stringify(startupEnv)
            });
            setStartupDirty(false);
            setStartupMessage('Startup configuration saved to Atlas.');
            setStartupMessageType('success');
            setLastStartupSaveAt(new Date().toLocaleTimeString([], { hour12: false, hour: '2-digit', minute: '2-digit', second: '2-digit' }));
            fetchDetails();
        } catch (err) {
            console.error("Failed to update startup configuration", err);
            const errorMessage = (err as any)?.response?.data?.error || 'Failed to save startup configuration.';
            setStartupMessage(errorMessage);
            setStartupMessageType('error');
        } finally {
            setStartupSaving(false);
        }
    };

    if (loading) return <div className="p-12 text-center text-muted animate-pulse font-bold tracking-widest mt-20">Connecting...</div>;
    if (!service) return <div className="p-12 text-center text-red-500 font-bold">Server not found.</div>;

    // Helper for status formatting
    const getStatusInfo = (status: string) => {
        const s = status?.toLowerCase() || 'unknown';
        if (s === 'running') return { label: 'Connected', color: 'bg-emerald-500', pill: 'bg-emerald-500/10 text-emerald-500 border-emerald-500/20' };
        if (s === 'starting') return { label: 'Starting', color: 'bg-blue-500', pill: 'bg-blue-500/10 text-blue-500 border-blue-500/20' };
        if (s === 'stopping') return { label: 'Stopping', color: 'bg-amber-500', pill: 'bg-amber-500/10 text-amber-500 border-amber-500/20' };
        if (s === 'installing') return { label: 'Installing', color: 'bg-primary', pill: 'bg-primary/10 text-primary border-primary/20' };
        if (s === 'installation_failed') return { label: 'Install Failed', color: 'bg-red-500', pill: 'bg-red-500/10 text-red-500 border-red-500/20' };
        if (s === 'offline' || s === '' || s === 'unknown') return { label: 'Offline', color: 'bg-zinc-500', pill: 'bg-zinc-500/10 text-zinc-500 border-zinc-500/20' };
        return { label: 'Error', color: 'bg-red-500', pill: 'bg-red-500/10 text-red-500 border-red-500/20' };
    };

    const statusInfo = getStatusInfo(service.status);
    const installationFailed = service.status === 'installation_failed';
    const installationInProgress = service.status === 'installing';

    return (
        <div className="space-y-8 max-w-7xl mx-auto animation-enter relative">
            {/* Installation Overlay */}
            {(installationInProgress || installationFailed) && (
                <div className="fixed inset-0 z-[100] bg-background flex flex-col items-center overflow-y-auto p-8 animate-in text-foreground">
                    <div className="max-w-4xl w-full space-y-12 my-auto py-8">
                        <div className="text-center space-y-4">
                            <h2 className={clsx("text-4xl font-bold tracking-tight", installationFailed && "text-red-500")}>
                                {installationFailed ? 'Installation Failed' : 'Installing Server'}
                            </h2>
                            <p className="text-muted font-medium">
                                {installationFailed
                                    ? `The game container for "${service.name}" was not created.`
                                    : `Setting up "${service.name}" on ${service.node?.name}.`}
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
                                <span className={clsx("text-[10px] font-bold uppercase tracking-widest", installationFailed ? "text-red-400" : "text-zinc-500")}>
                                    {installationFailed ? 'Installer Failure' : 'Installation Logs'}
                                </span>
                                <div className="flex items-center gap-2">
                                    <span className={clsx("text-[10px] uppercase font-mono", installationFailed ? "text-red-400" : "text-zinc-600")}>
                                        {service.installation_stage || 'Waiting...'}
                                    </span>
                                    <div className={clsx("w-1.5 h-1.5 rounded-full", installationFailed ? "bg-red-500" : "bg-primary animate-pulse")} />
                                </div>
                            </div>
                            <div ref={installationLogRef} className="p-6 font-mono text-xs overflow-y-auto h-[calc(100%-35px)] space-y-1 text-zinc-400">
                                {logs.map((log, i) => (
                                    <div key={log.id} className="flex gap-4">
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
                                    className={clsx("h-full transition-all duration-1000", installationFailed ? "bg-red-500" : "bg-primary")}
                                    style={{ width: `${service.installation_progress}%` }}
                                />
                            </div>
                        </div>
                        {installationFailed && (
                            <div className="panel-card border-red-500/30 bg-red-500/5 p-6 flex flex-col md:flex-row md:items-center justify-between gap-5">
                                <div>
                                    <div className="text-xs font-bold uppercase tracking-widest text-red-400 mb-2">Recorded failure</div>
                                    <p className="text-sm font-mono text-red-200 break-all">
                                        {service.installation_stage || 'The installer exited before Atlas could create the game container.'}
                                    </p>
                                </div>
                                <button
                                    onClick={() => setShowReinstallConfirm(true)}
                                    disabled={actionLoading || (!isOwner && !permissions?.can_control_power)}
                                    className="px-6 py-3 rounded-xl bg-red-600 text-white font-bold text-xs uppercase tracking-wider hover:bg-red-700 transition-colors disabled:opacity-40 shrink-0"
                                >
                                    Retry installation
                                </button>
                            </div>
                        )}
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
                        disabled={actionLoading || service.status === 'running' || service.status === 'starting' || isContainerUnavailableStatus(service.status) || (!isOwner && !permissions?.can_control_power)}
                        className="flex items-center gap-2 px-4 py-2.5 rounded-xl hover:bg-secondary text-emerald-500 transition-all disabled:opacity-30 text-xs font-bold uppercase tracking-wider"
                    >
                        <Play size={18} fill="currentColor" />
                        <span>Start</span>
                    </button>
                    <button
                        onClick={() => handlePowerAction('restart')}
                        disabled={actionLoading || isContainerUnavailableStatus(service.status) || (!isOwner && !permissions?.can_control_power)}
                        className="flex items-center gap-2 px-4 py-2.5 rounded-xl hover:bg-secondary text-blue-500 transition-all disabled:opacity-30 text-xs font-bold uppercase tracking-wider"
                    >
                        <RefreshCcw size={18} />
                        <span>Restart</span>
                    </button>
                    <button
                        onClick={() => handlePowerAction('stop')}
                        disabled={actionLoading || service.status === 'offline' || service.status === 'stopping' || isContainerUnavailableStatus(service.status) || (!isOwner && !permissions?.can_control_power)}
                        className="flex items-center gap-2 px-4 py-2.5 rounded-xl hover:bg-secondary text-amber-500 transition-all disabled:opacity-30 text-xs font-bold uppercase tracking-wider"
                    >
                        <Square size={18} fill="currentColor" />
                        <span>Stop</span>
                    </button>
                    <button
                        onClick={() => handlePowerAction('kill')}
                        disabled={actionLoading || isContainerUnavailableStatus(service.status) || (!isOwner && !permissions?.can_control_power)}
                        className="flex items-center gap-2 px-4 py-2.5 rounded-xl hover:bg-secondary text-red-500 transition-all disabled:opacity-30 text-xs font-bold uppercase tracking-wider"
                    >
                        <Skull size={18} />
                        <span>Kill</span>
                    </button>
                </div>
            </div>

            {/* Navigation */}
            <div className="flex items-center gap-1 border-b border-border mb-8 overflow-x-auto no-scrollbar">
                {[
                    { id: 'console', label: 'Console', icon: Terminal, show: isOwner || permissions?.can_view_console },
                    { id: 'files', label: 'File Manager', icon: FolderClosed, show: isOwner || permissions?.can_manage_files },
                    { id: 'startup', label: 'Startup', icon: Rocket, show: isOwner || permissions?.can_edit_startup },
                    { id: 'users', label: 'Sub-Users', icon: Users, show: isOwner },
                    { id: 'activity', label: 'Activity', icon: Clock, show: isOwner },
                    { id: 'settings', label: 'Settings', icon: Settings, show: isOwner },
                ].filter(tab => tab.show).map(tab => (
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
                                <div className="flex items-center gap-3">
                                    <span className="text-[10px] font-black uppercase tracking-widest text-zinc-500">Terminal Output</span>
                                    <button
                                        type="button"
                                        onClick={() => setFollowOutput(true)}
                                        className={clsx(
                                            "text-[9px] font-bold uppercase tracking-widest transition-colors",
                                            followOutput ? "text-emerald-500" : "text-amber-500 hover:text-amber-400"
                                        )}
                                    >
                                        {followOutput ? 'Following' : 'Resume output'}
                                    </button>
                                </div>
                                <div className="flex items-center gap-2">
                                    <div className={clsx("w-1.5 h-1.5 rounded-full", statusInfo.color)} />
                                    <span className="text-[10px] text-zinc-600 uppercase font-bold tracking-widest">
                                        {statusInfo.label}
                                    </span>
                                </div>
                            </div>
                            <div
                                ref={consoleRef}
                                onMouseDown={() => setFollowOutput(false)}
                                onScroll={(event) => {
                                    const element = event.currentTarget;
                                    const atBottom = element.scrollHeight - element.scrollTop - element.clientHeight < 24;
                                    if (atBottom !== followOutput) setFollowOutput(atBottom);
                                }}
                                className="flex-1 p-6 font-mono text-sm overflow-y-auto space-y-1 selection:bg-primary/30 custom-scrollbar scroll-smooth"
                                style={{ scrollBehavior: 'auto' }}
                            >
                                {logs.map((log) => (
                                    <div key={log.id} className="flex gap-4 group/line">
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
                                    placeholder={isOwner || permissions?.can_send_commands ? "Type a command..." : "You do not have permission to send commands"}
                                    disabled={!isOwner && !permissions?.can_send_commands}
                                    className={clsx(
                                        "bg-transparent border-none outline-none flex-1 text-sm font-mono text-zinc-200",
                                        (!isOwner && !permissions?.can_send_commands) && "opacity-50 cursor-not-allowed"
                                    )}
                                    onKeyDown={(e) => {
                                        if (e.key === 'Enter') {
                                            const input = (e.target as HTMLInputElement).value;
                                            if (!input) return;
                                            setLogs(prev => [...prev, makeLogLine(`> ${input}`)]);
                                            handleSendCommand(input);
                                            (e.target as HTMLInputElement).value = '';
                                        }
                                    }}
                                />
                            </div>
                        </div>
                    )}

                    {activeTab === 'files' && (
                        <FileManager service={service} />
                    )}

                    {activeTab === 'startup' && (
                        <div className="space-y-6">
                            <div className="panel-card p-8 border-border">
                                <div className="flex items-center justify-between gap-4 mb-4">
                                    <h3 className="text-lg font-bold">Startup Configuration</h3>
                                    <button
                                        onClick={handleSaveStartupConfiguration}
                                        disabled={startupSaving}
                                        className="btn-primary flex items-center gap-2 disabled:opacity-50"
                                    >
                                        <Save size={16} />
                                        {startupSaving ? 'Saving...' : 'Save'}
                                    </button>
                                </div>
                                <div
                                    className={clsx(
                                        "mb-4 rounded-xl border px-4 py-3 text-sm",
                                        startupMessageType === 'success' && "border-emerald-500/20 bg-emerald-500/5 text-emerald-500",
                                        startupMessageType === 'error' && "border-red-500/20 bg-red-500/5 text-red-500",
                                        startupMessageType === 'info' && "border-blue-500/20 bg-blue-500/5 text-blue-500",
                                        startupMessageType === 'idle' && "border-border bg-secondary/20 text-muted"
                                    )}
                                >
                                    <div className="flex items-center justify-between gap-3">
                                        <div className="flex items-center gap-2">
                                            {startupMessageType === 'success' && <CheckCircle2 size={16} />}
                                            {startupMessageType === 'error' && <AlertTriangle size={16} />}
                                            {startupMessageType === 'info' && <Save size={16} />}
                                            {startupMessageType === 'idle' && <Clock size={16} />}
                                            <span>
                                                {startupMessage || (startupDirty ? 'Unsaved startup changes.' : 'No pending startup changes.')}
                                            </span>
                                        </div>
                                        <span className="text-xs uppercase tracking-widest">
                                            {lastStartupSaveAt ? `Saved ${lastStartupSaveAt}` : (startupDirty ? 'Pending' : 'Idle')}
                                        </span>
                                    </div>
                                </div>
                                <textarea
                                    value={startupCommand}
                                    onChange={(e) => {
                                        setStartupCommand(e.target.value);
                                        setStartupDirty(true);
                                        setStartupMessage('Unsaved startup changes.');
                                        setStartupMessageType('info');
                                    }}
                                    className="input-field font-mono text-xs min-h-[140px]"
                                />
                            </div>
                            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                                {Object.keys(startupEnv).map(key => (
                                    <div key={key} className="panel-card p-6 border-border">
                                        <label className="text-[10px] font-bold text-muted uppercase tracking-widest block mb-4">{key}</label>
                                        <input
                                            type="text"
                                            value={startupEnv[key] ?? ''}
                                            onChange={(e) => {
                                                setStartupEnv(prev => ({ ...prev, [key]: e.target.value }));
                                                setStartupDirty(true);
                                                setStartupMessage('Unsaved startup changes.');
                                                setStartupMessageType('info');
                                            }}
                                            className="input-field"
                                        />
                                    </div>
                                ))}
                            </div>
                            <div className="panel-card p-6 border-border">
                                <div className="flex items-center justify-between gap-4 text-xs uppercase tracking-widest">
                                    <span className="text-muted">Server Snapshot</span>
                                    <span className="font-bold">
                                        {Object.keys(startupEnv).length} variables
                                    </span>
                                </div>
                                <div className="mt-4 grid grid-cols-1 md:grid-cols-2 gap-4 text-sm">
                                    <div>
                                        <div className="text-muted mb-2">Current saved startup command</div>
                                        <div className="rounded-xl border border-border bg-secondary/20 p-3 font-mono text-xs break-all">
                                            {service.startup_command || service.egg?.startup_command || 'Not set'}
                                        </div>
                                    </div>
                                    <div>
                                        <div className="text-muted mb-2">Current saved environment JSON</div>
                                        <div className="rounded-xl border border-border bg-secondary/20 p-3 font-mono text-xs break-all">
                                            {service.environment || '{}'}
                                        </div>
                                    </div>
                                </div>
                            </div>
                        </div>
                    )}

                    {activeTab === 'users' && (
                        <ServiceUsersTab />
                    )}

                    {activeTab === 'activity' && (
                        <ServiceActivityTab />
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

                            <div className="pt-4 border-t border-border/40 grid grid-cols-2 gap-4">
                                <div className="space-y-1">
                                    <div className="text-[9px] font-black text-muted uppercase tracking-widest flex items-center gap-1">
                                        <ArrowDown size={10} className="text-emerald-500" /> Inbound
                                    </div>
                                    <div className="text-xs font-bold font-mono">{(stats?.network?.rx / 1024 / 1024).toFixed(2)} MB</div>
                                </div>
                                <div className="space-y-1 text-right">
                                    <div className="text-[9px] font-black text-muted uppercase tracking-widest flex items-center gap-1 justify-end">
                                        Outbound <ArrowUp size={10} className="text-blue-500" />
                                    </div>
                                    <div className="text-xs font-bold font-mono">{(stats?.network?.tx / 1024 / 1024).toFixed(2)} MB</div>
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
