import { useState } from 'react';
import { useAuth } from '../../context/AuthContext';
import { User, Shield, Lock, Bell, Terminal, Palette, Globe } from 'lucide-react';
import clsx from 'clsx';

type SettingsTab = 'profile' | 'security' | 'appearance' | 'notifications';

export default function SettingsPage() {
    const { user } = useAuth();
    const [activeTab, setActiveTab] = useState<SettingsTab>('profile');

    return (
        <div className="max-w-5xl mx-auto animation-enter">
            <div className="mb-10">
                <h1 className="text-4xl font-bold tracking-tight mb-2">Account Settings</h1>
                <p className="text-muted text-lg">Manage your identity, security preferences, and dashboard customization.</p>
            </div>

            <div className="flex flex-col lg:flex-row gap-10">
                {/* Navigation */}
                <div className="w-full lg:w-64 shrink-0 flex flex-col gap-1">
                    {[
                        { id: 'profile', label: 'My Identity', icon: User },
                        { id: 'security', label: 'Security & Keys', icon: Shield },
                        { id: 'appearance', label: 'Interface', icon: Palette },
                        { id: 'notifications', label: 'Alerts', icon: Bell },
                    ].map(tab => (
                        <button
                            key={tab.id}
                            onClick={() => setActiveTab(tab.id as SettingsTab)}
                            className={clsx(
                                "flex items-center gap-3 px-4 py-3 rounded-xl text-sm font-bold transition-all",
                                activeTab === tab.id ? "bg-primary text-white shadow-lg shadow-primary/20" : "text-muted hover:text-foreground hover:bg-secondary/50"
                            )}
                        >
                            <tab.icon size={18} />
                            {tab.label}
                        </button>
                    ))}
                </div>

                {/* Content */}
                <div className="flex-1">
                    {activeTab === 'profile' && (
                        <div className="space-y-8 animate-in slide-in-from-bottom-2 duration-300">
                            <div className="panel-card p-8">
                                <h3 className="text-xl font-bold mb-6">Profile Information</h3>
                                <div className="space-y-6">
                                    <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
                                        <div className="space-y-2">
                                            <label className="text-[10px] font-bold uppercase tracking-widest text-muted ml-1">Username</label>
                                            <input disabled value={user?.username} className="w-full bg-secondary/30 border border-border/50 rounded-xl px-4 py-3 text-sm font-bold opacity-70" />
                                        </div>
                                        <div className="space-y-2">
                                            <label className="text-[10px] font-bold uppercase tracking-widest text-muted ml-1">Account Role</label>
                                            <div className="w-full bg-secondary/30 border border-border/50 rounded-xl px-4 py-3 text-sm font-bold text-primary italic">
                                                {user?.is_admin ? 'Platform Administrator' : 'Standard User'}
                                            </div>
                                        </div>
                                    </div>
                                    <div className="p-4 rounded-xl bg-primary/5 border border-primary/20 flex gap-4">
                                        <Globe size={20} className="text-primary shrink-0 mt-1" />
                                        <p className="text-xs text-muted leading-relaxed">
                                            Your account is synchronized across the Atlas Network. Contact your system admin to modify your username or permissions.
                                        </p>
                                    </div>
                                </div>
                            </div>

                            <div className="panel-card p-8 border-red-500/20">
                                <h3 className="text-xl font-bold text-red-500 mb-2 font-bold tracking-tight">Danger Zone</h3>
                                <p className="text-sm text-muted mb-6">Permanently delete your account and all associated data.</p>
                                <button className="px-6 py-2 rounded-xl bg-red-600/10 text-red-500 border border-red-500/20 font-bold text-xs hover:bg-red-600 hover:text-white transition-all">Deactivate Identity</button>
                            </div>
                        </div>
                    )}

                    {activeTab === 'security' && (
                        <div className="space-y-8 animate-in slide-in-from-bottom-2 duration-300">
                            <div className="panel-card p-8">
                                <h3 className="text-xl font-bold mb-6 flex items-center gap-3"><Lock size={20} className="text-primary" /> Update Password</h3>
                                <div className="space-y-4 max-w-md">
                                    <input type="password" placeholder="Current Password" className="w-full bg-secondary/50 border border-border rounded-xl px-4 py-3 text-sm" />
                                    <input type="password" placeholder="New Password" className="w-full bg-secondary/50 border border-border rounded-xl px-4 py-3 text-sm" />
                                    <button className="bg-primary text-white px-6 py-2.5 rounded-xl font-bold text-sm shadow-lg shadow-primary/20">Update Security Credentials</button>
                                </div>
                            </div>

                            <div className="panel-card p-8 bg-secondary/10 border-dashed">
                                <div className="text-center py-6">
                                    <Shield size={40} className="mx-auto text-muted mb-4 opacity-20" />
                                    <h4 className="font-bold mb-1">Two-Factor Authentication</h4>
                                    <p className="text-xs text-muted max-w-xs mx-auto">Hardware security keys and TOTP generators can be linked to your Atlas identity for enhanced reach protection.</p>
                                    <button disabled className="mt-6 px-6 py-2 rounded-xl bg-secondary text-muted font-bold text-xs uppercase cursor-not-allowed">Coming in Next Cluster Sync</button>
                                </div>
                            </div>
                        </div>
                    )}

                    {(activeTab === 'appearance' || activeTab === 'notifications') && (
                        <div className="panel-card p-20 flex flex-col items-center justify-center text-center opacity-50">
                            <Terminal size={48} className="mb-4 text-primary" />
                            <h3 className="text-xl font-bold">Encrypted Configuration Block</h3>
                            <p className="text-sm text-muted mt-2">Custom {activeTab} preference data is currently restricted by node policy.</p>
                        </div>
                    )}
                </div>
            </div>
        </div>
    );
}
