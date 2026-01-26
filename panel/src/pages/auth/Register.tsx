import { useState } from 'react';
import { useNavigate, Link } from 'react-router-dom';
import Logo from '../../components/Logo';
import api from '../../lib/api';
import { useAuth } from '../../context/AuthContext';

export default function RegisterPage() {
    const navigate = useNavigate();
    const { login } = useAuth();
    const [username, setUsername] = useState('');
    const [password, setPassword] = useState('');
    const [confirmPassword, setConfirmPassword] = useState('');
    const [error, setError] = useState('');
    const [loading, setLoading] = useState(false);

    const handleRegister = async (e: React.FormEvent) => {
        e.preventDefault();
        setError('');

        if (password !== confirmPassword) {
            setError('Passwords do not match');
            return;
        }

        if (password.length < 8) {
            setError('Password must be at least 8 characters');
            return;
        }

        setLoading(true);

        try {
            const res = await api.post('/auth/register', { username, password });
            login(res.data.token, res.data.user);
            navigate('/');
        } catch (err: any) {
            setError(err.response?.data?.error || 'Failed to register');
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
                    <p className="text-muted text-sm mt-2">Create your account to join Atlas</p>
                </div>

                {error && (
                    <div className="bg-red-500/10 border border-red-500/20 text-red-500 p-3 rounded-lg text-sm text-center mb-4">
                        {error}
                    </div>
                )}

                <form onSubmit={handleRegister} className="flex flex-col gap-5">
                    <div className="space-y-2">
                        <label className="text-xs font-medium uppercase tracking-wide text-muted">Username</label>
                        <input
                            type="text"
                            className="w-full bg-secondary/50 border border-border rounded-lg px-4 py-3 text-sm focus:outline-none focus:ring-2 focus:ring-primary/50 transition-all placeholder:text-muted/50"
                            placeholder="Pick a unique username"
                            value={username}
                            onChange={(e) => setUsername(e.target.value)}
                            required
                        />
                    </div>

                    <div className="space-y-2">
                        <label className="text-xs font-medium uppercase tracking-wide text-muted">Password</label>
                        <input
                            type="password"
                            className="w-full bg-secondary/50 border border-border rounded-lg px-4 py-3 text-sm focus:outline-none focus:ring-2 focus:ring-primary/50 transition-all placeholder:text-muted/50"
                            placeholder="At least 8 characters"
                            value={password}
                            onChange={(e) => setPassword(e.target.value)}
                            required
                        />
                    </div>

                    <div className="space-y-2">
                        <label className="text-xs font-medium uppercase tracking-wide text-muted">Confirm Password</label>
                        <input
                            type="password"
                            className="w-full bg-secondary/50 border border-border rounded-lg px-4 py-3 text-sm focus:outline-none focus:ring-2 focus:ring-primary/50 transition-all placeholder:text-muted/50"
                            placeholder="Repeat your password"
                            value={confirmPassword}
                            onChange={(e) => setConfirmPassword(e.target.value)}
                            required
                        />
                    </div>

                    <button
                        type="submit"
                        disabled={loading}
                        className="w-full bg-white text-black font-semibold py-3 rounded-lg hover:bg-gray-200 transition-colors mt-2 disabled:opacity-50"
                    >
                        {loading ? 'Creating Account...' : 'Sign Up'}
                    </button>
                </form>

                <div className="text-center mt-8 space-y-4">
                    <p className="text-xs text-muted">
                        Already have an account? <Link to="/login" className="text-primary hover:text-primary/80 font-bold">Sign In</Link>
                    </p>
                    <p className="text-xs text-muted">
                        By signing up, you agree to our <a href="#" className="underline hover:text-white">Terms of Service</a>
                    </p>
                </div>
            </div>
        </div>
    );
}
