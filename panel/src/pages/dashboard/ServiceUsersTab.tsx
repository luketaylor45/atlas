import { useState, useEffect } from 'react';
import { useParams } from 'react-router-dom';
import api from '../../lib/api';
import { Users, Plus, Shield, Terminal, FileText, Settings, Zap, Key, Trash2, AlertTriangle, CheckCircle } from 'lucide-react';
import clsx from 'clsx';

interface ServiceUser {
    id: number;
    user_id: number;
    can_view_console: boolean;
    can_send_commands: boolean;
    can_manage_files: boolean;
    can_edit_startup: boolean;
    can_control_power: boolean;
    can_access_sftp: boolean;
    created_at: string;
    user?: {
        username: string;
    };
}

export default function ServiceUsersTab() {
    const { uuid } = useParams();
    const [users, setUsers] = useState<ServiceUser[]>([]);
    const [allUsers, setAllUsers] = useState<any[]>([]);
    const [loading, setLoading] = useState(true);
    const [showAddModal, setShowAddModal] = useState(false);
    const [selectedUser, setSelectedUser] = useState<number>(0);
    const [sftpPassword, setSftpPassword] = useState('');
    const [permissions, setPermissions] = useState({
        can_view_console: true,
        can_send_commands: false,
        can_manage_files: false,
        can_edit_startup: false,
        can_control_power: false,
        can_access_sftp: false,
    });

    useEffect(() => {
        fetchServiceUsers();
        fetchAllUsers();
    }, [uuid]);

    const fetchServiceUsers = async () => {
        try {
            const res = await api.get(`/services/${uuid}/users`);
            setUsers(res.data);
        } catch (err) {
            console.error('Failed to fetch service users', err);
        } finally {
            setLoading(false);
        }
    };

    const fetchAllUsers = async () => {
        try {
            const res = await api.get('/admin/users');
            setAllUsers(res.data);
        } catch (err) {
            console.error('Failed to fetch users', err);
        }
    };

    const handleAddUser = async () => {
        if (!selectedUser || !sftpPassword) {
            alert('Please select a user and enter an SFTP password');
            return;
        }

        try {
            await api.post(`/services/${uuid}/users`, {
                user_id: selectedUser,
                sftp_password: sftpPassword,
                ...permissions
            });

            setShowAddModal(false);
            setSftpPassword('');
            setSelectedUser(0);
            setPermissions({
                can_view_console: true,
                can_send_commands: false,
                can_manage_files: false,
                can_edit_startup: false,
                can_control_power: false,
                can_access_sftp: false,
            });
            fetchServiceUsers();
        } catch (err: any) {
            alert(err.response?.data?.error || 'Failed to add user');
        }
    };

    const handleRemoveUser = async (userId: number) => {
        if (!confirm('Remove this user\'s access to the service?')) return;

        try {
            await api.delete(`/services/${uuid}/users/${userId}`);
            fetchServiceUsers();
        } catch (err) {
            alert('Failed to remove user');
        }
    };

    const generatePassword = () => {
        const chars = 'ABCDEFGHJKLMNPQRSTUVWXYZabcdefghjkmnpqrstuvwxyz23456789!@#$%';
        let password = '';
        for (let i = 0; i < 16; i++) {
            password += chars.charAt(Math.floor(Math.random() * chars.length));
        }
        setSftpPassword(password);
    };

    const permissionIcons: Record<string, any> = {
        can_view_console: Terminal,
        can_send_commands: Zap,
        can_manage_files: FileText,
        can_edit_startup: Settings,
        can_control_power: Shield,
        can_access_sftp: Key,
    };

    const permissionLabels: Record<string, string> = {
        can_view_console: 'View Console',
        can_send_commands: 'Send Commands',
        can_manage_files: 'Manage Files',
        can_edit_startup: 'Edit Startup',
        can_control_power: 'Control Power',
        can_access_sftp: 'SFTP Access',
    };

    if (loading) return <div className="p-12 text-center text-muted animate-pulse">Loading sub-users...</div>;

    return (
        <div className="space-y-8">
            {/* Add Modal */}
            {showAddModal && (
                <div className="fixed inset-0 z-[110] flex items-center justify-center p-6 bg-black/60 backdrop-blur-sm animate-in">
                    <div className="panel-card max-w-2xl w-full border-primary/20 shadow-2xl">
                        <div className="flex items-center gap-4 text-primary mb-8">
                            <Users size={32} />
                            <h2 className="text-2xl font-bold tracking-tight">Add Sub-User</h2>
                        </div>

                        <div className="space-y-6">
                            <div className="space-y-3">
                                <label className="text-xs font-bold text-muted uppercase tracking-widest">Select User</label>
                                <select
                                    value={selectedUser}
                                    onChange={(e) => setSelectedUser(parseInt(e.target.value))}
                                    className="input-field"
                                >
                                    <option value={0}>Choose a user...</option>
                                    {allUsers.filter(u => !users.some(su => su.user_id === u.id)).map(user => (
                                        <option key={user.id} value={user.id}>{user.username}</option>
                                    ))}
                                </select>
                            </div>

                            <div className="space-y-3">
                                <label className="text-xs font-bold text-muted uppercase tracking-widest">SFTP Password</label>
                                <div className="flex gap-2">
                                    <input
                                        type="text"
                                        value={sftpPassword}
                                        onChange={(e) => setSftpPassword(e.target.value)}
                                        className="input-field flex-1"
                                        placeholder="Enter secure password..."
                                    />
                                    <button
                                        onClick={generatePassword}
                                        className="px-4 py-2 bg-primary text-white rounded-xl font-bold text-sm hover:bg-primary/90 transition-all"
                                    >
                                        Generate
                                    </button>
                                </div>
                                <p className="text-xs text-muted">This password is used for SFTP connections only. Minimum 8 characters.</p>
                            </div>

                            <div className="space-y-3">
                                <label className="text-xs font-bold text-muted uppercase tracking-widest">Permissions</label>
                                <div className="grid grid-cols-2 gap-3">
                                    {Object.keys(permissions).map(key => {
                                        const Icon = permissionIcons[key];
                                        return (
                                            <label
                                                key={key}
                                                className={clsx(
                                                    "flex items-center gap-3 p-4 rounded-xl border-2 cursor-pointer transition-all",
                                                    permissions[key as keyof typeof permissions]
                                                        ? "border-primary bg-primary/5"
                                                        : "border-border hover:border-primary/30"
                                                )}
                                            >
                                                <input
                                                    type="checkbox"
                                                    checked={permissions[key as keyof typeof permissions]}
                                                    onChange={(e) => setPermissions({ ...permissions, [key]: e.target.checked })}
                                                    className="hidden"
                                                />
                                                <Icon size={18} className={permissions[key as keyof typeof permissions] ? "text-primary" : "text-muted"} />
                                                <span className="text-sm font-bold">{permissionLabels[key]}</span>
                                            </label>
                                        );
                                    })}
                                </div>
                            </div>
                        </div>

                        <div className="flex gap-3 mt-8">
                            <button
                                onClick={() => setShowAddModal(false)}
                                className="flex-1 px-6 py-3 rounded-xl font-bold bg-secondary hover:bg-secondary/80 transition-all"
                            >
                                Cancel
                            </button>
                            <button
                                onClick={handleAddUser}
                                disabled={!selectedUser || !sftpPassword}
                                className="flex-1 px-6 py-3 rounded-xl font-bold bg-primary text-white hover:bg-primary/90 transition-all disabled:opacity-50 disabled:cursor-not-allowed shadow-lg shadow-primary/20"
                            >
                                Add Sub-User
                            </button>
                        </div>
                    </div>
                </div>
            )}

            {/* Header */}
            <div className="flex items-center justify-between">
                <div>
                    <h3 className="text-2xl font-bold tracking-tight flex items-center gap-3">
                        <Users className="text-primary" size={28} /> Sub-Users
                    </h3>
                    <p className="text-muted text-sm mt-1">Manage user access and permissions for this service</p>
                </div>
                <button
                    onClick={() => setShowAddModal(true)}
                    className="btn-primary flex items-center gap-2"
                >
                    <Plus size={18} /> Add Sub-User
                </button>
            </div>

            {/* Users Grid */}
            {users.length === 0 ? (
                <div className="panel-card p-12 text-center border-dashed border-2">
                    <Users size={48} className="mx-auto text-muted/30 mb-4" />
                    <h4 className="font-bold text-lg mb-2">No Sub-Users</h4>
                    <p className="text-sm text-muted mb-6">Grant other users access to this service with custom permissions</p>
                    <button
                        onClick={() => setShowAddModal(true)}
                        className="btn-primary inline-flex items-center gap-2"
                    >
                        <Plus size={18} /> Add First Sub-User
                    </button>
                </div>
            ) : (
                <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
                    {users.map(user => (
                        <div key={user.id} className="panel-card p-6 border-border">
                            <div className="flex items-start justify-between mb-6">
                                <div className="flex items-center gap-4">
                                    <div className="w-12 h-12 rounded-2xl bg-primary/10 flex items-center justify-center text-primary">
                                        <Users size={24} />
                                    </div>
                                    <div>
                                        <h4 className="font-bold text-lg">{user.user?.username || `User #${user.user_id}`}</h4>
                                        <p className="text-xs text-muted">Added {new Date(user.created_at).toLocaleDateString()}</p>
                                    </div>
                                </div>
                                <button
                                    onClick={() => handleRemoveUser(user.user_id)}
                                    className="p-2 hover:bg-red-500/10 text-red-500 rounded-lg transition-all"
                                    title="Remove Access"
                                >
                                    <Trash2 size={18} />
                                </button>
                            </div>

                            <div className="space-y-2">
                                <div className="text-xs font-bold text-muted uppercase tracking-widest mb-3">Permissions</div>
                                <div className="grid grid-cols-2 gap-2">
                                    {Object.keys(permissions).map(key => {
                                        const Icon = permissionIcons[key];
                                        const hasPermission = user[key as keyof ServiceUser];
                                        return (
                                            <div
                                                key={key}
                                                className={clsx(
                                                    "flex items-center gap-2 p-2 rounded-lg text-xs font-medium",
                                                    hasPermission ? "bg-emerald-500/10 text-emerald-500" : "bg-secondary text-muted"
                                                )}
                                            >
                                                {hasPermission ? <CheckCircle size={14} /> : <Icon size={14} className="opacity-30" />}
                                                <span>{permissionLabels[key]}</span>
                                            </div>
                                        );
                                    })}
                                </div>
                            </div>
                        </div>
                    ))}
                </div>
            )}

            {/* Info Card */}
            <div className="panel-card p-6 bg-blue-500/5 border-blue-500/20">
                <div className="flex gap-4">
                    <AlertTriangle size={24} className="text-blue-500 shrink-0" />
                    <div className="space-y-2">
                        <h4 className="font-bold text-sm">About Sub-Users</h4>
                        <p className="text-xs text-muted leading-relaxed">
                            Sub-users can access this service based on the permissions you grant. Each user gets their own SFTP password.
                            They can log in to SFTP using their Atlas username and the password you set here. All actions are logged in the Activity tab.
                        </p>
                    </div>
                </div>
            </div>
        </div>
    );
}
