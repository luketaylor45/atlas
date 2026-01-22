import { useState, useEffect } from 'react';
import api from '../../lib/api';
import { Users, Mail, Shield, Trash2, UserPlus, Search, User } from 'lucide-react';

interface UserData {
    id: number;
    username: string;
    email: string;
    is_admin: boolean;
    created_at: string;
}

export default function AdminUsersPage() {
    const [users, setUsers] = useState<UserData[]>([]);
    const [loading, setLoading] = useState(true);
    const [searchTerm, setSearchTerm] = useState('');

    useEffect(() => {
        const fetchUsers = async () => {
            try {
                const res = await api.get('/admin/users');
                setUsers(res.data);
            } catch (err) {
                console.error("Failed to fetch users", err);
            } finally {
                setLoading(false);
            }
        };
        fetchUsers();
    }, []);

    const filteredUsers = users.filter(user =>
        (user.username || '').toLowerCase().includes(searchTerm.toLowerCase()) ||
        user.email.toLowerCase().includes(searchTerm.toLowerCase())
    );

    const deleteUser = async (id: number) => {
        if (!confirm("Are you sure you want to delete this user? This will fail if they have active services.")) return;
        try {
            await api.delete(`/admin/users/${id}`);
            setUsers(users.filter(u => u.id !== id));
        } catch (err: any) {
            alert(err.response?.data?.error || "Failed to delete user");
        }
    };

    if (loading) return <div className="p-12 text-center animate-pulse text-muted font-bold tracking-widest mt-20 uppercase">Loading User Database...</div>;

    return (
        <div className="space-y-8 animation-enter">
            <div className="flex flex-col md:flex-row md:items-center justify-between gap-6 bg-secondary/20 p-8 rounded-3xl border border-border/50">
                <div>
                    <h1 className="text-3xl font-bold tracking-tight flex items-center gap-3">
                        <Users className="text-primary" size={28} /> User Management
                    </h1>
                    <p className="text-muted text-sm font-medium mt-1">Easily manage all {users.length} registered accounts and their permissions.</p>
                </div>
                <div className="flex items-center gap-4">
                    <div className="relative group">
                        <Search className="absolute left-4 top-1/2 -translate-y-1/2 text-muted group-focus-within:text-primary transition-colors" size={16} />
                        <input
                            type="text"
                            placeholder="Search by name or email..."
                            value={searchTerm}
                            onChange={(e) => setSearchTerm(e.target.value)}
                            className="bg-background border border-border/50 pl-11 pr-5 py-2.5 rounded-xl text-sm focus:outline-none focus:ring-2 focus:ring-primary/20 focus:border-primary transition-all w-72 font-medium"
                        />
                    </div>
                </div>
            </div>

            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6 pb-12">
                {filteredUsers.map(user => (
                    <div key={user.id} className="panel-card group relative p-6 flex flex-col border-border/50 hover:border-primary/30 transition-all">
                        <div className="flex items-start justify-between mb-6">
                            <div className="flex items-center gap-4">
                                <div className="w-12 h-12 rounded-2xl bg-secondary flex items-center justify-center text-primary shadow-inner">
                                    <User size={24} />
                                </div>
                                <div className="flex flex-col">
                                    <h3 className="font-bold text-lg group-hover:text-primary transition-colors leading-tight">{user.username || 'Unset Username'}</h3>
                                    <div className="flex items-center gap-1.5 mt-1 text-muted">
                                        <Mail size={12} />
                                        <span className="text-[11px] font-medium tracking-tight truncate border-b border-border/40">{user.email}</span>
                                    </div>
                                </div>
                            </div>
                            {user.is_admin && (
                                <div className="bg-primary/10 text-primary border border-primary/20 px-2 py-0.5 rounded text-[10px] font-bold uppercase tracking-wider flex items-center gap-1">
                                    <Shield size={10} /> Admin
                                </div>
                            )}
                        </div>

                        <div className="mt-auto pt-6 border-t border-border/30 flex items-center justify-between">
                            <div className="text-[10px] text-muted font-bold uppercase tracking-widest">
                                Joined {new Date(user.created_at).toLocaleDateString()}
                            </div>
                            <button
                                onClick={() => deleteUser(user.id)}
                                className="text-red-400 hover:text-red-500 hover:bg-red-500/10 p-2 rounded-lg transition-all"
                                title="Delete User"
                            >
                                <Trash2 size={16} />
                            </button>
                        </div>
                    </div>
                ))}

                <button className="panel-card p-10 flex flex-col items-center justify-center gap-4 border-dashed border-2 border-border/60 hover:border-primary/50 hover:bg-primary/[0.02] transition-all group min-h-[220px]">
                    <div className="w-14 h-14 rounded-full bg-secondary group-hover:bg-primary group-hover:text-white transition-all flex items-center justify-center shadow-inner">
                        <UserPlus size={28} />
                    </div>
                    <div className="text-center">
                        <span className="block font-bold text-lg">Add New User</span>
                        <span className="text-xs text-muted font-medium mt-1">Manually create a new account identity</span>
                    </div>
                </button>
            </div>
        </div>
    );
}
