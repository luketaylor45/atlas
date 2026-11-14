import { useState, type FormEvent } from 'react';
import { Save } from 'lucide-react';
import api from '../../../lib/api';
import { useAuth } from '../../../context/auth';
import { errorMessage } from '../../../lib/errors';
import { parseJSON, type EggVariable, type Service } from '../../../lib/types';
import { Alert, Badge, Button, Card, CardHeader, EmptyState, Field, Input, Table, Td, Textarea, Th, Tr } from '../../../components/ui';

interface EnvironmentResponse {
    startup_command?: string;
    environment?: string;
}

interface StartupTabProps {
    service: Service;
    /** Receives the saved values so the page shows them straight away. */
    onSaved: (saved: { startup_command?: string; environment?: string }) => void | Promise<void>;
}

function savedEnvironment(service: Service): Record<string, string> {
    const parsed = parseJSON<unknown>(service.environment, {});
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) return {};
    const result: Record<string, string> = {};
    for (const [key, value] of Object.entries(parsed)) {
        if (value !== null && value !== undefined) result[key] = String(value);
    }
    return result;
}

export function StartupTab({ service, onSaved }: StartupTabProps) {
    const isAdmin = Boolean(useAuth().user?.is_admin);

    // Only the user's edits are kept here; everything else follows the server,
    // so a background refresh never overwrites what is being typed.
    const [commandEdit, setCommandEdit] = useState<string | null>(null);
    const [edits, setEdits] = useState<Record<string, string>>({});
    const [saving, setSaving] = useState(false);
    const [commandError, setCommandError] = useState('');
    const [result, setResult] = useState<{ tone: 'success' | 'danger'; text: string } | null>(null);

    const savedCommand = service.startup_command || service.egg?.startup_command || '';
    const environment = savedEnvironment(service);
    const variables: EggVariable[] = (service.egg?.variables ?? []).filter((variable) => isAdmin || variable.user_viewable);

    const currentValue = (variable: EggVariable) => environment[variable.environment_variable] ?? variable.default_value ?? '';
    const canEdit = (variable: EggVariable) => isAdmin || variable.user_editable;

    const command = commandEdit ?? savedCommand;
    const commandChanged = isAdmin && commandEdit !== null && commandEdit !== savedCommand;

    // Only what changed, and only what this user is allowed to change.
    const changed: Record<string, string> = {};
    for (const variable of variables) {
        const key = variable.environment_variable;
        if (canEdit(variable) && key in edits && edits[key] !== currentValue(variable)) {
            changed[key] = edits[key];
        }
    }
    const dirty = commandChanged || Object.keys(changed).length > 0;

    const submit = async (event: FormEvent) => {
        event.preventDefault();
        if (!dirty || saving) return;

        if (commandChanged && !command.trim()) {
            setCommandError('The startup command cannot be empty.');
            return;
        }

        setSaving(true);
        setResult(null);
        try {
            const res = await api.post<EnvironmentResponse>(`/services/${service.uuid}/environment`, {
                environment: JSON.stringify(changed),
                ...(commandChanged ? { startup_command: command } : {}),
            });
            await onSaved({
                environment: res.data.environment,
                startup_command: res.data.startup_command || (commandChanged ? command : undefined),
            });
            setEdits({});
            setCommandEdit(null);
            setResult({ tone: 'success', text: 'Startup settings saved. They apply the next time the server starts.' });
        } catch (err) {
            setResult({ tone: 'danger', text: errorMessage(err, 'Could not save the startup settings.') });
            // The node can refuse an update Atlas has already stored, so show what is saved now.
            await onSaved({});
        } finally {
            setSaving(false);
        }
    };

    const ports = service.ports ?? [];

    return (
        <form onSubmit={submit} className="space-y-6">
            {result && <Alert tone={result.tone}>{result.text}</Alert>}

            <Card>
                <CardHeader title="Startup command" description="The command that runs when the server starts." />
                {isAdmin ? (
                    <Field label="Command" error={commandError || undefined}>
                        {(field) => (
                            <Textarea
                                {...field}
                                value={command}
                                onChange={(event) => {
                                    setCommandEdit(event.target.value);
                                    setCommandError('');
                                    setResult(null);
                                }}
                                rows={4}
                                spellCheck={false}
                                className="font-mono text-xs"
                            />
                        )}
                    </Field>
                ) : (
                    <>
                        <pre className="overflow-x-auto whitespace-pre-wrap break-all rounded-control border border-border bg-secondary p-3 font-mono text-xs leading-relaxed text-foreground">
                            {savedCommand || 'Not set'}
                        </pre>
                        <p className="mt-2 text-xs text-muted">Only an administrator can change the startup command.</p>
                    </>
                )}
            </Card>

            <Card>
                <CardHeader title="Variables" description="Settings the server reads when it starts." />
                {variables.length === 0 ? (
                    <EmptyState title="No variables" description="This egg has no settings to change here." className="py-8" />
                ) : (
                    <div className="grid grid-cols-1 gap-x-6 gap-y-5 md:grid-cols-2">
                        {variables.map((variable) => {
                            const key = variable.environment_variable;
                            const editable = canEdit(variable);
                            const aside = !editable
                                ? 'Locked'
                                : isAdmin && !variable.user_viewable
                                  ? 'Hidden from users'
                                  : isAdmin && !variable.user_editable
                                    ? 'Locked for users'
                                    : undefined;
                            return (
                                <Field
                                    key={variable.id ?? key}
                                    label={variable.name || key}
                                    aside={aside}
                                    hint={
                                        <>
                                            <code className="break-all font-mono">{key}</code>
                                            {variable.description && <span className="mt-1 block">{variable.description}</span>}
                                        </>
                                    }
                                >
                                    {(field) => (
                                        <Input
                                            {...field}
                                            type={variable.input_type === 'password' ? 'password' : 'text'}
                                            autoComplete={variable.input_type === 'password' ? 'new-password' : 'off'}
                                            value={edits[key] ?? currentValue(variable)}
                                            disabled={!editable}
                                            onChange={(event) => {
                                                setEdits((current) => ({ ...current, [key]: event.target.value }));
                                                setResult(null);
                                            }}
                                        />
                                    )}
                                </Field>
                            );
                        })}
                    </div>
                )}
            </Card>

            <Card flush>
                <CardHeader
                    title="Container"
                    description="Set by an administrator. Shown here for reference."
                    className="mb-0 px-5 pt-5 sm:px-6 sm:pt-6"
                />
                <div className="px-5 py-4 sm:px-6">
                    <p className="text-sm text-muted">Docker image</p>
                    <p className="mt-1 break-all font-mono text-sm text-foreground">{service.docker_image || 'Not set'}</p>
                </div>
                {ports.length > 0 && (
                    <div className="border-t border-border">
                        <Table>
                            <thead>
                                <tr>
                                    <Th>Port name</Th>
                                    <Th>Port</Th>
                                    <Th>Protocol</Th>
                                    <Th>Variable</Th>
                                </tr>
                            </thead>
                            <tbody>
                                {ports.map((port) => (
                                    <Tr key={port.id ?? `${port.env}-${port.port}`}>
                                        <Td>
                                            <span className="flex items-center gap-2">
                                                {port.name || (port.primary ? 'Game port' : 'Port')}
                                                {port.primary && <Badge tone="primary">Primary</Badge>}
                                            </span>
                                        </Td>
                                        <Td className="tabular font-mono">{port.port}</Td>
                                        <Td>{port.protocol ? port.protocol.toUpperCase() : '—'}</Td>
                                        <Td className="font-mono text-muted">{port.env || '—'}</Td>
                                    </Tr>
                                ))}
                            </tbody>
                        </Table>
                    </div>
                )}
            </Card>

            <div className="flex flex-wrap items-center justify-between gap-3">
                <p className="text-sm text-muted">
                    {dirty ? 'You have unsaved changes.' : 'Changes apply the next time the server starts.'}
                </p>
                <Button type="submit" variant="primary" loading={saving} disabled={!dirty} icon={<Save size={16} aria-hidden />}>
                    Save changes
                </Button>
            </div>
        </form>
    );
}
