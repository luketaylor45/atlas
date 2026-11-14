import { useEffect, useState, type FormEvent } from 'react';
import { Navigate, useNavigate } from 'react-router-dom';
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
} from '../auth/account';

interface FieldErrors {
    username?: string;
    password?: string;
    confirmation?: string;
}

/** First-run page: creates the administrator account. */
export default function SetupPage() {
    const navigate = useNavigate();
    const { user, login } = useAuth();
    const { status, loading } = useSetupStatus();

    const [username, setUsername] = useState('');
    const [password, setPassword] = useState('');
    const [confirmation, setConfirmation] = useState('');
    const [fieldErrors, setFieldErrors] = useState<FieldErrors>({});
    const [error, setError] = useState('');
    const [submitting, setSubmitting] = useState(false);

    const alreadySetUp = status?.setup_required === false;
    useEffect(() => {
        if (alreadySetUp) navigate('/login', { replace: true });
    }, [alreadySetUp, navigate]);

    if (user) {
        return <Navigate to="/" replace />;
    }

    if (loading || alreadySetUp) {
        return (
            <AuthShell title="Set up Atlas">
                <LoadingState className="py-8" />
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
            const res = await api.post<Partial<AuthResponse>>('/auth/setup', { username: name, password });
            if (res.data.token && res.data.user) {
                login(res.data.token, res.data.user);
            } else {
                navigate('/login', { replace: true });
            }
        } catch (err) {
            if (errorStatus(err) === 403) {
                // Someone else finished setup first.
                navigate('/login', { replace: true });
                return;
            }
            setError(errorMessage(err, 'Could not finish setup. Please try again.'));
        } finally {
            setSubmitting(false);
        }
    };

    return (
        <AuthShell
            title="Set up Atlas"
            description="Create the administrator account. It has full control of this panel: nodes, servers and other people's accounts."
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
                    Create administrator account
                </Button>
            </form>
        </AuthShell>
    );
}
