import { useEffect, useId, useState, type FormEvent } from 'react';
import api from '../../../lib/api';
import { errorMessage, errorStatus } from '../../../lib/errors';
import { formatCpu, formatMB } from '../../../lib/format';
import { eggImages, type Node, type Service } from '../../../lib/types';
import ResourceSlider from '../../../components/ResourceSlider';
import { Alert, Button, Field, Input, Modal, Select } from '../../../components/ui';
import { resourceLimits } from './resourceLimits';
import { portNumber, wholeNumber } from './ServiceForm';

export interface ServiceEditModalProps {
    service: Service;
    onClose: () => void;
    /** Called after a successful save, with the node's warning if it had one. */
    onSaved: (service: Service, warning?: string) => void;
}

type FieldName = 'name' | 'memory' | 'cpu' | 'disk' | 'port' | 'backups';

/** The most backups a server may be allowed to keep. */
const MAX_BACKUP_LIMIT = 50;

interface UpdateResponse {
    service: Service;
    warning?: string;
}

/**
 * Edits a server's name, limits, primary port, image and how many backups it
 * may keep. Render it only while it should be open, keyed by the server, so
 * the form starts from its values.
 */
export function ServiceEditModal({ service, onClose, onSaved }: ServiceEditModalProps) {
    const formId = useId();
    const [name, setName] = useState(service.name);
    const [memory, setMemory] = useState(String(service.memory));
    const [cpu, setCpu] = useState(String(service.cpu));
    const [disk, setDisk] = useState(String(service.disk));
    const [port, setPort] = useState(String(service.port));
    const [image, setImage] = useState(service.docker_image);
    const [backups, setBackups] = useState(String(service.backup_limit));
    const [errors, setErrors] = useState<Partial<Record<FieldName, string>>>({});
    const [formError, setFormError] = useState('');
    const [saving, setSaving] = useState(false);

    // The node's current allocation, for the sliders. The dialog works without
    // it (the server still checks the limits when saving).
    const [node, setNode] = useState<Node | null>(null);
    useEffect(() => {
        let cancelled = false;
        api.get<Node[] | null>('/admin/nodes')
            .then((res) => {
                if (!cancelled) setNode((res.data ?? []).find((item) => item.id === service.node_id) ?? null);
            })
            .catch(() => undefined);
        return () => {
            cancelled = true;
        };
    }, [service.node_id]);
    const limits = resourceLimits(node, { memory: service.memory, disk: service.disk, cpu: service.cpu });

    // The current image stays selectable even if the egg no longer lists it.
    const listed = eggImages(service.egg);
    const images = service.docker_image && !listed.includes(service.docker_image) ? [service.docker_image, ...listed] : listed;

    const portChanged = port.trim() !== String(service.port);
    const imageChanged = image !== service.docker_image;

    const submit = async (event: FormEvent) => {
        event.preventDefault();
        if (saving) return;

        const next: Partial<Record<FieldName, string>> = {};
        const trimmedName = name.trim();
        const memoryValue = wholeNumber(memory);
        const cpuValue = wholeNumber(cpu);
        const diskValue = wholeNumber(disk);
        const portValue = portNumber(port);
        const backupsValue = wholeNumber(backups);

        if (!trimmedName) next.name = 'Enter a name for the server.';
        if (memoryValue === null || memoryValue < 1) next.memory = 'Enter the memory limit in MB, as a whole number above zero.';
        if (cpuValue === null || cpuValue < 1) next.cpu = 'Enter the CPU limit as a percentage above zero.';
        if (diskValue === null) next.disk = 'Enter the disk limit in MB, as a whole number.';
        if (portValue === null) next.port = 'Enter a port between 1 and 65535.';
        if (backupsValue === null || backupsValue > MAX_BACKUP_LIMIT) {
            next.backups = `Enter a whole number from 0 to ${MAX_BACKUP_LIMIT}.`;
        }

        setErrors(next);
        setFormError('');
        if (
            Object.keys(next).length > 0 ||
            memoryValue === null ||
            cpuValue === null ||
            diskValue === null ||
            portValue === null ||
            backupsValue === null
        ) {
            return;
        }

        // Only what changed is sent; the API leaves everything else alone.
        const changes: Record<string, string | number> = {};
        if (trimmedName !== service.name) changes.name = trimmedName;
        if (memoryValue !== service.memory) changes.memory = memoryValue;
        if (cpuValue !== service.cpu) changes.cpu = cpuValue;
        if (diskValue !== service.disk) changes.disk = diskValue;
        if (portValue !== service.port) changes.port = portValue;
        if (image && image !== service.docker_image) changes.docker_image = image;
        if (backupsValue !== service.backup_limit) changes.backup_limit = backupsValue;

        if (Object.keys(changes).length === 0) {
            onClose();
            return;
        }

        setSaving(true);
        try {
            const res = await api.put<UpdateResponse>(`/admin/services/${service.id}`, changes);
            onSaved(res.data.service, res.data.warning);
        } catch (err) {
            const message = errorMessage(err, 'Could not save the server.');
            if (errorStatus(err) === 409 && 'port' in changes) {
                setErrors({ port: message });
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
            dismissible={!saving}
            size="lg"
            title={`Edit ${service.name}`}
            footer={
                <>
                    <Button variant="ghost" onClick={onClose} disabled={saving}>
                        Cancel
                    </Button>
                    <Button type="submit" form={formId} variant="primary" loading={saving}>
                        Save changes
                    </Button>
                </>
            }
        >
            <form id={formId} onSubmit={submit} className="space-y-4" noValidate>
                {formError && <Alert tone="danger">{formError}</Alert>}

                <Field label="Name" error={errors.name}>
                    {(field) => <Input {...field} value={name} onChange={(event) => setName(event.target.value)} maxLength={255} />}
                </Field>

                <div className="space-y-5">
                    <ResourceSlider
                        label="Memory"
                        unit="MB"
                        value={memory}
                        onChange={setMemory}
                        min={256}
                        max={Math.max(limits.memory.max, service.memory)}
                        step={256}
                        format={formatMB}
                        capacity={limits.memory.capacity}
                        hint={node ? limits.memory.hint : undefined}
                        error={errors.memory}
                    />
                    <ResourceSlider
                        label="CPU"
                        unit="%"
                        value={cpu}
                        onChange={setCpu}
                        min={25}
                        max={Math.max(limits.cpu.max, service.cpu)}
                        step={25}
                        format={formatCpu}
                        hint={node ? limits.cpu.hint : '100% is one full core.'}
                        error={errors.cpu}
                    />
                    <ResourceSlider
                        label="Disk"
                        unit="MB"
                        value={disk}
                        onChange={setDisk}
                        min={1024}
                        max={Math.max(limits.disk.max, service.disk)}
                        step={1024}
                        format={formatMB}
                        capacity={limits.disk.capacity}
                        hint={node ? limits.disk.hint : undefined}
                        error={errors.disk}
                    />
                </div>

                <div className="grid gap-4 sm:grid-cols-3">
                    <Field label="Primary port" error={errors.port}>
                        {(field) => <Input {...field} inputMode="numeric" value={port} onChange={(event) => setPort(event.target.value)} />}
                    </Field>
                    <Field label="Docker image" className="sm:col-span-2">
                        {(field) => (
                            <Select {...field} value={image} onChange={(event) => setImage(event.target.value)} className="font-mono text-xs">
                                {images.map((option) => (
                                    <option key={option} value={option}>
                                        {option}
                                    </option>
                                ))}
                            </Select>
                        )}
                    </Field>
                </div>

                <Field
                    label="Backups to keep"
                    hint="How many backups this server may have. 0 turns backups off."
                    error={errors.backups}
                    className="sm:max-w-xs"
                >
                    {(field) => <Input {...field} inputMode="numeric" value={backups} onChange={(event) => setBackups(event.target.value)} />}
                </Field>

                <Alert tone={portChanged || imageChanged ? 'warning' : 'info'}>
                    Changing the port or image rebuilds the server&apos;s container, which restarts the server if it is running. Its
                    files are kept.
                </Alert>
            </form>
        </Modal>
    );
}
