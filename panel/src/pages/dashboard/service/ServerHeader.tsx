import { Link } from 'react-router-dom';
import { ArrowLeft, Play, RotateCw, Skull, Square } from 'lucide-react';
import { gameIdentity } from '../../../lib/gameIdentity';
import { isInstallState } from '../../../lib/status';
import type { Service } from '../../../lib/types';
import { GameIcon } from '../../../components/GameCover';
import { Button, CopyButton, PageHeader, StatusBadge } from '../../../components/ui';
import { isLiveStatus, type PowerAction } from './shared';

interface ServerHeaderProps {
    /** Null while the server is still loading or could not be loaded. */
    service: Service | null;
    canPower: boolean;
    /** The power action in flight, if any. */
    busy: PowerAction | null;
    onPower: (action: PowerAction) => void;
}

/** Back link, name, status, connect address and the power buttons. */
export function ServerHeader({ service, canPower, busy, onPower }: ServerHeaderProps) {
    const back = (
        <Link to="/services" className="inline-flex items-center gap-1.5 transition-colors hover:text-foreground">
            <ArrowLeft size={16} aria-hidden />
            Back to servers
        </Link>
    );

    if (!service) {
        return <PageHeader eyebrow={back} title="Server" />;
    }

    const status = service.status;
    const installing = isInstallState(status);
    const up = status === 'running' || status === 'starting';
    const offline = !installing && !isLiveStatus(status);

    const primaryPort = service.ports?.find((port) => port.primary)?.port ?? service.port;
    const address = service.node?.address ? `${service.node.address}:${primaryPort}` : '';

    const denied = canPower ? undefined : 'You do not have permission to control power on this server';
    const button = (action: PowerAction, enabled: boolean) => ({
        onClick: () => onPower(action),
        loading: busy === action,
        disabled: !canPower || !enabled || busy !== null,
        title: denied,
    });

    return (
        <PageHeader
            eyebrow={back}
            title={
                <span className="flex items-center gap-3">
                    <GameIcon identity={gameIdentity(service)} size={18} className="h-9 w-9 rounded-control" />
                    <span className="min-w-0 truncate">{service.name}</span>
                    <StatusBadge status={status} suspended={service.is_suspended} className="shrink-0" />
                </span>
            }
            description={
                <span className="flex flex-wrap items-center gap-x-3 gap-y-1">
                    {address && (
                        <span className="inline-flex min-w-0 items-center gap-1">
                            <span className="break-all font-mono text-foreground">{address}</span>
                            <CopyButton value={address} label="Copy address" />
                        </span>
                    )}
                    {service.egg?.name && <span>{service.egg.name}</span>}
                    {service.node?.name && <span>{service.node.name}</span>}
                </span>
            }
            actions={
                <>
                    <Button variant="outline" icon={<Play size={16} aria-hidden />} {...button('start', offline)}>
                        Start
                    </Button>
                    <Button variant="outline" icon={<RotateCw size={16} aria-hidden />} {...button('restart', up)}>
                        Restart
                    </Button>
                    <Button variant="outline" icon={<Square size={16} aria-hidden />} {...button('stop', up)}>
                        Stop
                    </Button>
                    <Button
                        variant="danger-outline"
                        icon={<Skull size={16} aria-hidden />}
                        {...button('kill', up || status === 'stopping')}
                    >
                        Kill
                    </Button>
                </>
            }
        />
    );
}
