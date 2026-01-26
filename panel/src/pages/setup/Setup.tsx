import { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import api from '../../lib/api';
import Logo from '../../components/Logo';
import { useAuth } from '../../context/AuthContext';

export default function SetupPage() {
    const navigate = useNavigate();
    const [loading, setLoading] = useState(false);
    const [error, setError] = useState('');

    const { user } = useAuth();

    useEffect(() => {
        if (user) {
            navigate('/');
            return;
        }

        api.get('/auth/setup-status')
            .then(res => {
                if (!res.data.setup_required) {
                    navigate('/login');
                }
            })
            .catch(() => { });
    }, [navigate, user]);

    const handleSetup = async (e: React.FormEvent<HTMLFormElement>) => {
        e.preventDefault();
        setError('');
        setLoading(true);

        const formData = new FormData(e.currentTarget);
        const username = formData.get('username') as string;
        const password = formData.get('password') as string;

        try {
            const res = await api.post('/auth/setup', { username, password });

            // Auto login if token provided
            if (res.data.token) {
                localStorage.setItem('token', res.data.token);
                // Refreshing context is handled by AuthContext if we redirect to /
                window.location.href = '/';
            } else {
                navigate('/login');
            }
        } catch (err: any) {
            setError(err.response?.data?.error || 'Setup failed');
        } finally {
            setLoading(false);
        }
    };

    return (
        <div className="min-h-screen flex items-center justify-center bg-background text-foreground font-sans dark relative overflow-hidden">
            <div className="absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 w-[600px] h-[600px] bg-emerald-500/10 rounded-full blur-[120px] pointer-events-none" />

            <div className="w-full max-w-md p-8 glass rounded-2xl border border-border/50 shadow-2xl relative z-10">
                <div className="flex flex-col items-center mb-8">
                    <Logo size="lg" className="mb-2" />
                    <h1 className="text-2xl font-bold tracking-tight">System Setup</h1>
                    <p className="text-muted text-sm mt-2">Create your Root Administrator account</p>
                </div>

                {error && (
                    <div className="bg-red-500/10 border border-red-500/20 text-red-500 p-3 rounded-lg text-sm text-center mb-4">
                        {error}
                    </div>
                )}

                <form onSubmit={handleSetup} className="flex flex-col gap-5">
                    <div className="space-y-2">
                        <label className="text-xs font-medium uppercase tracking-wide text-muted">Admin Username</label>
                        <input
                            name="username"
                            type="text"
                            required
                            minLength={3}
                            className="w-full bg-secondary/50 border border-border rounded-lg px-4 py-3 text-sm focus:outline-none focus:ring-2 focus:ring-primary/50 transition-all placeholder:text-muted/50"
                            placeholder="admin"
                        />
                    </div>

                    <div className="space-y-2">
                        <label className="text-xs font-medium uppercase tracking-wide text-muted">Password</label>
                        <input
                            name="password"
                            type="password"
                            required
                            minLength={8}
                            className="w-full bg-secondary/50 border border-border rounded-lg px-4 py-3 text-sm focus:outline-none focus:ring-2 focus:ring-primary/50 transition-all placeholder:text-muted/50"
                            placeholder="••••••••"
                        />
                    </div>

                    <button
                        type="submit"
                        disabled={loading}
                        className="w-full bg-emerald-500 text-white font-semibold py-3 rounded-lg hover:bg-emerald-600 transition-colors mt-2 disabled:opacity-50"
                    >
                        {loading ? 'Configuring System...' : 'Create Administrator'}
                    </button>
                </form>
            </div>
        </div>
    );
}
