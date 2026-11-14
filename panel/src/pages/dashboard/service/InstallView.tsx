import { useEffect, useRef, useState } from 'react';
import { RotateCcw, X } from 'lucide-react';
import api from '../../../lib/api';
import { errorMessage } from '../../../lib/errors';
import type { Service } from '../../../lib/types';
import { usePolling } from '../../../lib/usePolling';
import { Alert, Button, Card, CardHeader, ProgressBar, useConfirm, useToast } from '../../../components/ui';
import { stripAnsi, type ServiceAccess } from './shared';

// Keep the view bounded; the newest output matters most.
const MAX_TRANSCRIPT = 200000;

interface InstallLogResponse {
    content?: string;
    offset?: number;
    running?: boolean;
}

interface InstallViewProps {
    service: Service;
    access: ServiceAccess;
    /** Starts a fresh installation. Resolves true when it was accepted. */
    onReinstall: () => Promise<boolean>;
    /** Reloads the server after an action. */
    onChanged: () => void | Promise<void>;
}

/** Shown in place of the tabs while the server is installing or its installation failed. */
export function InstallView({ service, access, onReinstall, onChanged }: InstallViewProps) {
    const uuid = service.uuid;
    const toast = useToast();
    const confirm = useConfirm();

    const failed = service.status === 'installation_failed';
    const progress = Math.min(100, Math.max(0, Number(service.installation_progress) || 0));

    const [transcript, setTranscript] = useState('');
    const [cancelling, setCancelling] = useState(false);
    const [retrying, setRetrying] = useState(false);
    const offset = useRef(0);
    const box = useRef<HTMLPreElement>(null);
    // Follow new output unless the user has scrolled up to read.
    const follow = useRef(true);

    // The node keeps the full output of the install script; it is followed
    // here by byte offset.
    usePolling(
        async () => {
            try {
                const res = await api.get<InstallLogResponse>(`/services/${uuid}/install/log`, { params: { offset: offset.current } });
                const next = Number(res.data.offset) || 0;
                const chunk = String(res.data.content || '');
                if (next < offset.current) {
                    // A new installation started: the transcript was reset.
                    setTranscript(chunk);
                } else if (chunk) {
                    setTranscript((current) => (current + chunk).slice(-MAX_TRANSCRIPT));
                }
                offset.current = next;
            } catch {
                // The node may be briefly unreachable; the next poll tries again.
            }
        },
        failed ? 5000 : 1500,
        access.canConsole,
    );

    useEffect(() => {
        const node = box.current;
        if (node && follow.current) node.scrollTop = node.scrollHeight;
    }, [transcript]);

    const cancel = async () => {
        setCancelling(true);
        try {
            await api.post(`/services/${uuid}/install/cancel`);
        } catch (err) {
            toast.error(errorMessage(err, 'Could not cancel the installation.'));
        } finally {
            setCancelling(false);
            void onChanged();
        }
    };

    const retry = async () => {
        const confirmed = await confirm({
            title: 'Retry installation?',
            description: `This runs the installer for "${service.name}" again from the start. Every file on the server is wiped first.`,
            confirmLabel: 'Retry installation',
            danger: true,
        });
        if (!confirmed) return;

        setRetrying(true);
        const started = await onReinstall();
        if (started) {
            offset.current = 0;
            follow.current = true;
            setTranscript('');
        }
        setRetrying(false);
    };

    const output = stripAnsi(transcript);
    const placeholder = !access.canConsole
        ? 'You do not have permission to view the installation log.'
        : failed
          ? 'No installer output was recorded.'
          : 'Waiting for the installer to start…';

    return (
        <div className="space-y-4">
            {failed && (
                <Alert
                    tone="danger"
                    title="Installation failed"
                    actions={
                        access.canPower && (
                            <Button variant="outline" size="sm" loading={retrying} onClick={retry} icon={<RotateCcw size={16} aria-hidden />}>
                                Retry installation
                            </Button>
                        )
                    }
                >
                    <p className="break-words">
                        {service.installation_stage || 'The installer stopped before the server could be created.'}
                    </p>
                    {!access.canPower && <p className="mt-1">Ask the server owner to retry the installation.</p>}
                </Alert>
            )}

            <Card>
                <CardHeader
                    title={failed ? 'Installation failed' : 'Installing…'}
                    description={
                        failed
                            ? `"${service.name}" was not installed. The installer output is below.`
                            : `Setting up "${service.name}"${service.node?.name ? ` on ${service.node.name}` : ''}. This can take a few minutes; you can leave this page.`
                    }
                    actions={
                        !failed &&
                        access.canPower && (
                            <Button variant="outline" loading={cancelling} onClick={cancel} icon={<X size={16} aria-hidden />}>
                                Cancel installation
                            </Button>
                        )
                    }
                />

                <div className="space-y-2">
                    <div className="flex items-baseline justify-between gap-4 text-sm">
                        <span className="min-w-0 break-words text-muted">
                            {failed ? 'Stopped' : service.installation_stage || 'Waiting to start…'}
                        </span>
                        <span className="tabular shrink-0 font-medium text-foreground">{Math.round(progress)}%</span>
                    </div>
                    <ProgressBar value={progress} tone={failed ? 'danger' : 'primary'} label="Installation progress" />
                </div>

                <pre
                    ref={box}
                    tabIndex={0}
                    aria-label="Installer output"
                    onScroll={(event) => {
                        const node = event.currentTarget;
                        follow.current = node.scrollHeight - node.scrollTop - node.clientHeight < 24;
                    }}
                    className="mt-5 h-80 overflow-y-auto whitespace-pre-wrap break-all rounded-control border border-terminal-border bg-terminal p-4 font-mono text-xs leading-relaxed text-terminal-foreground sm:h-[26rem]"
                >
                    {output || <span className="text-terminal-muted">{placeholder}</span>}
                </pre>
            </Card>
        </div>
    );
}
