import { useState } from 'react';
import { isAxiosError } from 'axios';
import { useNavigate, Link } from 'react-router-dom';
import Logo from '../../components/Logo';
import api from '../../lib/api';
import { useAuth } from '../../context/auth';

export default function LoginPage() {
    const navigate = useNavigate();
    const { login } = useAuth();
    const [username, setUsername] = useState('');
    const [password, setPassword] = useState('');
    const [rememberMe, setRememberMe] = useState(false);
    const [error, setError] = useState('');
    const [loading, setLoading] = useState(false);

    const handleLogin = async (e: React.FormEvent) => {
        e.preventDefault();
        setError('');
        setLoading(true);

        try {
            const res = await api.post('/auth/login', { username, password });
            login(res.data.token, res.data.user, rememberMe);
            navigate('/');
        } catch (err: unknown) {
            const message = isAxiosError<{ error?: string }>(err)
                ? err.response?.data?.error
                : undefined;
            setError(message || 'Failed to login');
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
                    <div role="alert" className="bg-red-500/10 border border-red-500/20 text-red-500 p-3 rounded-lg text-sm text-center mb-4">
                        {error}
                    </div>
                )}

                <form onSubmit={handleLogin} className="flex flex-col gap-5">
                    <div className="space-y-2">
                        <label htmlFor="username" className="text-xs font-medium uppercase tracking-wide text-muted">Username</label>
                        <input
                            id="username"
                            name="username"
                            type="text"
                            autoComplete="username"
                            required
                            className="w-full bg-secondary/50 border border-border rounded-lg px-4 py-3 text-sm focus:outline-none focus:ring-2 focus:ring-primary/50 transition-all placeholder:text-muted/50"
                            placeholder="e.g. admin"
                            value={username}
                            onChange={(e) => setUsername(e.target.value)}
                        />
                    </div>

                    <div className="space-y-2">
                        <div className="flex items-center justify-between">
                            <label htmlFor="password" className="text-xs font-medium uppercase tracking-wide text-muted">Password</label>
                            <a href="#" className="text-xs text-primary hover:text-primary/80">Forgot?</a>
                        </div>
                        <input
                            id="password"
                            name="password"
                            type="password"
                            autoComplete="current-password"
                            required
                            className="w-full bg-secondary/50 border border-border rounded-lg px-4 py-3 text-sm focus:outline-none focus:ring-2 focus:ring-primary/50 transition-all placeholder:text-muted/50"
                            placeholder="••••••••"
                            value={password}
                            onChange={(e) => setPassword(e.target.value)}
                        />
                    </div>

                    <label htmlFor="remember-me" className="flex w-fit cursor-pointer select-none items-center gap-2.5 text-xs font-medium text-muted">
                        <input
                            id="remember-me"
                            name="remember-me"
                            type="checkbox"
                            checked={rememberMe}
                            disabled={loading}
                            onChange={(e) => setRememberMe(e.target.checked)}
                            className="h-4 w-4 rounded border-border bg-secondary accent-primary focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/50 focus-visible:ring-offset-2 focus-visible:ring-offset-background disabled:cursor-not-allowed disabled:opacity-50"
                        />
                        <span>Remember me on this device</span>
                    </label>

                    <button
                        type="submit"
                        disabled={loading}
                        className="w-full bg-white text-black font-semibold py-3 rounded-lg hover:bg-gray-200 transition-colors mt-2 disabled:opacity-50"
                        aria-busy={loading}
                    >
                        {loading ? 'Signing in...' : 'Sign In'}
                    </button>
                </form>

                <div className="text-center mt-8 space-y-4">
                    <p className="text-xs text-muted">
                        Don't have an account? <Link to="/register" className="text-primary hover:text-primary/80 font-bold">Sign Up</Link>
                    </p>
                    <p className="text-xs text-muted">
                        By signing in, you agree to our <a href="#" className="underline hover:text-white">Terms of Service</a>
                    </p>
                </div>
            </div>
        </div>
    );
}
