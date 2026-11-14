import { useEffect, useState, type FormEvent } from 'react';
import { Link, Navigate, useLocation, useNavigate } from 'react-router-dom';
import AuthShell, { PasswordInput } from '../../components/AuthShell';
import { Alert, Button, Field, Input } from '../../components/ui';
import { useAuth } from '../../context/auth';
import api from '../../lib/api';
import { errorMessage } from '../../lib/errors';
import { useSetupStatus, type AuthResponse } from './account';

interface LoginLocationState {
    from?: { pathname?: string; search?: string; hash?: string };
}

/** Where to go after signing in: the page RequireAuth interrupted, or the dashboard. */
function destination(state: unknown): string {
    const from = (state as LoginLocationState | null)?.from;
    if (!from?.pathname || !from.pathname.startsWith('/') || from.pathname === '/login') return '/';
    return `${from.pathname}${from.search ?? ''}${from.hash ?? ''}`;
}

export default function LoginPage() {
    const navigate = useNavigate();
    const location = useLocation();
    const { user, login } = useAuth();
    const { status } = useSetupStatus();

    const [username, setUsername] = useState('');
    const [password, setPassword] = useState('');
    const [rememberMe, setRememberMe] = useState(false);
    const [fieldErrors, setFieldErrors] = useState<{ username?: string; password?: string }>({});
    const [error, setError] = useState('');
    const [submitting, setSubmitting] = useState(false);

    const setupRequired = status?.setup_required === true;
    useEffect(() => {
        if (setupRequired) navigate('/setup', { replace: true });
    }, [setupRequired, navigate]);

    // Covers both "already signed in" and the moment a sign-in succeeds.
    if (user) {
        return <Navigate to={destination(location.state)} replace />;
    }

    const handleSubmit = async (event: FormEvent) => {
        event.preventDefault();
        setError('');

        const problems = {
            username: username.trim() ? undefined : 'Enter your username.',
            password: password ? undefined : 'Enter your password.',
        };
        setFieldErrors(problems);
        if (problems.username || problems.password) return;

        setSubmitting(true);
        try {
            const res = await api.post<AuthResponse>('/auth/login', { username: username.trim(), password });
            login(res.data.token, res.data.user, rememberMe);
        } catch (err) {
            setError(errorMessage(err, 'Could not sign you in. Please try again.'));
        } finally {
            setSubmitting(false);
        }
    };

    return (
        <AuthShell
            title="Sign in"
            description="Use your Atlas account to manage your servers."
            footer={
                status?.registration_enabled && (
                    <>
                        New here?{' '}
                        <Link to="/register" className="font-medium text-primary-text hover:underline">
                            Create an account
                        </Link>
                    </>
                )
            }
        >
            <form onSubmit={handleSubmit} noValidate className="space-y-4">
                {error && <Alert tone="danger">{error}</Alert>}

                <Field label="Username" error={fieldErrors.username}>
                    {(field) => (
                        <Input
                            {...field}
                            name="username"
                            autoComplete="username"
                            autoCapitalize="none"
                            spellCheck={false}
                            autoFocus
                            value={username}
                            onChange={(event) => setUsername(event.target.value)}
                        />
                    )}
                </Field>

                <Field label="Password" error={fieldErrors.password}>
                    {(field) => (
                        <PasswordInput
                            {...field}
                            name="password"
                            autoComplete="current-password"
                            value={password}
                            onChange={(event) => setPassword(event.target.value)}
                        />
                    )}
                </Field>

                <label className="flex w-fit cursor-pointer select-none items-center gap-2.5 text-sm text-foreground">
                    <input
                        type="checkbox"
                        name="remember"
                        checked={rememberMe}
                        onChange={(event) => setRememberMe(event.target.checked)}
                        className="h-4 w-4 accent-primary"
                    />
                    Keep me signed in
                </label>

                <Button type="submit" variant="primary" size="lg" fullWidth loading={submitting}>
                    Sign in
                </Button>
            </form>
        </AuthShell>
    );
}
