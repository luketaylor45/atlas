import { useState, type FormEvent } from 'react';
import { LogOut } from 'lucide-react';
import { PasswordInput } from '../../components/AuthShell';
import ThemeToggle from '../../components/ThemeToggle';
import { Alert, Button, Card, CardHeader, Field, PageHeader, useToast } from '../../components/ui';
import { useAuth } from '../../context/auth';
import api from '../../lib/api';
import { errorMessage, errorStatus } from '../../lib/errors';
import { PASSWORD_HINT, validateConfirmation, validatePassword } from '../auth/account';

interface PasswordErrors {
    current?: string;
    next?: string;
    confirmation?: string;
}

function ChangePasswordCard() {
    const toast = useToast();
    const [current, setCurrent] = useState('');
    const [next, setNext] = useState('');
    const [confirmation, setConfirmation] = useState('');
    const [fieldErrors, setFieldErrors] = useState<PasswordErrors>({});
    const [error, setError] = useState('');
    const [submitting, setSubmitting] = useState(false);

    const handleSubmit = async (event: FormEvent) => {
        event.preventDefault();
        setError('');

        const problems: PasswordErrors = {
            current: current ? undefined : 'Enter your current password.',
            next: validatePassword(next) ?? (next === current ? 'Choose a password different from your current one.' : undefined),
            confirmation: validateConfirmation(next, confirmation),
        };
        setFieldErrors(problems);
        if (problems.current || problems.next || problems.confirmation) return;

        setSubmitting(true);
        try {
            await api.post('/auth/password', { current_password: current, new_password: next });
            setCurrent('');
            setNext('');
            setConfirmation('');
            toast.success('Your password has been changed.');
        } catch (err) {
            if (errorStatus(err) === 403) {
                setFieldErrors({ current: errorMessage(err, 'Your current password is incorrect.') });
            } else {
                setError(errorMessage(err, 'Could not change your password. Please try again.'));
            }
        } finally {
            setSubmitting(false);
        }
    };

    return (
        <Card>
            <CardHeader
                title="Change password"
                description="This password is also used for SFTP access to your servers."
            />
            <form onSubmit={handleSubmit} noValidate className="space-y-4">
                {error && <Alert tone="danger">{error}</Alert>}

                <Field label="Current password" error={fieldErrors.current}>
                    {(field) => (
                        <PasswordInput
                            {...field}
                            name="current-password"
                            autoComplete="current-password"
                            value={current}
                            onChange={(event) => setCurrent(event.target.value)}
                        />
                    )}
                </Field>

                <Field label="New password" hint={PASSWORD_HINT} error={fieldErrors.next}>
                    {(field) => (
                        <PasswordInput
                            {...field}
                            name="new-password"
                            autoComplete="new-password"
                            value={next}
                            onChange={(event) => setNext(event.target.value)}
                        />
                    )}
                </Field>

                <Field label="Confirm new password" error={fieldErrors.confirmation}>
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

                <Button type="submit" variant="primary" loading={submitting}>
                    Change password
                </Button>
            </form>
        </Card>
    );
}

export default function SettingsPage() {
    const { user, logout } = useAuth();

    return (
        <>
            <PageHeader title="Settings" description="Your account, password and how Atlas looks on this device." />

            <div className="grid grid-cols-1 items-start gap-6 lg:grid-cols-2">
                <div className="space-y-6">
                    <Card>
                        <CardHeader title="Account" description="An administrator can change your username or role." />
                        <dl className="grid grid-cols-1 gap-4 text-sm sm:grid-cols-2">
                            <div className="min-w-0">
                                <dt className="text-muted">Username</dt>
                                <dd className="mt-1 break-words font-medium text-foreground">{user?.username}</dd>
                            </div>
                            <div>
                                <dt className="text-muted">Role</dt>
                                <dd className="mt-1 font-medium text-foreground">{user?.is_admin ? 'Administrator' : 'Member'}</dd>
                            </div>
                        </dl>
                    </Card>

                    <Card>
                        <CardHeader title="Appearance" description="Choose a light or dark theme. System follows your device's setting." />
                        <ThemeToggle showLabels />
                    </Card>

                    <Card>
                        <CardHeader title="Sign out" description="End your session on this device." className="mb-4" />
                        <Button variant="outline" icon={<LogOut size={16} aria-hidden />} onClick={logout}>
                            Sign out
                        </Button>
                    </Card>
                </div>

                <ChangePasswordCard />
            </div>
        </>
    );
}
