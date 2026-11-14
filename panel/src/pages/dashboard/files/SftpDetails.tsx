import { Button, CopyButton, Modal } from '../../../components/ui';
import { useAuth } from '../../../context/auth';
import type { FileManagerService } from './util';

function Row({ label, value }: { label: string; value: string }) {
    return (
        <div className="flex items-center justify-between gap-3 px-4 py-3">
            <div className="min-w-0">
                <dt className="text-xs text-muted">{label}</dt>
                <dd className="mt-0.5 break-all font-mono text-sm text-foreground">{value || '—'}</dd>
            </div>
            {value && <CopyButton value={value} label={`Copy ${label.toLowerCase()}`} />}
        </div>
    );
}

/** How to reach this server's files with an SFTP client. */
export default function SftpDetails({ open, onClose, service }: { open: boolean; onClose: () => void; service: FileManagerService }) {
    const { user } = useAuth();

    const host = service.node?.address ?? '';
    const port = String(service.node?.sftp_port || '2022');
    const username = service.uuid && user?.username ? `${service.uuid.slice(0, 8)}.${user.username}` : '';

    return (
        <Modal
            open={open}
            onClose={onClose}
            size="sm"
            title="SFTP details"
            description="Connect with an SFTP client such as FileZilla or WinSCP to move large files or whole folders."
            footer={<Button onClick={onClose}>Done</Button>}
        >
            <dl className="divide-y divide-border rounded-card border border-border">
                <Row label="Host" value={host} />
                <Row label="Port" value={port} />
                <Row label="Username" value={username} />
                <div className="px-4 py-3">
                    <dt className="text-xs text-muted">Password</dt>
                    <dd className="mt-0.5 text-sm text-foreground">Your Atlas account password</dd>
                </div>
            </dl>
            {host && username && (
                <div className="mt-3 flex items-center justify-between gap-3 text-xs text-muted">
                    <span className="min-w-0 break-all font-mono">{`sftp://${username}@${host}:${port}`}</span>
                    <CopyButton value={`sftp://${username}@${host}:${port}`} label="Copy address" />
                </div>
            )}
        </Modal>
    );
}
