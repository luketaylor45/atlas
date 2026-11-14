import { useId, useState, type FormEvent } from 'react';
import api from '../../../lib/api';
import { errorMessage, errorStatus } from '../../../lib/errors';
import { eggImages, type Service } from '../../../lib/types';
import { Alert, Button, Field, Input, Modal, Select } from '../../../components/ui';
import { portNumber, wholeNumber } from './ServiceForm';

export interface ServiceEditModalProps {
    service: Service;
    onClose: () => void;
    /** Called after a successful save, with the node's warning if it had one. */
    onSaved: (service: Service, warning?: string) => void;
}

type FieldName = 'name' | 'memory' | 'cpu' | 'disk' | 'port';

interface UpdateResponse {
    service: Service;
    warning?: string;
}

/**
 * Edits a server's name, limits, primary port and image. Render it only while
 * it should be open, keyed by the server, so the form starts from its values.
 */
export function ServiceEditModal({ service, onClose, onSaved }: ServiceEditModalProps) {
    const formId = useId();
    const [name, setName] = useState(service.name);
    const [memory, setMemory] = useState(String(service.memory));
    const [cpu, setCpu] = useState(String(service.cpu));
    const [disk, setDisk] = useState(String(service.disk));
    const [port, setPort] = useState(String(service.port));
    const [image, setImage] = useState(service.docker_image);
    const [errors, setErrors] = useState<Partial<Record<FieldName, string>>>({});
    const [formError, setFormError] = useState('');
    const [saving, setSaving] = useState(false);

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

        if (!trimmedName) next.name = 'Enter a name for the server.';
        if (memoryValue === null || memoryValue < 1) next.memory = 'Enter the memory limit in MB, as a whole number above zero.';
        if (cpuValue === null || cpuValue < 1) next.cpu = 'Enter the CPU limit as a percentage above zero.';
        if (diskValue === null) next.disk = 'Enter the disk limit in MB, as a whole number.';
        if (portValue === null) next.port = 'Enter a port between 1 and 65535.';

        setErrors(next);
        setFormError('');
        if (
            Object.keys(next).length > 0 ||
            memoryValue === null ||
            cpuValue === null ||
            diskValue === null ||
            portValue === null
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

                <div className="grid gap-4 sm:grid-cols-3">
                    <Field label="Memory (MB)" error={errors.memory}>
                        {(field) => (
                            <Input {...field} inputMode="numeric" value={memory} onChange={(event) => setMemory(event.target.value)} />
                        )}
                    </Field>
                    <Field label="CPU (%)" hint="100 = one core" error={errors.cpu}>
                        {(field) => <Input {...field} inputMode="numeric" value={cpu} onChange={(event) => setCpu(event.target.value)} />}
                    </Field>
                    <Field label="Disk (MB)" error={errors.disk}>
                        {(field) => <Input {...field} inputMode="numeric" value={disk} onChange={(event) => setDisk(event.target.value)} />}
                    </Field>
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

                <Alert tone={portChanged || imageChanged ? 'warning' : 'info'}>
                    Changing the port or image rebuilds the server&apos;s container, which restarts the server if it is running. Its
                    files are kept.
                </Alert>
            </form>
        </Modal>
    );
}
