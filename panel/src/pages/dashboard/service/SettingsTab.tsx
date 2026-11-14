import { useState } from 'react';
import api from '../../../lib/api';
import { errorMessage } from '../../../lib/errors';
import type { Service } from '../../../lib/types';
import { Button, Card, CardHeader, useConfirm, useToast } from '../../../components/ui';

interface SettingsTabProps {
    service: Service;
    /** Starts a fresh installation. Resolves true when it was accepted. */
    onReinstall: () => Promise<boolean>;
    /** Reloads the server after an action. */
    onChanged: () => void | Promise<void>;
}

export function SettingsTab({ service, onReinstall, onChanged }: SettingsTabProps) {
    const toast = useToast();
    const confirm = useConfirm();
    const [busy, setBusy] = useState<'rebuild' | 'reinstall' | null>(null);

    const rebuild = async () => {
        const confirmed = await confirm({
            title: 'Rebuild container?',
            description: `This recreates the container for "${service.name}" from its current settings. Your files are kept. If the server is running it is stopped and started again.`,
            confirmLabel: 'Rebuild container',
        });
        if (!confirmed) return;

        setBusy('rebuild');
        try {
            await api.post(`/services/${service.uuid}/rebuild`);
            toast.success('The container was rebuilt. Your files were not changed.');
        } catch (err) {
            toast.error(errorMessage(err, 'Could not rebuild the container.'));
        } finally {
            setBusy(null);
            void onChanged();
        }
    };

    const reinstall = async () => {
        const confirmed = await confirm({
            title: 'Reinstall server?',
            description: `Every file on "${service.name}" is permanently deleted, including worlds, configuration and anything you uploaded, and the server is installed again from scratch. This cannot be undone.`,
            confirmLabel: 'Reinstall server',
            danger: true,
            requireText: service.name,
        });
        if (!confirmed) return;

        setBusy('reinstall');
        await onReinstall();
        setBusy(null);
    };

    return (
        <div className="space-y-6">
            <Card>
                <CardHeader
                    title="Rebuild container"
                    description="Recreates the server's container from its current settings and keeps all files. Useful when the server will not start after a settings change, or to pick up a new Docker image."
                    actions={
                        <Button variant="outline" loading={busy === 'rebuild'} disabled={busy !== null} onClick={rebuild}>
                            Rebuild container
                        </Button>
                    }
                    className="mb-0"
                />
            </Card>

            <Card className="border-danger/30">
                <CardHeader title="Danger zone" description="These actions delete data and cannot be undone." />
                <div className="flex flex-wrap items-start justify-between gap-3 border-t border-border pt-5">
                    <div className="min-w-0 max-w-xl">
                        <p className="text-sm font-medium text-foreground">Reinstall server</p>
                        <p className="mt-1 text-sm text-muted">
                            Wipes every file on the server and runs the installation again. Download anything you want to keep first.
                        </p>
                    </div>
                    <Button variant="danger-outline" loading={busy === 'reinstall'} disabled={busy !== null} onClick={reinstall}>
                        Reinstall server
                    </Button>
                </div>
            </Card>
        </div>
    );
}
