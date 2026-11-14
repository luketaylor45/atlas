import { useId, useState, type FormEvent } from 'react';
import { Pencil, Plus, Search, Trash2, Users as UsersIcon } from 'lucide-react';
import {
    Alert,
    Badge,
    Button,
    Card,
    EmptyState,
    Field,
    IconButton,
    Input,
    LoadingState,
    Modal,
    PageHeader,
    Switch,
    Table,
    Td,
    Th,
    Tr,
    useConfirm,
    useToast,
} from '../../components/ui';
import { useAuth } from '../../context/auth';
import api from '../../lib/api';
import { errorMessage, errorStatus } from '../../lib/errors';
import { formatDateTime, formatRelative } from '../../lib/format';
import type { User } from '../../lib/types';
import { usePolling } from '../../lib/usePolling';

interface UserFormErrors {
    username?: string;
    password?: string;
}

function usernameError(username: string): string | undefined {
    if (username.length < 3 || username.length > 32) return 'Use between 3 and 32 characters.';
    if (!/^[A-Za-z0-9._-]+$/.test(username)) return 'Use only letters, numbers, dots, dashes and underscores.';
    return undefined;
}

function passwordError(password: string): string | undefined {
    if (password.length < 8) return 'Use at least 8 characters.';
    // The API counts bytes, so accented characters and emoji count for more than one.
    if (new TextEncoder().encode(password).length > 72) return 'Use at most 72 characters.';
    return undefined;
}

interface UserModalProps {
    /** The user being edited, or null to create one. */
    user: User | null;
    isSelf: boolean;
    onClose: () => void;
    onSaved: (user: User) => void;
}

function UserModal({ user, isSelf, onClose, onSaved }: UserModalProps) {
    const toast = useToast();
    const formId = useId();
    const [username, setUsername] = useState(user?.username ?? '');
    const [password, setPassword] = useState('');
    const [isAdmin, setIsAdmin] = useState(user?.is_admin ?? false);
    const [errors, setErrors] = useState<UserFormErrors>({});
    const [formError, setFormError] = useState<string | null>(null);
    const [saving, setSaving] = useState(false);

    const submit = async (event: FormEvent<HTMLFormElement>) => {
        event.preventDefault();
        if (saving) return;

        const name = username.trim();
        const problems: UserFormErrors = {};
        const nameProblem = usernameError(name);
        if (nameProblem) problems.username = nameProblem;
        if (!user || password) {
            const passwordProblem = passwordError(password);
            if (passwordProblem) problems.password = passwordProblem;
        }
        setErrors(problems);
        if (problems.username || problems.password) return;

        setSaving(true);
        setFormError(null);
        try {
            if (user) {
                const changes: { username?: string; password?: string; is_admin?: boolean } = {};
                if (name !== user.username) changes.username = name;
                if (password) changes.password = password;
                if (isAdmin !== user.is_admin) changes.is_admin = isAdmin;
                if (Object.keys(changes).length === 0) {
                    onClose();
                    return;
                }
                const res = await api.put<User>(`/admin/users/${user.id}`, changes);
                toast.success(`Saved ${res.data.username}`);
                onSaved(res.data);
            } else {
                const res = await api.post<User>('/admin/users', { username: name, password, is_admin: isAdmin });
                toast.success(`Created ${res.data.username}`);
                onSaved(res.data);
            }
        } catch (err) {
            const message = errorMessage(err, user ? 'Could not save the user.' : 'Could not create the user.');
            const status = errorStatus(err);
            if ((status === 409 || status === 400) && /username/i.test(message)) {
                setErrors({ username: message });
            } else if (status === 400 && /password/i.test(message)) {
                setErrors({ password: message });
            } else {
                setFormError(message);
            }
            setSaving(false);
        }
    };

    return (
        <Modal
            open
            onClose={onClose}
            title={user ? `Edit ${user.username}` : 'Create user'}
            dismissible={!saving}
            footer={
                <>
                    <Button variant="ghost" onClick={onClose} disabled={saving}>
                        Cancel
                    </Button>
                    <Button variant="primary" type="submit" form={formId} loading={saving}>
                        {user ? 'Save changes' : 'Create user'}
                    </Button>
                </>
            }
        >
            <form id={formId} onSubmit={submit} noValidate className="space-y-4">
                {formError && <Alert tone="danger">{formError}</Alert>}

                <Field
                    label="Username"
                    error={errors.username}
                    hint="3 to 32 characters: letters, numbers, dots, dashes and underscores."
                >
                    {(field) => (
                        <Input
                            {...field}
                            value={username}
                            onChange={(event) => {
                                setUsername(event.target.value);
                                setErrors((current) => ({ ...current, username: undefined }));
                            }}
                            autoComplete="off"
                            autoCapitalize="none"
                            spellCheck={false}
                            maxLength={32}
                        />
                    )}
                </Field>

                <Field
                    label={user ? 'New password' : 'Password'}
                    aside={user ? 'Optional' : undefined}
                    error={errors.password}
                    hint={user ? 'Leave blank to keep the current password.' : '8 to 72 characters.'}
                >
                    {(field) => (
                        <Input
                            {...field}
                            type="password"
                            value={password}
                            onChange={(event) => {
                                setPassword(event.target.value);
                                setErrors((current) => ({ ...current, password: undefined }));
                            }}
                            autoComplete="new-password"
                        />
                    )}
                </Field>

                <div className="rounded-control border border-border px-3 py-2">
                    <Switch
                        checked={isAdmin}
                        onChange={setIsAdmin}
                        disabled={isSelf}
                        label="Administrator"
                        description={
                            isSelf
                                ? 'You cannot remove your own administrator access. Ask another administrator to do it.'
                                : 'Can manage every server, node, egg, user and announcement. Members only see servers they own or are invited to.'
                        }
                    />
                </div>
            </form>
        </Modal>
    );
}

type ModalState = { mode: 'create' } | { mode: 'edit'; user: User } | null;

export default function AdminUsersPage() {
    const { user: me } = useAuth();
    const toast = useToast();
    const confirm = useConfirm();
    const [users, setUsers] = useState<User[] | null>(null);
    const [error, setError] = useState<string | null>(null);
    const [search, setSearch] = useState('');
    const [modal, setModal] = useState<ModalState>(null);
    const [deletingId, setDeletingId] = useState<number | null>(null);

    const load = async () => {
        try {
            const res = await api.get<User[]>('/admin/users');
            setUsers(Array.isArray(res.data) ? res.data : []);
            setError(null);
        } catch (err) {
            setError(errorMessage(err, 'Could not load users.'));
        }
    };

    usePolling(load, 30000);

    const retry = () => {
        setError(null);
        void load();
    };

    const deleteUser = async (user: User) => {
        const confirmed = await confirm({
            title: `Delete ${user.username}?`,
            description:
                'Their account is removed and they can no longer sign in. Any access they were given to other people\'s servers is removed too. This cannot be undone.',
            confirmLabel: 'Delete user',
            danger: true,
        });
        if (!confirmed) return;

        setDeletingId(user.id);
        try {
            await api.delete(`/admin/users/${user.id}`);
            setUsers((current) => current?.filter((item) => item.id !== user.id) ?? current);
            toast.success(`Deleted ${user.username}`);
        } catch (err) {
            toast.error(errorMessage(err, 'Could not delete the user.'));
        } finally {
            setDeletingId(null);
        }
    };

    const saved = (user: User) => {
        setUsers((current) => {
            if (!current) return current;
            return current.some((item) => item.id === user.id)
                ? current.map((item) => (item.id === user.id ? user : item))
                : [...current, user];
        });
        setModal(null);
    };

    const header = (
        <PageHeader
            title="Users"
            description="Everyone who can sign in to the panel."
            actions={
                <Button variant="primary" icon={<Plus size={16} aria-hidden />} onClick={() => setModal({ mode: 'create' })}>
                    Create user
                </Button>
            }
        />
    );

    const modalElement = modal && (
        <UserModal
            key={modal.mode === 'edit' ? modal.user.id : 'create'}
            user={modal.mode === 'edit' ? modal.user : null}
            isSelf={modal.mode === 'edit' && modal.user.id === me?.id}
            onClose={() => setModal(null)}
            onSaved={saved}
        />
    );

    if (!users) {
        return (
            <>
                {header}
                {error ? (
                    <Alert tone="danger" title="Could not load users" actions={<Button size="sm" onClick={retry}>Try again</Button>}>
                        {error}
                    </Alert>
                ) : (
                    <LoadingState label="Loading users…" />
                )}
                {modalElement}
            </>
        );
    }

    const term = search.trim().toLowerCase();
    const visible = term ? users.filter((user) => (user.username || '').toLowerCase().includes(term)) : users;

    return (
        <>
            {header}

            <div className="space-y-4">
                {error && (
                    <Alert tone="warning" title="Could not refresh" actions={<Button size="sm" onClick={retry}>Try again</Button>}>
                        {error} Showing the last list received.
                    </Alert>
                )}

                <div className="relative w-full sm:max-w-xs">
                    <Search size={16} className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-subtle" aria-hidden />
                    <Input
                        type="search"
                        value={search}
                        onChange={(event) => setSearch(event.target.value)}
                        placeholder="Search by username"
                        aria-label="Search users by username"
                        className="pl-9"
                    />
                </div>

                <Card flush>
                    {users.length === 0 ? (
                        <EmptyState
                            icon={<UsersIcon size={18} aria-hidden />}
                            title="No users yet"
                            description="Create an account for each person who should manage servers."
                            action={<Button onClick={() => setModal({ mode: 'create' })}>Create user</Button>}
                        />
                    ) : visible.length === 0 ? (
                        <EmptyState
                            icon={<Search size={18} aria-hidden />}
                            title="No users match your search"
                            description={`Nobody has a username containing "${search.trim()}".`}
                            action={<Button onClick={() => setSearch('')}>Clear search</Button>}
                        />
                    ) : (
                        <Table>
                            <thead>
                                <tr>
                                    <Th>Username</Th>
                                    <Th>Role</Th>
                                    <Th>Created</Th>
                                    <Th className="text-right">Actions</Th>
                                </tr>
                            </thead>
                            <tbody>
                                {visible.map((user) => {
                                    const isSelf = user.id === me?.id;
                                    return (
                                        <Tr key={user.id}>
                                            <Td>
                                                <div className="flex items-center gap-2">
                                                    <span className="max-w-64 truncate font-medium text-foreground" title={user.username}>
                                                        {user.username}
                                                    </span>
                                                    {isSelf && <Badge>You</Badge>}
                                                </div>
                                            </Td>
                                            <Td>
                                                <Badge tone={user.is_admin ? 'primary' : 'neutral'}>
                                                    {user.is_admin ? 'Administrator' : 'Member'}
                                                </Badge>
                                            </Td>
                                            <Td className="whitespace-nowrap text-muted" title={formatDateTime(user.created_at)}>
                                                {formatRelative(user.created_at)}
                                            </Td>
                                            <Td>
                                                <div className="flex items-center justify-end gap-1">
                                                    <IconButton
                                                        label={`Edit ${user.username}`}
                                                        size="sm"
                                                        disabled={deletingId === user.id}
                                                        onClick={() => setModal({ mode: 'edit', user })}
                                                    >
                                                        <Pencil size={16} aria-hidden />
                                                    </IconButton>
                                                    {!isSelf && (
                                                        <IconButton
                                                            label={`Delete ${user.username}`}
                                                            size="sm"
                                                            className="hover:bg-danger/10 hover:text-danger"
                                                            loading={deletingId === user.id}
                                                            onClick={() => void deleteUser(user)}
                                                        >
                                                            <Trash2 size={16} aria-hidden />
                                                        </IconButton>
                                                    )}
                                                </div>
                                            </Td>
                                        </Tr>
                                    );
                                })}
                            </tbody>
                        </Table>
                    )}
                </Card>
            </div>

            {modalElement}
        </>
    );
}
