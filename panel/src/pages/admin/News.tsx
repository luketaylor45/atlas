import { useState, useEffect } from 'react';
import api from '../../lib/api';
import { Bell, Plus, Trash2, Edit3, X, Megaphone, Info, AlertTriangle } from 'lucide-react';
import clsx from 'clsx';

interface NewsItem {
    id: number;
    title: string;
    content: string;
    type: string;
    created_at: string;
}

export default function AdminNewsPage() {
    const [news, setNews] = useState<NewsItem[]>([]);
    const [loading, setLoading] = useState(true);
    const [showModal, setShowModal] = useState(false);

    // Form State
    const [editingId, setEditingId] = useState<number | null>(null);
    const [title, setTitle] = useState('');
    const [content, setContent] = useState('');
    const [type, setType] = useState('Release');

    const fetchNews = async () => {
        try {
            const res = await api.get('/news');
            setNews(res.data);
        } catch (err) {
            console.error(err);
        } finally {
            setLoading(false);
        }
    };

    useEffect(() => {
        fetchNews();
    }, []);

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault();
        try {
            if (editingId) {
                await api.put(`/admin/news/${editingId}`, { title, content, type });
            } else {
                await api.post('/admin/news', { title, content, type });
            }
            setShowModal(false);
            resetForm();
            fetchNews();
        } catch (err) {
            alert("Failed to save news item");
        }
    };

    const handleDelete = async (id: number) => {
        if (!confirm("Delete this announcement?")) return;
        try {
            await api.delete(`/admin/news/${id}`);
            fetchNews();
        } catch (err) {
            alert("Delete failed");
        }
    };

    const resetForm = () => {
        setEditingId(null);
        setTitle('');
        setContent('');
        setType('Release');
    };

    const openEdit = (item: NewsItem) => {
        setEditingId(item.id);
        setTitle(item.title);
        setContent(item.content);
        setType(item.type);
        setShowModal(true);
    };

    if (loading) return <div className="p-12 text-center animate-pulse text-muted uppercase font-bold tracking-widest mt-20">Syncing Announcements...</div>;

    return (
        <div className="space-y-8 animation-enter">
            <div className="flex flex-col md:flex-row md:items-center justify-between gap-6 bg-primary/5 p-8 rounded-3xl border border-primary/20">
                <div>
                    <h1 className="text-3xl font-bold tracking-tight flex items-center gap-3">
                        <Bell className="text-primary" size={28} /> Global Announcements
                    </h1>
                    <p className="text-muted text-sm font-medium mt-1 pr-10">Broadcast critical updates, maintenance notices, and milestone reachings to all platform users dashboard.</p>
                </div>
                <button
                    onClick={() => { resetForm(); setShowModal(true); }}
                    className="flex items-center gap-2 bg-primary text-white px-6 py-3 rounded-xl font-bold text-sm hover:scale-105 transition-all shadow-lg shadow-primary/20 shrink-0"
                >
                    <Plus size={18} />
                    New Broadcast
                </button>
            </div>

            <div className="grid grid-cols-1 gap-4 pb-12">
                {news.length === 0 ? (
                    <div className="py-20 text-center panel-card border-dashed border-2">
                        <Megaphone size={48} className="mx-auto text-muted mb-4 opacity-20" />
                        <p className="text-muted font-bold tracking-tight">No announcements have been dispatched yet.</p>
                    </div>
                ) : (
                    news.map(item => (
                        <div key={item.id} className="panel-card flex flex-col md:flex-row md:items-center justify-between gap-6 p-6 group hover:border-primary/40 transition-all">
                            <div className="flex items-start gap-5">
                                <div className={clsx(
                                    "p-3 rounded-2xl shrink-0 mt-1",
                                    item.type === 'Release' ? "bg-primary/10 text-primary" :
                                        item.type === 'Status' ? "bg-blue-500/10 text-blue-500" :
                                            "bg-amber-500/10 text-amber-500"
                                )}>
                                    {item.type === 'Release' ? <Plus size={20} /> : item.type === 'Status' ? <Info size={20} /> : <AlertTriangle size={20} />}
                                </div>
                                <div>
                                    <div className="flex items-center gap-3 mb-1">
                                        <h3 className="font-bold text-lg leading-tight group-hover:text-primary transition-colors">{item.title}</h3>
                                        <span className="text-[9px] font-black uppercase tracking-widest text-muted">{new Date(item.created_at).toLocaleDateString()}</span>
                                    </div>
                                    <p className="text-xs text-muted font-medium line-clamp-2 leading-relaxed max-w-2xl">{item.content}</p>
                                </div>
                            </div>
                            <div className="flex items-center gap-2 opacity-0 group-hover:opacity-100 transition-opacity">
                                <button
                                    onClick={() => openEdit(item)}
                                    className="p-2.5 hover:bg-secondary rounded-xl text-muted hover:text-foreground transition-all border border-transparent hover:border-border"
                                >
                                    <Edit3 size={16} />
                                </button>
                                <button
                                    onClick={() => handleDelete(item.id)}
                                    className="p-2.5 hover:bg-red-500/10 rounded-xl text-muted hover:text-red-500 transition-all border border-transparent hover:border-red-500/20"
                                >
                                    <Trash2 size={16} />
                                </button>
                            </div>
                        </div>
                    ))
                )}
            </div>

            {showModal && (
                <div className="fixed inset-0 z-[100] flex items-center justify-center p-6 bg-black/60 backdrop-blur-sm">
                    <div className="panel-card max-w-2xl w-full p-0 shadow-2xl animation-enter overflow-hidden">
                        <div className="px-8 py-6 border-b border-border/50 bg-secondary/20 flex items-center justify-between">
                            <h2 className="text-xl font-bold tracking-tight">{editingId ? 'Edit Announcement' : 'Compose Broadcast'}</h2>
                            <button onClick={() => setShowModal(false)} className="p-2 hover:bg-secondary rounded-lg transition-colors">
                                <X size={20} className="text-muted" />
                            </button>
                        </div>

                        <form onSubmit={handleSubmit} className="p-8 space-y-6">
                            <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
                                <div className="space-y-2">
                                    <label className="text-[10px] font-bold uppercase tracking-widest text-muted ml-1">Headline</label>
                                    <input
                                        required
                                        value={title}
                                        onChange={e => setTitle(e.target.value)}
                                        className="w-full bg-secondary/50 border border-border rounded-xl px-4 py-3 text-sm focus:outline-none focus:ring-2 focus:ring-primary/50 transition-all font-medium"
                                        placeholder="Brief, punchy title"
                                    />
                                </div>
                                <div className="space-y-2">
                                    <label className="text-[10px] font-bold uppercase tracking-widest text-muted ml-1">Category</label>
                                    <select
                                        value={type}
                                        onChange={e => setType(e.target.value)}
                                        className="w-full bg-secondary/50 border border-border rounded-xl px-4 py-3 text-sm focus:outline-none focus:ring-2 focus:ring-primary/50 transition-all font-bold appearance-none cursor-pointer"
                                    >
                                        <option value="Release">Release / Update</option>
                                        <option value="Status">System Status</option>
                                        <option value="Maintenance">Maintenance</option>
                                    </select>
                                </div>
                            </div>

                            <div className="space-y-2">
                                <label className="text-[10px] font-bold uppercase tracking-widest text-muted ml-1">Body Content</label>
                                <textarea
                                    required
                                    rows={5}
                                    value={content}
                                    onChange={e => setContent(e.target.value)}
                                    className="w-full bg-secondary/50 border border-border rounded-2xl px-4 py-4 text-sm focus:outline-none focus:ring-2 focus:ring-primary/50 transition-all font-medium resize-none"
                                    placeholder="Detailed information for the users. Supports plain text."
                                />
                            </div>

                            <div className="flex gap-4 pt-4">
                                <button
                                    type="button"
                                    onClick={() => setShowModal(false)}
                                    className="flex-1 px-6 py-3 rounded-xl bg-secondary hover:bg-secondary/80 text-sm font-bold transition-all"
                                >
                                    Discard Changes
                                </button>
                                <button
                                    type="submit"
                                    className="flex-1 px-6 py-3 rounded-xl bg-primary text-white hover:opacity-90 text-sm font-bold shadow-lg shadow-primary/20 transition-all"
                                >
                                    Dispatch Announcement
                                </button>
                            </div>
                        </form>
                    </div>
                </div>
            )}
        </div>
    );
}
