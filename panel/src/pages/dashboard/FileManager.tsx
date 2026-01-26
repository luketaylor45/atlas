import { useState, useEffect } from 'react';
import { useParams } from 'react-router-dom';
import api from '../../lib/api';
import {
    Folder, ChevronRight, Home, Upload,
    Plus, Trash2, Save, X, MoreVertical,
    FileText, Code, Settings, CornerUpLeft, RefreshCw, Key
} from 'lucide-react';
import clsx from 'clsx';
import { useAuth } from '../../context/AuthContext';

interface FileInfo {
    name: string;
    size: number;
    is_dir: boolean;
}

export default function FileManager({ service }: { service?: any }) {
    const { user } = useAuth();
    const { uuid } = useParams();
    const [path, setPath] = useState('');
    const [files, setFiles] = useState<FileInfo[]>([]);
    const [loading, setLoading] = useState(true);
    const [editingFile, setEditingFile] = useState<{ name: string, content: string } | null>(null);
    const [showNewFolderModal, setShowNewFolderModal] = useState(false);
    const [showSFTPModal, setShowSFTPModal] = useState(false);
    const [newFolderName, setNewFolderName] = useState('');

    const fetchFiles = async () => {
        setLoading(true);
        try {
            const res = await api.get(`/services/${uuid}/files/list?path=${path}`);
            setFiles(res.data);
        } catch (err) {
            console.error(err);
        } finally {
            setLoading(false);
        }
    };

    useEffect(() => {
        fetchFiles();
    }, [path]);

    const navigateTo = (folderName: string) => {
        const newPath = path === '' ? folderName : `${path}/${folderName}`;
        setPath(newPath);
    };

    const goBack = () => {
        const parts = path.split('/');
        parts.pop();
        setPath(parts.join('/'));
    };

    const formatSize = (bytes: number) => {
        if (bytes === 0) return '0 B';
        const k = 1024;
        const sizes = ['B', 'KB', 'MB', 'GB'];
        const i = Math.floor(Math.log(bytes) / Math.log(k));
        return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
    };

    const getFileIcon = (name: string, is_dir: boolean) => {
        if (is_dir) return <Folder className="text-blue-500 fill-blue-500/20" size={20} />;
        const ext = name.split('.').pop()?.toLowerCase();
        if (['json', 'yml', 'yaml', 'toml', 'conf'].includes(ext || '')) return <Settings className="text-orange-500" size={20} />;
        if (['js', 'ts', 'tsx', 'jsx', 'php', 'py', 'go', 'sh'].includes(ext || '')) return <Code className="text-emerald-500" size={20} />;
        return <FileText className="text-muted" size={20} />;
    };

    const openFile = async (name: string) => {
        try {
            const fullPath = path === '' ? name : `${path}/${name}`;
            const res = await api.get(`/services/${uuid}/files/content?path=${fullPath}`);
            setEditingFile({ name: fullPath, content: res.data });
        } catch (err) {
            alert("Failed to read file.");
        }
    };

    const saveFile = async () => {
        if (!editingFile) return;
        try {
            await api.post(`/services/${uuid}/files/write`, {
                path: editingFile.name,
                content: editingFile.content
            });
            setEditingFile(null);
            fetchFiles();
        } catch (err) {
            alert("Failed to save file.");
        }
    };

    const deleteItem = async (name: string) => {
        if (!confirm(`Are you sure you want to delete ${name}?`)) return;
        try {
            const fullPath = path === '' ? name : `${path}/${name}`;
            await api.delete(`/services/${uuid}/files?path=${fullPath}`);
            fetchFiles();
        } catch (err) {
            alert("Failed to delete.");
        }
    };

    const createFolder = async () => {
        try {
            const fullPath = path === '' ? newFolderName : `${path}/${newFolderName}`;
            await api.post(`/services/${uuid}/files/create-folder`, { path: fullPath });
            setNewFolderName('');
            setShowNewFolderModal(false);
            fetchFiles();
        } catch (err) {
            alert("Failed to create folder.");
        }
    };

    const handleUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
        const file = e.target.files?.[0];
        if (!file) return;

        const formData = new FormData();
        formData.append('file', file);

        setLoading(true);
        try {
            await api.post(`/services/${uuid}/files/upload?path=${path}`, formData, {
                headers: { 'Content-Type': 'multipart/form-data' }
            });
            fetchFiles();
        } catch (err) {
            alert("Failed to upload file.");
        } finally {
            setLoading(false);
        }
    };

    return (
        <div className="space-y-6 animation-enter">
            <div className="flex flex-col md:flex-row md:items-center justify-between gap-6">
                <div>
                    <div className="flex items-center gap-2">
                        <button
                            onClick={() => setPath('')}
                            className="bg-secondary/50 hover:bg-secondary p-1.5 rounded-lg transition-colors"
                        >
                            <Home size={14} className="text-muted" />
                        </button>
                        {path.split('/').filter(p => p !== '').map((part, i, arr) => (
                            <div key={i} className="flex items-center gap-2">
                                <ChevronRight size={14} className="text-muted/50" />
                                <button
                                    onClick={() => setPath(arr.slice(0, i + 1).join('/'))}
                                    className="text-xs font-bold text-muted hover:text-foreground transition-colors uppercase tracking-widest"
                                >
                                    {part}
                                </button>
                            </div>
                        ))}
                    </div>
                </div>

                <div className="flex items-center gap-3">
                    <button
                        onClick={() => setShowSFTPModal(true)}
                        className="bg-secondary text-primary border border-primary/20 px-4 py-2 rounded-xl text-sm font-bold flex items-center gap-2 hover:bg-primary/5 transition-all"
                    >
                        <Key size={18} /> SFTP
                    </button>
                    <button
                        onClick={() => setShowNewFolderModal(true)}
                        className="bg-secondary text-foreground border border-border/50 px-4 py-2 rounded-xl text-sm font-bold flex items-center gap-2 hover:bg-secondary/80 transition-all"
                    >
                        <Plus size={18} /> New Folder
                    </button>
                    <label className="bg-primary text-white shadow-lg shadow-primary/20 px-4 py-2 rounded-xl text-sm font-bold flex items-center gap-2 hover:brightness-110 transition-all cursor-pointer">
                        <Upload size={18} /> Upload
                        <input type="file" className="hidden" onChange={handleUpload} />
                    </label>
                </div>
            </div>

            <div className="panel-card !p-0 border-border/40 overflow-hidden min-h-[500px] flex flex-col">
                <div className="bg-secondary/30 border-b border-border/50 px-6 py-4 flex items-center justify-between">
                    <span className="text-[10px] font-bold text-muted uppercase tracking-[0.2em]">Listing {files.length} items</span>
                    <button
                        onClick={fetchFiles}
                        className="p-1.5 hover:bg-secondary rounded-lg transition-all text-muted"
                    >
                        <RefreshCw size={14} className={clsx(loading && "animate-spin")} />
                    </button>
                </div>

                {loading ? (
                    <div className="flex-1 flex items-center justify-center">
                        <div className="animate-pulse text-[10px] font-bold text-muted uppercase tracking-widest">Hydrating Filesystem...</div>
                    </div>
                ) : (
                    <div className="flex-1">
                        <table className="w-full text-left">
                            <thead>
                                <tr className="border-b border-border/30">
                                    <th className="px-6 py-3 text-[10px] font-bold text-muted uppercase tracking-widest">Name</th>
                                    <th className="px-6 py-3 text-[10px] font-bold text-muted uppercase tracking-widest">Size</th>
                                    <th className="px-6 py-3 text-[10px] font-bold text-muted uppercase tracking-widest text-right">Actions</th>
                                </tr>
                            </thead>
                            <tbody className="divide-y divide-border/20">
                                {path !== '' && (
                                    <tr
                                        onClick={goBack}
                                        className="hover:bg-primary/[0.02] cursor-pointer group transition-colors"
                                    >
                                        <td className="px-6 py-4 flex items-center gap-4">
                                            <CornerUpLeft className="text-muted/50 group-hover:text-primary transition-colors" size={18} />
                                            <span className="text-sm font-bold text-muted">...</span>
                                        </td>
                                        <td></td>
                                        <td></td>
                                    </tr>
                                )}
                                {files.map(file => (
                                    <tr
                                        key={file.name}
                                        onClick={() => file.is_dir ? navigateTo(file.name) : openFile(file.name)}
                                        className="hover:bg-primary/[0.02] cursor-pointer group transition-colors"
                                    >
                                        <td className="px-6 py-4">
                                            <div className="flex items-center gap-4">
                                                {getFileIcon(file.name, file.is_dir)}
                                                <span className="text-sm font-bold group-hover:text-primary transition-colors">{file.name}</span>
                                            </div>
                                        </td>
                                        <td className="px-6 py-4">
                                            <span className="text-xs font-medium text-muted">{file.is_dir ? '--' : formatSize(file.size)}</span>
                                        </td>
                                        <td className="px-6 py-4 text-right">
                                            <div className="flex items-center justify-end gap-2 opacity-0 group-hover:opacity-100 transition-all">
                                                <button
                                                    onClick={(e) => { e.stopPropagation(); deleteItem(file.name); }}
                                                    className="p-2 hover:bg-red-500/10 hover:text-red-500 rounded-lg text-muted transition-colors"
                                                >
                                                    <Trash2 size={16} />
                                                </button>
                                                <button className="p-2 hover:bg-secondary rounded-lg text-muted transition-colors">
                                                    <MoreVertical size={16} />
                                                </button>
                                            </div>
                                        </td>
                                    </tr>
                                ))}
                            </tbody>
                        </table>
                        {files.length === 0 && (
                            <div className="p-20 text-center space-y-4">
                                <div className="w-16 h-16 bg-secondary/30 rounded-3xl flex items-center justify-center mx-auto">
                                    <Folder className="text-muted" size={32} />
                                </div>
                                <div className="text-[10px] font-bold text-muted uppercase tracking-widest">Directory is empty</div>
                            </div>
                        )}
                    </div>
                )}
            </div>

            {/* File Editor Modal */}
            {editingFile && (
                <div className="fixed inset-0 z-[100] flex items-center justify-center p-4 bg-black/60 backdrop-blur-sm animate-in fade-in">
                    <div className="bg-background border border-border/60 rounded-3xl w-full max-w-5xl h-[80vh] shadow-2xl overflow-hidden flex flex-col scale-in">
                        <div className="p-6 border-b border-border/50 flex items-center justify-between bg-secondary/10">
                            <div className="flex items-center gap-3">
                                <FileText className="text-primary" />
                                <div>
                                    <h3 className="font-bold">{editingFile.name}</h3>
                                    <p className="text-[10px] text-muted font-bold uppercase tracking-widest">Editing File Content</p>
                                </div>
                            </div>
                            <div className="flex items-center gap-3">
                                <button
                                    onClick={() => setEditingFile(null)}
                                    className="px-4 py-2 rounded-xl text-sm font-bold text-muted hover:bg-secondary transition-all"
                                >
                                    Cancel
                                </button>
                                <button
                                    onClick={saveFile}
                                    className="bg-primary text-white px-6 py-2 rounded-xl text-sm font-bold flex items-center gap-2 hover:brightness-110 transition-all shadow-lg shadow-primary/20"
                                >
                                    <Save size={18} /> Save File
                                </button>
                            </div>
                        </div>
                        <div className="flex-1 p-0 bg-[#0d1117]">
                            <textarea
                                className="w-full h-full bg-transparent text-slate-300 font-mono text-sm p-8 outline-none resize-none selection:bg-primary/30"
                                value={editingFile.content}
                                onChange={(e) => setEditingFile({ ...editingFile, content: e.target.value })}
                                spellCheck={false}
                            />
                        </div>
                    </div>
                </div>
            )}

            {/* New Folder Modal */}
            {showNewFolderModal && (
                <div className="fixed inset-0 z-[100] flex items-center justify-center p-4 bg-black/60 backdrop-blur-sm animate-in fade-in">
                    <div className="bg-background border border-border/60 rounded-3xl w-full max-w-md shadow-2xl scale-in">
                        <div className="p-6 border-b border-border/50 flex items-center justify-between">
                            <h3 className="font-bold text-lg">Create New Folder</h3>
                            <button onClick={() => setShowNewFolderModal(false)} className="p-2 hover:bg-secondary rounded-xl transition-colors"><X size={20} /></button>
                        </div>
                        <div className="p-8 space-y-6">
                            <div className="space-y-2">
                                <label className="text-[10px] font-bold text-muted uppercase tracking-widest pl-1">Folder Name</label>
                                <input
                                    autoFocus
                                    className="input-field"
                                    placeholder="e.g. plugins"
                                    value={newFolderName}
                                    onChange={(e) => setNewFolderName(e.target.value)}
                                    onKeyDown={(e) => e.key === 'Enter' && createFolder()}
                                />
                            </div>
                        </div>
                        <div className="p-6 bg-secondary/10 flex justify-end gap-3">
                            <button
                                onClick={createFolder}
                                className="bg-primary text-white px-8 py-2.5 rounded-xl font-bold text-sm shadow-lg shadow-primary/20 hover:brightness-110 transition-all"
                            >
                                Create Folder
                            </button>
                        </div>
                    </div>
                </div>
            )}
            {/* SFTP Modal */}
            {showSFTPModal && (
                <div className="fixed inset-0 z-[100] flex items-center justify-center p-4 bg-black/60 backdrop-blur-sm animate-in fade-in">
                    <div className="bg-background border border-border/60 rounded-3xl w-full max-w-md shadow-2xl scale-in overflow-hidden">
                        <div className="p-6 border-b border-border/50 flex items-center justify-between bg-primary/5">
                            <div className="flex items-center gap-3">
                                <Key className="text-primary" size={20} />
                                <h3 className="font-bold text-lg">SFTP Connection</h3>
                            </div>
                            <button onClick={() => setShowSFTPModal(false)} className="p-2 hover:bg-secondary rounded-xl transition-colors"><X size={20} /></button>
                        </div>
                        <div className="p-8 space-y-6">
                            <div className="p-4 bg-blue-500/5 border border-blue-500/20 rounded-xl text-[10px] text-blue-500 font-medium leading-relaxed uppercase tracking-wider">
                                Use a client like FileZilla or WinSCP to manage your server files directly over SSH.
                            </div>

                            <div className="space-y-4">
                                <div className="space-y-1.5">
                                    <label className="text-[10px] font-bold text-muted uppercase tracking-widest pl-1">Server Address</label>
                                    <div className="p-3 bg-secondary/50 border border-border rounded-xl font-mono text-xs break-all">
                                        {service.node?.address || 'N/A'}
                                    </div>
                                </div>
                                <div className="space-y-1.5">
                                    <label className="text-[10px] font-bold text-muted uppercase tracking-widest pl-1">Port</label>
                                    <div className="p-3 bg-secondary/50 border border-border rounded-xl font-mono text-xs">
                                        {service.node?.sftp_port || '2022'}
                                    </div>
                                </div>
                                <div className="space-y-1.5">
                                    <label className="text-[10px] font-bold text-muted uppercase tracking-widest pl-1">Username</label>
                                    <div className="p-3 bg-secondary/50 border border-border rounded-xl font-mono text-xs flex justify-between items-center group">
                                        <span>{service.uuid?.substring(0, 8)}.{user?.username}</span>
                                        <button
                                            onClick={() => navigator.clipboard.writeText(`${service.uuid?.substring(0, 8)}.${user?.username}`)}
                                            className="text-[9px] font-bold text-primary opacity-0 group-hover:opacity-100 transition-opacity uppercase"
                                        >
                                            Copy
                                        </button>
                                    </div>
                                </div>
                                <div className="space-y-1.5">
                                    <label className="text-[10px] font-bold text-muted uppercase tracking-widest pl-1">Password</label>
                                    <div className="p-3 bg-emerald-500/5 border border-emerald-500/20 rounded-xl text-xs font-bold text-emerald-500 italic">
                                        Your Atlas Account Password
                                    </div>
                                </div>
                            </div>
                        </div>
                        <div className="p-6 bg-secondary/10 flex justify-end">
                            <button
                                onClick={() => setShowSFTPModal(false)}
                                className="bg-primary text-white px-8 py-2.5 rounded-xl font-bold text-sm shadow-lg shadow-primary/20 hover:brightness-110 transition-all w-full"
                            >
                                Got it
                            </button>
                        </div>
                    </div>
                </div>
            )}
        </div>
    );
}
