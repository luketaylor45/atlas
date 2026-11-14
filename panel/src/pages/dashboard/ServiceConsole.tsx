import { useRef, useState } from 'react';
import { Navigate, useParams, useSearchParams } from 'react-router-dom';
import { Folder, History, Rocket, Settings, SquareTerminal, Users } from 'lucide-react';
import api from '../../lib/api';
import { errorMessage } from '../../lib/errors';
import { isInstallState } from '../../lib/status';
import type { ServiceDetails } from '../../lib/types';
import { usePolling } from '../../lib/usePolling';
import { Alert, Button, EmptyState, LoadingState, Tabs, useConfirm, useToast, type TabItem } from '../../components/ui';
import FileManager from './FileManager';
import ServiceActivityTab from './ServiceActivityTab';
import ServiceUsersTab from './ServiceUsersTab';
import { ConsoleTab, type ConsoleHandle } from './service/ConsoleTab';
import { InstallView } from './service/InstallView';
import { ServerHeader } from './service/ServerHeader';
import { SettingsTab } from './service/SettingsTab';
import { StartupTab } from './service/StartupTab';
import { serviceAccess, type PowerAction } from './service/shared';

type TabId = 'console' | 'files' | 'startup' | 'users' | 'activity' | 'settings';

const powerNotices: Record<PowerAction, string> = {
    start: 'Starting server…',
    stop: 'Stopping server…',
    restart: 'Restarting server…',
    kill: 'Killing server process…',
};

export default function ServiceConsolePage() {
    const { uuid } = useParams();
    if (!uuid) return <Navigate to="/services" replace />;
    // Keyed by server so nothing (console output, form edits) carries over to another one.
    return <ServicePage key={uuid} uuid={uuid} />;
}

function ServicePage({ uuid }: { uuid: string }) {
    const toast = useToast();
    const confirm = useConfirm();
    const [searchParams, setSearchParams] = useSearchParams();
    const consoleRef = useRef<ConsoleHandle>(null);

    const [details, setDetails] = useState<ServiceDetails | null>(null);
    const [loadError, setLoadError] = useState<string | null>(null);
    const [retrying, setRetrying] = useState(false);
    const [powerBusy, setPowerBusy] = useState<PowerAction | null>(null);

    const load = async () => {
        try {
            const res = await api.get<ServiceDetails>(`/services/${uuid}`);
            setDetails(res.data);
            setLoadError(null);
        } catch (err) {
            setLoadError(errorMessage(err, 'Could not load this server.'));
        }
    };

    const status = details?.service.status;
    usePolling(load, status === 'installing' ? 1500 : 5000);

    const tryAgain = async () => {
        setRetrying(true);
        await load();
        setRetrying(false);
    };

    if (!details) {
        return (
            <>
                <ServerHeader service={null} canPower={false} busy={null} onPower={() => undefined} />
                {loadError ? (
                    <Alert
                        tone="danger"
                        title="This server could not be loaded"
                        actions={
                            <Button size="sm" variant="outline" loading={retrying} onClick={tryAgain}>
                                Try again
                            </Button>
                        }
                    >
                        {loadError}
                    </Alert>
                ) : (
                    <LoadingState label="Loading server…" />
                )}
            </>
        );
    }

    const service = details.service;
    const access = serviceAccess(details);
    const installState = isInstallState(service.status);

    const power = async (action: PowerAction) => {
        if (action === 'kill') {
            const confirmed = await confirm({
                title: 'Kill server?',
                description: `"${service.name}" is stopped immediately, without a chance to save. Recent progress that was not saved may be lost.`,
                confirmLabel: 'Kill server',
                danger: true,
            });
            if (!confirmed) return;
        }

        setPowerBusy(action);
        consoleRef.current?.notice(powerNotices[action]);
        try {
            await api.post(`/services/${uuid}/power`, { action });
            // Give the container a moment, then pick up its new output stream.
            setTimeout(() => consoleRef.current?.reconnect(), 500);
        } catch (err) {
            const message = errorMessage(err, `Could not ${action} the server.`);
            toast.error(message);
            consoleRef.current?.notice(message, 'error');
        } finally {
            setPowerBusy(null);
            void load();
        }
    };

    const reinstall = async (): Promise<boolean> => {
        try {
            await api.post(`/services/${uuid}/reinstall`);
            setDetails((current) =>
                current
                    ? {
                          ...current,
                          service: {
                              ...current.service,
                              status: 'installing',
                              installation_stage: 'Reinstallation queued',
                              installation_progress: 0,
                          },
                      }
                    : current,
            );
            return true;
        } catch (err) {
            toast.error(errorMessage(err, 'Could not start the installation.'));
            return false;
        } finally {
            void load();
        }
    };

    const startupSaved = async (saved: { startup_command?: string; environment?: string }) => {
        setDetails((current) =>
            current
                ? {
                      ...current,
                      service: {
                          ...current.service,
                          startup_command: saved.startup_command ?? current.service.startup_command,
                          environment: saved.environment ?? current.service.environment,
                      },
                  }
                : current,
        );
        await load();
    };

    const allTabs: (TabItem<TabId> & { show: boolean })[] = [
        { id: 'console', label: 'Console', icon: <SquareTerminal size={16} aria-hidden />, show: access.canConsole },
        { id: 'files', label: 'Files', icon: <Folder size={16} aria-hidden />, show: access.canFiles },
        { id: 'startup', label: 'Startup', icon: <Rocket size={16} aria-hidden />, show: access.canStartup },
        { id: 'users', label: 'Users', icon: <Users size={16} aria-hidden />, show: access.isOwner },
        { id: 'activity', label: 'Activity', icon: <History size={16} aria-hidden />, show: access.isOwner },
        { id: 'settings', label: 'Settings', icon: <Settings size={16} aria-hidden />, show: access.canPower },
    ];
    const tabs: TabItem<TabId>[] = allTabs.filter((tab) => tab.show).map(({ id, label, icon }) => ({ id, label, icon }));

    // The tab lives in the URL so a refresh keeps it; an unavailable one falls back to the first.
    const requested = searchParams.get('tab');
    const activeTab = tabs.find((tab) => tab.id === requested)?.id ?? tabs[0]?.id;

    const selectTab = (id: TabId) => {
        const next = new URLSearchParams(searchParams);
        next.set('tab', id);
        setSearchParams(next, { replace: true });
    };

    return (
        <>
            <ServerHeader service={service} canPower={access.canPower} busy={powerBusy} onPower={power} />

            <div className="space-y-6">
                {loadError && (
                    <Alert tone="warning" title="This page may be out of date">
                        {loadError}
                    </Alert>
                )}
                {service.is_suspended && (
                    <Alert tone="warning" title="This server is suspended">
                        It cannot be started until an administrator lifts the suspension.
                    </Alert>
                )}

                {installState ? (
                    <InstallView service={service} access={access} onReinstall={reinstall} onChanged={load} />
                ) : !activeTab ? (
                    <EmptyState
                        title="Nothing to show here"
                        description="Your access to this server does not include anything in the panel. Ask the server owner to change your permissions."
                    />
                ) : (
                    <>
                        <Tabs tabs={tabs} value={activeTab} onChange={selectTab} label="Server sections" />
                        <div role="tabpanel">
                            {activeTab === 'console' && <ConsoleTab ref={consoleRef} service={service} access={access} />}
                            {activeTab === 'files' && <FileManager uuid={uuid} service={service} />}
                            {activeTab === 'startup' && <StartupTab service={service} onSaved={startupSaved} />}
                            {activeTab === 'users' && <ServiceUsersTab uuid={uuid} />}
                            {activeTab === 'activity' && <ServiceActivityTab uuid={uuid} />}
                            {activeTab === 'settings' && <SettingsTab service={service} onReinstall={reinstall} onChanged={load} />}
                        </div>
                    </>
                )}
            </div>
        </>
    );
}
