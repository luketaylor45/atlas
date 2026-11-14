import { useEffect, useState, type FormEvent } from 'react';
import { Link, Navigate, useNavigate } from 'react-router-dom';
import AuthShell, { PasswordInput } from '../../components/AuthShell';
import { Alert, Button, Field, Input, LoadingState } from '../../components/ui';
import { useAuth } from '../../context/auth';
import api from '../../lib/api';
import { errorMessage, errorStatus } from '../../lib/errors';
import {
    PASSWORD_HINT,
    USERNAME_HINT,
    useSetupStatus,
    validateConfirmation,
    validatePassword,
    validateUsername,
    type AuthResponse,
} from './account';

interface FieldErrors {
    username?: string;
    password?: string;
    confirmation?: string;
}

const signInLink = (
    <Link to="/login" className="font-medium text-primary-text hover:underline">
        Sign in
    </Link>
);

export default function RegisterPage() {
    const navigate = useNavigate();
    const { user, login } = useAuth();
    const { status, loading } = useSetupStatus();

    const [username, setUsername] = useState('');
    const [password, setPassword] = useState('');
    const [confirmation, setConfirmation] = useState('');
    const [fieldErrors, setFieldErrors] = useState<FieldErrors>({});
    const [error, setError] = useState('');
    const [submitting, setSubmitting] = useState(false);
    // Set when the API itself refuses a registration (403).
    const [refused, setRefused] = useState(false);

    const setupRequired = status?.setup_required === true;
    useEffect(() => {
        if (setupRequired) navigate('/setup', { replace: true });
    }, [setupRequired, navigate]);

    if (user) {
        return <Navigate to="/" replace />;
    }

    if (loading) {
        return (
            <AuthShell title="Create an account">
                <LoadingState className="py-8" />
            </AuthShell>
        );
    }

    if (refused || status?.registration_enabled === false) {
        return (
            <AuthShell
                title="Registration is closed"
                description="Accounts on this panel are created by an administrator. Ask yours for an account, then sign in with the details they give you."
            >
                <Link
                    to="/login"
                    className="inline-flex h-11 w-full items-center justify-center rounded-control bg-primary px-5 text-sm font-medium text-primary-foreground shadow-card transition-colors hover:bg-primary-hover"
                >
                    Back to sign in
                </Link>
            </AuthShell>
        );
    }

    const handleSubmit = async (event: FormEvent) => {
        event.preventDefault();
        setError('');

        const name = username.trim();
        const problems: FieldErrors = {
            username: validateUsername(name),
            password: validatePassword(password),
            confirmation: validateConfirmation(password, confirmation),
        };
        setFieldErrors(problems);
        if (problems.username || problems.password || problems.confirmation) return;

        setSubmitting(true);
        try {
            const res = await api.post<AuthResponse>('/auth/register', { username: name, password });
            login(res.data.token, res.data.user);
        } catch (err) {
            if (errorStatus(err) === 403) {
                setRefused(true);
            } else if (errorStatus(err) === 409) {
                setFieldErrors({ username: errorMessage(err, 'That username is already taken.') });
            } else {
                setError(errorMessage(err, 'Could not create your account. Please try again.'));
            }
        } finally {
            setSubmitting(false);
        }
    };

    return (
        <AuthShell
            title="Create an account"
            description="Choose a username and password for Atlas."
            footer={<>Already have an account? {signInLink}</>}
        >
            <form onSubmit={handleSubmit} noValidate className="space-y-4">
                {error && <Alert tone="danger">{error}</Alert>}

                <Field label="Username" hint={USERNAME_HINT} error={fieldErrors.username}>
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

                <Field label="Password" hint={PASSWORD_HINT} error={fieldErrors.password}>
                    {(field) => (
                        <PasswordInput
                            {...field}
                            name="password"
                            autoComplete="new-password"
                            value={password}
                            onChange={(event) => setPassword(event.target.value)}
                        />
                    )}
                </Field>

                <Field label="Confirm password" error={fieldErrors.confirmation}>
                    {(field) => (
                        <PasswordInput
                            {...field}
                            name="confirm-password"
                            autoComplete="new-password"
                            value={confirmation}
                            onChange={(event) => setConfirmation(event.target.value)}
                        />
                    )}
                </Field>

                <Button type="submit" variant="primary" size="lg" fullWidth loading={submitting}>
                    Create account
                </Button>
            </form>
        </AuthShell>
    );
}
