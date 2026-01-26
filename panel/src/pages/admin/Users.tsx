import { useState, useEffect } from 'react';
import api from '../../lib/api';
import { Users, Shield, Trash2, UserPlus, Search, User as UserIcon, Edit3, X } from 'lucide-react';

interface UserData {
    id: number;
    username: string;
    is_admin: boolean;
    created_at: string;
}

export default function AdminUsersPage() {
    const [users, setUsers] = useState<UserData[]>([]);
    const [loading, setLoading] = useState(true);
    const [searchTerm, setSearchTerm] = useState('');

    // Create User State
    const [showCreateModal, setShowCreateModal] = useState(false);
    const [newUsername, setNewUsername] = useState('');
    const [newPassword, setNewPassword] = useState('');
    const [newIsAdmin, setNewIsAdmin] = useState(false);
    const [createLoading, setCreateLoading] = useState(false);

    // Edit User State
    const [editingUser, setEditingUser] = useState<UserData | null>(null);
    const [editUsername, setEditUsername] = useState('');
    const [editPassword, setEditPassword] = useState('');
    const [editIsAdmin, setEditIsAdmin] = useState(false);
    const [editLoading, setEditLoading] = useState(false);

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

    useEffect(() => {
        fetchUsers();
    }, []);

    const filteredUsers = users.filter(user =>
        (user.username || '').toLowerCase().includes(searchTerm.toLowerCase())
    );

    const handleCreateUser = async (e: React.FormEvent) => {
        e.preventDefault();
        setCreateLoading(true);
        try {
            const res = await api.post('/admin/users', {
                username: newUsername,
                password: newPassword,
                is_admin: newIsAdmin
            });
            setUsers([...users, res.data]);
            setShowCreateModal(false);
            setNewUsername('');
            setNewPassword('');
            setNewIsAdmin(false);
        } catch (err: any) {
            alert(err.response?.data?.error || "Failed to create user");
        } finally {
            setCreateLoading(false);
        }
    };

    const handleUpdateUser = async (e: React.FormEvent) => {
        e.preventDefault();
        if (!editingUser) return;
        setEditLoading(true);
        try {
            const res = await api.put(`/admin/users/${editingUser.id}`, {
                username: editUsername,
                password: editPassword || undefined,
                is_admin: editIsAdmin
            });
            setUsers(users.map(u => u.id === editingUser.id ? res.data : u));
            setEditingUser(null);
        } catch (err: any) {
            alert(err.response?.data?.error || "Failed to update user");
        } finally {
            setEditLoading(false);
        }
    };

    const startEdit = (user: UserData) => {
        setEditingUser(user);
        setEditUsername(user.username);
        setEditPassword('');
        setEditIsAdmin(user.is_admin);
    };

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
                    <p className="text-muted text-sm font-medium mt-1">Easily manage all registered accounts and their permissions.</p>
                </div>
                <div className="flex items-center gap-4">
                    <div className="relative group">
                        <Search className="absolute left-4 top-1/2 -translate-y-1/2 text-muted group-focus-within:text-primary transition-colors" size={16} />
                        <input
                            type="text"
                            placeholder="Search by username..."
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
                                    <UserIcon size={24} />
                                </div>
                                <div className="flex flex-col">
                                    <h3 className="font-bold text-lg group-hover:text-primary transition-colors leading-tight">{user.username || 'Unset Username'}</h3>
                                    <div className="flex items-center gap-1.5 mt-1 text-muted">
                                        <span className="text-[10px] font-medium tracking-tight uppercase">Base User Account</span>
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
                            <div className="flex items-center gap-2">
                                <button
                                    onClick={() => startEdit(user)}
                                    className="text-muted hover:text-primary hover:bg-primary/10 p-2 rounded-lg transition-all"
                                    title="Edit User"
                                >
                                    <Edit3 size={16} />
                                </button>
                                <button
                                    onClick={() => deleteUser(user.id)}
                                    className="text-red-400 hover:text-red-500 hover:bg-red-500/10 p-2 rounded-lg transition-all"
                                    title="Delete User"
                                >
                                    <Trash2 size={16} />
                                </button>
                            </div>
                        </div>
                    </div>
                ))}

                <button
                    onClick={() => setShowCreateModal(true)}
                    className="panel-card p-10 flex flex-col items-center justify-center gap-4 border-dashed border-2 border-border/60 hover:border-primary/50 hover:bg-primary/[0.02] transition-all group min-h-[220px]"
                >
                    <div className="w-14 h-14 rounded-full bg-secondary group-hover:bg-primary group-hover:text-white transition-all flex items-center justify-center shadow-inner">
                        <UserPlus size={28} />
                    </div>
                    <div className="text-center">
                        <span className="block font-bold text-lg">Add New User</span>
                        <span className="text-xs text-muted font-medium mt-1">Manually create a new account identity</span>
                    </div>
                </button>
            </div>

            {/* Create User Modal */}
            {showCreateModal && (
                <div className="fixed inset-0 z-[100] flex items-center justify-center p-6 bg-black/60 backdrop-blur-sm">
                    <div className="panel-card max-w-md w-full p-8 shadow-2xl animation-enter">
                        <div className="flex items-center justify-between mb-8">
                            <div className="flex items-center gap-4">
                                <div className="p-3 rounded-2xl bg-primary/10 text-primary">
                                    <UserPlus size={24} />
                                </div>
                                <h2 className="text-2xl font-bold tracking-tight">Create Identity</h2>
                            </div>
                            <button onClick={() => setShowCreateModal(false)} className="p-2 hover:bg-secondary rounded-lg transition-colors">
                                <X size={20} className="text-muted" />
                            </button>
                        </div>

                        <form onSubmit={handleCreateUser} className="space-y-6">
                            <div className="space-y-2">
                                <label className="text-[10px] font-bold uppercase tracking-widest text-muted ml-1">Username</label>
                                <input
                                    type="text"
                                    required
                                    value={newUsername}
                                    onChange={e => setNewUsername(e.target.value)}
                                    className="w-full bg-secondary/50 border border-border rounded-xl px-4 py-3 text-sm focus:outline-none focus:ring-2 focus:ring-primary/50 transition-all font-medium"
                                    placeholder="e.g. jdoe"
                                />
                            </div>

                            <div className="space-y-2">
                                <label className="text-[10px] font-bold uppercase tracking-widest text-muted ml-1">Initial Password</label>
                                <input
                                    type="password"
                                    required
                                    value={newPassword}
                                    onChange={e => setNewPassword(e.target.value)}
                                    className="w-full bg-secondary/50 border border-border rounded-xl px-4 py-3 text-sm focus:outline-none focus:ring-2 focus:ring-primary/50 transition-all font-medium"
                                    placeholder="Min 8 characters"
                                />
                            </div>

                            <label className="flex items-center gap-3 p-4 rounded-xl border border-border/50 bg-secondary/20 cursor-pointer hover:bg-secondary/40 transition-all group">
                                <div className="relative flex items-center">
                                    <input
                                        type="checkbox"
                                        checked={newIsAdmin}
                                        onChange={e => setNewIsAdmin(e.target.checked)}
                                        className="w-5 h-5 rounded border-border bg-background text-primary focus:ring-primary/50 transition-all cursor-pointer"
                                    />
                                </div>
                                <div>
                                    <span className="block text-sm font-bold group-hover:text-primary transition-colors">Administrator Access</span>
                                    <span className="block text-[10px] text-muted font-medium">Grant full system access and node management</span>
                                </div>
                            </label>

                            <div className="flex gap-4 pt-4">
                                <button
                                    type="button"
                                    onClick={() => setShowCreateModal(false)}
                                    className="flex-1 px-6 py-3 rounded-xl bg-secondary hover:bg-secondary/80 text-sm font-bold transition-all"
                                >
                                    Cancel
                                </button>
                                <button
                                    type="submit"
                                    disabled={createLoading}
                                    className="flex-1 px-6 py-3 rounded-xl bg-primary text-white hover:opacity-90 text-sm font-bold shadow-lg shadow-primary/20 transition-all disabled:opacity-50"
                                >
                                    {createLoading ? 'Provisioning...' : 'Create Account'}
                                </button>
                            </div>
                        </form>
                    </div>
                </div>
            )}

            {/* Edit User Modal */}
            {editingUser && (
                <div className="fixed inset-0 z-[100] flex items-center justify-center p-6 bg-black/60 backdrop-blur-sm">
                    <div className="panel-card max-w-md w-full p-8 shadow-2xl animation-enter">
                        <div className="flex items-center justify-between mb-8">
                            <div className="flex items-center gap-4">
                                <div className="p-3 rounded-2xl bg-primary/10 text-primary">
                                    <Edit3 size={24} />
                                </div>
                                <h2 className="text-2xl font-bold tracking-tight">Modify Identity</h2>
                            </div>
                            <button onClick={() => setEditingUser(null)} className="p-2 hover:bg-secondary rounded-lg transition-colors">
                                <X size={20} className="text-muted" />
                            </button>
                        </div>

                        <form onSubmit={handleUpdateUser} className="space-y-6">
                            <div className="space-y-2">
                                <label className="text-[10px] font-bold uppercase tracking-widest text-muted ml-1">Username</label>
                                <input
                                    type="text"
                                    required
                                    value={editUsername}
                                    onChange={e => setEditUsername(e.target.value)}
                                    className="w-full bg-secondary/50 border border-border rounded-xl px-4 py-3 text-sm focus:outline-none focus:ring-2 focus:ring-primary/50 transition-all font-medium"
                                />
                            </div>

                            <div className="space-y-2">
                                <label className="text-[10px] font-bold uppercase tracking-widest text-muted ml-1">New Password (Optional)</label>
                                <input
                                    type="password"
                                    value={editPassword}
                                    onChange={e => setEditPassword(e.target.value)}
                                    className="w-full bg-secondary/50 border border-border rounded-xl px-4 py-3 text-sm focus:outline-none focus:ring-2 focus:ring-primary/50 transition-all font-medium"
                                    placeholder="Leave blank to keep current"
                                />
                            </div>

                            <label className="flex items-center gap-3 p-4 rounded-xl border border-border/50 bg-secondary/20 cursor-pointer hover:bg-secondary/40 transition-all group">
                                <div className="relative flex items-center">
                                    <input
                                        type="checkbox"
                                        checked={editIsAdmin}
                                        onChange={e => setEditIsAdmin(e.target.checked)}
                                        className="w-5 h-5 rounded border-border bg-background text-primary focus:ring-primary/50 transition-all cursor-pointer"
                                    />
                                </div>
                                <div>
                                    <span className="block text-sm font-bold group-hover:text-primary transition-colors">Administrator Access</span>
                                    <span className="block text-[10px] text-muted font-medium">Grant full system access and node management</span>
                                </div>
                            </label>

                            <div className="flex gap-4 pt-4">
                                <button
                                    type="button"
                                    onClick={() => setEditingUser(null)}
                                    className="flex-1 px-6 py-3 rounded-xl bg-secondary hover:bg-secondary/80 text-sm font-bold transition-all"
                                >
                                    Cancel
                                </button>
                                <button
                                    type="submit"
                                    disabled={editLoading}
                                    className="flex-1 px-6 py-3 rounded-xl bg-primary text-white hover:opacity-90 text-sm font-bold shadow-lg shadow-primary/20 transition-all disabled:opacity-50"
                                >
                                    {editLoading ? 'Saving...' : 'Update Account'}
                                </button>
                            </div>
                        </form>
                    </div>
                </div>
            )}
        </div>
    );
}
