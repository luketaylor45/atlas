import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import Logo from '../../components/Logo';
import api from '../../lib/api';
import { useAuth } from '../../context/AuthContext';

export default function LoginPage() {
    const navigate = useNavigate();
    const { login } = useAuth();
    const [email, setEmail] = useState('');
    const [password, setPassword] = useState('');
    const [error, setError] = useState('');
    const [loading, setLoading] = useState(false);

    const handleLogin = async (e: React.FormEvent) => {
        e.preventDefault();
        setError('');
        setLoading(true);

        try {
            const res = await api.post('/auth/login', { email, password });
            login(res.data.token, res.data.user);
            navigate('/');
        } catch (err: any) {
            setError(err.response?.data?.error || 'Failed to login');
        } finally {
            setLoading(false);
        }
    };

    return (
        <div className="min-h-screen flex items-center justify-center bg-background text-foreground font-sans dark relative overflow-hidden">
            {/* Background glow effects */}
            <div className="absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 w-[600px] h-[600px] bg-primary/20 rounded-full blur-[120px] pointer-events-none" />

            <div className="w-full max-w-md p-8 glass rounded-2xl border border-border/50 shadow-2xl relative z-10">
                <div className="flex flex-col items-center mb-10">
                    <Logo size="lg" className="mb-2" />
                    <p className="text-muted text-sm mt-2">Enter your credentials to access Atlas</p>
                </div>

                {error && (
                    <div className="bg-red-500/10 border border-red-500/20 text-red-500 p-3 rounded-lg text-sm text-center mb-4">
                        {error}
                    </div>
                )}

                <form onSubmit={handleLogin} className="flex flex-col gap-5">
                    <div className="space-y-2">
                        <label className="text-xs font-medium uppercase tracking-wide text-muted">Email</label>
                        <input
                            type="email"
                            className="w-full bg-secondary/50 border border-border rounded-lg px-4 py-3 text-sm focus:outline-none focus:ring-2 focus:ring-primary/50 transition-all placeholder:text-muted/50"
                            placeholder="name@example.com"
                            value={email}
                            onChange={(e) => setEmail(e.target.value)}
                        />
                    </div>

                    <div className="space-y-2">
                        <div className="flex items-center justify-between">
                            <label className="text-xs font-medium uppercase tracking-wide text-muted">Password</label>
                            <a href="#" className="text-xs text-primary hover:text-primary/80">Forgot?</a>
                        </div>
                        <input
                            type="password"
                            className="w-full bg-secondary/50 border border-border rounded-lg px-4 py-3 text-sm focus:outline-none focus:ring-2 focus:ring-primary/50 transition-all placeholder:text-muted/50"
                            placeholder="••••••••"
                            value={password}
                            onChange={(e) => setPassword(e.target.value)}
                        />
                    </div>

                    <button
                        type="submit"
                        disabled={loading}
                        className="w-full bg-white text-black font-semibold py-3 rounded-lg hover:bg-gray-200 transition-colors mt-2 disabled:opacity-50"
                    >
                        {loading ? 'Signing in...' : 'Sign In'}
                    </button>
                </form>

                <p className="text-center text-xs text-muted mt-8">
                    By signing in, you agree to our <a href="#" className="underline hover:text-white">Terms of Service</a>
                </p>
            </div>
        </div>
    );
}
