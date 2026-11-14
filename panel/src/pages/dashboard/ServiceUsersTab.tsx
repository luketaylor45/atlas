import { useEffect, useState, type FormEvent } from 'react';
import { Trash2, UserPlus, Users } from 'lucide-react';
import api from '../../lib/api';
import { errorMessage, errorStatus } from '../../lib/errors';
import { formatDateTime, formatRelative } from '../../lib/format';
import type { ServicePermissions, ServiceUser } from '../../lib/types';
import {
    Alert,
    Button,
    Card,
    CardHeader,
    CopyButton,
    EmptyState,
    Field,
    IconButton,
    Input,
    LoadingState,
    Switch,
    useConfirm,
    useToast,
} from '../../components/ui';

type PermissionKey = keyof ServicePermissions;

const permissionList: { key: PermissionKey; label: string; description: string }[] = [
    { key: 'can_view_console', label: 'View console', description: 'See the live console output and resource use.' },
    { key: 'can_send_commands', label: 'Send commands', description: 'Type commands into the server console.' },
    { key: 'can_control_power', label: 'Control power', description: 'Start, stop, restart and kill the server, and rebuild or reinstall it.' },
    { key: 'can_manage_files', label: 'Manage files', description: 'Browse, edit, upload and delete files in the panel.' },
    { key: 'can_edit_startup', label: 'Edit startup', description: 'Change the startup variables.' },
    { key: 'can_access_sftp', label: 'SFTP access', description: 'Connect to the server files with an SFTP client.' },
    { key: 'can_manage_backups', label: 'Manage backups', description: 'Create, download, restore and delete backups.' },
    {
        key: 'can_manage_schedules',
        label: 'Manage schedules',
        description: 'Create, change and run schedules. They can only schedule actions they are allowed to do themselves.',
    },
];

const defaultPermissions: ServicePermissions = {
    can_view_console: true,
    can_send_commands: false,
    can_manage_files: false,
    can_edit_startup: false,
    can_control_power: false,
    can_access_sftp: false,
    can_manage_backups: false,
    can_manage_schedules: false,
};

function pickPermissions(user: ServicePermissions): ServicePermissions {
    return {
        can_view_console: Boolean(user.can_view_console),
        can_send_commands: Boolean(user.can_send_commands),
        can_manage_files: Boolean(user.can_manage_files),
        can_edit_startup: Boolean(user.can_edit_startup),
        can_control_power: Boolean(user.can_control_power),
        can_access_sftp: Boolean(user.can_access_sftp),
        can_manage_backups: Boolean(user.can_manage_backups),
        can_manage_schedules: Boolean(user.can_manage_schedules),
    };
}

export default function ServiceUsersTab({ uuid }: { uuid: string }) {
    const toast = useToast();
    const confirm = useConfirm();

    const [users, setUsers] = useState<ServiceUser[] | null>(null);
    const [loadError, setLoadError] = useState<string | null>(null);
    const [reloadKey, setReloadKey] = useState(0);
    const [savingId, setSavingId] = useState<number | null>(null);

    const [username, setUsername] = useState('');
    const [usernameError, setUsernameError] = useState('');
    const [permissions, setPermissions] = useState<ServicePermissions>(defaultPermissions);
    const [adding, setAdding] = useState(false);

    useEffect(() => {
        let cancelled = false;
        void (async () => {
            try {
                const res = await api.get<ServiceUser[]>(`/services/${uuid}/users`);
                if (cancelled) return;
                setUsers(Array.isArray(res.data) ? res.data : []);
                setLoadError(null);
            } catch (err) {
                if (!cancelled) setLoadError(errorMessage(err, 'Could not load the sub-users.'));
            }
        })();
        return () => {
            cancelled = true;
        };
    }, [uuid, reloadKey]);

    const sftpPrefix = uuid.substring(0, 8);
    const nameOf = (user: ServiceUser) => user.user?.username || `User #${user.user_id}`;

    const togglePermission = async (user: ServiceUser, key: PermissionKey, value: boolean) => {
        const next = { ...pickPermissions(user), [key]: value };
        setSavingId(user.user_id);
        // Show the change at once; put it back if the save fails.
        setUsers((current) => current?.map((entry) => (entry.user_id === user.user_id ? { ...entry, ...next } : entry)) ?? current);
        try {
            await api.put(`/services/${uuid}/users/${user.user_id}`, next);
        } catch (err) {
            setUsers((current) => current?.map((entry) => (entry.user_id === user.user_id ? { ...entry, [key]: !value } : entry)) ?? current);
            toast.error(errorMessage(err, `Could not update the permissions for ${nameOf(user)}.`));
        } finally {
            setSavingId(null);
        }
    };

    const remove = async (user: ServiceUser) => {
        const confirmed = await confirm({
            title: `Remove ${nameOf(user)}?`,
            description: 'They lose access to this server in the panel and over SFTP straight away. Their Atlas account is not affected.',
            confirmLabel: 'Remove sub-user',
            danger: true,
        });
        if (!confirmed) return;

        setSavingId(user.user_id);
        try {
            await api.delete(`/services/${uuid}/users/${user.user_id}`);
            setUsers((current) => current?.filter((entry) => entry.user_id !== user.user_id) ?? current);
            toast.success(`${nameOf(user)} was removed.`);
        } catch (err) {
            toast.error(errorMessage(err, `Could not remove ${nameOf(user)}.`));
        } finally {
            setSavingId(null);
        }
    };

    const add = async (event: FormEvent) => {
        event.preventDefault();
        const name = username.trim();
        if (!name) {
            setUsernameError('Enter the Atlas username of the person to add.');
            return;
        }

        setAdding(true);
        setUsernameError('');
        try {
            const res = await api.post<ServiceUser>(`/services/${uuid}/users`, { username: name, ...permissions });
            setUsers((current) => [...(current ?? []), res.data]);
            setUsername('');
            setPermissions(defaultPermissions);
            toast.success(`${res.data.user?.username || name} was added.`);
        } catch (err) {
            const message = errorMessage(err, 'Could not add the sub-user.');
            const status = errorStatus(err);
            // These are all about the username that was typed.
            if (status === 404 || status === 409 || status === 403 || status === 400) {
                setUsernameError(message);
            } else {
                toast.error(message);
            }
        } finally {
            setAdding(false);
        }
    };

    return (
        <div className="space-y-6">
            <Card flush>
                <CardHeader
                    title="Sub-users"
                    description="People you have given access to this server, and what each of them may do. Changes save as you make them."
                    className="mb-0 px-5 pt-5 sm:px-6 sm:pt-6"
                />

                {loadError && (
                    <div className="px-5 pt-4 sm:px-6">
                        <Alert
                            tone="danger"
                            title="The sub-users could not be loaded"
                            actions={
                                <Button size="sm" variant="outline" onClick={() => setReloadKey((key) => key + 1)}>
                                    Try again
                                </Button>
                            }
                        >
                            {loadError}
                        </Alert>
                    </div>
                )}

                {users === null ? (
                    loadError ? (
                        <div className="pb-5" />
                    ) : (
                        <LoadingState label="Loading sub-users…" />
                    )
                ) : users.length === 0 ? (
                    <EmptyState
                        icon={<Users size={18} aria-hidden />}
                        title="No sub-users yet"
                        description="Add someone below by their Atlas username to share this server with them."
                    />
                ) : (
                    <ul className="mt-5 divide-y divide-border border-t border-border">
                        {users.map((user) => {
                            const name = nameOf(user);
                            const sftpUsername = user.user?.username ? `${sftpPrefix}.${user.user.username}` : '';
                            const busy = savingId === user.user_id;
                            return (
                                <li key={user.id} className="px-5 py-5 sm:px-6">
                                    <div className="flex items-start justify-between gap-3">
                                        <div className="min-w-0">
                                            <p className="truncate text-sm font-medium text-foreground">{name}</p>
                                            <p className="mt-0.5 text-xs text-muted" title={formatDateTime(user.created_at)}>
                                                Added {formatRelative(user.created_at)}
                                            </p>
                                            {user.can_access_sftp && sftpUsername && (
                                                <p className="mt-1 flex flex-wrap items-center gap-1 text-xs text-muted">
                                                    SFTP username
                                                    <code className="break-all font-mono text-foreground">{sftpUsername}</code>
                                                    <CopyButton value={sftpUsername} label="Copy SFTP username" />
                                                </p>
                                            )}
                                        </div>
                                        <IconButton
                                            label={`Remove ${name}`}
                                            variant="danger-outline"
                                            size="sm"
                                            disabled={busy}
                                            onClick={() => remove(user)}
                                        >
                                            <Trash2 size={16} aria-hidden />
                                        </IconButton>
                                    </div>
                                    <div className="mt-3 grid grid-cols-1 gap-x-8 gap-y-1 sm:grid-cols-2 lg:grid-cols-3">
                                        {permissionList.map((permission) => (
                                            <Switch
                                                key={permission.key}
                                                label={permission.label}
                                                checked={Boolean(user[permission.key])}
                                                disabled={busy}
                                                onChange={(value) => togglePermission(user, permission.key, value)}
                                            />
                                        ))}
                                    </div>
                                </li>
                            );
                        })}
                    </ul>
                )}
            </Card>

            <Card>
                <CardHeader title="Add a sub-user" description="The person needs an Atlas account first. Enter their username exactly." />
                <form onSubmit={add} className="space-y-5">
                    <Field label="Username" error={usernameError || undefined} className="max-w-sm">
                        {(field) => (
                            <Input
                                {...field}
                                value={username}
                                onChange={(event) => {
                                    setUsername(event.target.value);
                                    setUsernameError('');
                                }}
                                placeholder="Their Atlas username"
                                autoComplete="off"
                                autoCapitalize="off"
                                spellCheck={false}
                                maxLength={32}
                            />
                        )}
                    </Field>

                    <fieldset>
                        <legend className="text-sm font-medium text-foreground">Permissions</legend>
                        <div className="mt-2 grid grid-cols-1 gap-x-8 gap-y-2 md:grid-cols-2">
                            {permissionList.map((permission) => (
                                <Switch
                                    key={permission.key}
                                    label={permission.label}
                                    description={permission.description}
                                    checked={permissions[permission.key]}
                                    onChange={(value) => setPermissions((current) => ({ ...current, [permission.key]: value }))}
                                />
                            ))}
                        </div>
                    </fieldset>

                    <Alert tone="info" title="SFTP uses their own Atlas password">
                        There is no separate SFTP password to set. With SFTP access switched on, they sign in with the username{' '}
                        <code className="break-all font-mono text-foreground">{sftpPrefix}.their-username</code> and the password of their Atlas
                        account.
                    </Alert>

                    <div className="flex justify-end">
                        <Button type="submit" variant="primary" loading={adding} icon={<UserPlus size={16} aria-hidden />}>
                            Add sub-user
                        </Button>
                    </div>
                </form>
            </Card>
        </div>
    );
}
