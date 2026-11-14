import { useId, useState, type FormEvent } from 'react';
import api from '../../../lib/api';
import { errorMessage } from '../../../lib/errors';
import { parseInteger } from '../../../lib/format';
import type { Nest } from '../../../lib/types';
import { Alert, Button, Field, Input, Modal, Select, Textarea } from '../../../components/ui';

export interface NestFormModalProps {
    /** The nest being edited; leave out to create a new one. */
    nest?: Nest;
    /** Root nests a new nest can be placed under. */
    parents?: Nest[];
    /** Preselected parent for a new nest. */
    defaultParentId?: number | null;
    onClose: () => void;
    onSaved: (nest: Nest) => void;
}

/**
 * Creates or edits a nest. Render it only while it should be open (and give
 * it a `key`) so every opening starts from a clean form.
 */
export function NestFormModal({ nest, parents = [], defaultParentId = null, onClose, onSaved }: NestFormModalProps) {
    const formId = useId();
    const editing = Boolean(nest);
    const [name, setName] = useState(nest?.name ?? '');
    const [description, setDescription] = useState(nest?.description ?? '');
    const [parentId, setParentId] = useState(defaultParentId ? String(defaultParentId) : '');
    const [nameError, setNameError] = useState('');
    const [formError, setFormError] = useState('');
    const [saving, setSaving] = useState(false);

    const submit = async (event: FormEvent) => {
        event.preventDefault();
        if (saving) return;
        const trimmed = name.trim();
        if (!trimmed) {
            setNameError('Enter a name for the nest.');
            return;
        }
        setNameError('');
        setFormError('');
        setSaving(true);
        try {
            if (nest) {
                const res = await api.put<Nest>(`/admin/nests/${nest.id}`, { name: trimmed, description });
                onSaved(res.data);
            } else {
                const parent = parseInteger(parentId, 0);
                const res = await api.post<Nest>('/admin/nests', {
                    name: trimmed,
                    description,
                    ...(parent > 0 ? { parent_id: parent } : {}),
                });
                onSaved(res.data);
            }
        } catch (err) {
            setFormError(errorMessage(err, editing ? 'Could not save the nest.' : 'Could not create the nest.'));
            setSaving(false);
        }
    };

    return (
        <Modal
            open
            onClose={onClose}
            dismissible={!saving}
            title={editing ? 'Edit nest' : 'New nest'}
            description={editing ? undefined : 'Nests group eggs, for example by game or by engine.'}
            footer={
                <>
                    <Button variant="ghost" onClick={onClose} disabled={saving}>
                        Cancel
                    </Button>
                    <Button type="submit" form={formId} variant="primary" loading={saving}>
                        {editing ? 'Save nest' : 'Create nest'}
                    </Button>
                </>
            }
        >
            <form id={formId} onSubmit={submit} className="space-y-4" noValidate>
                {formError && <Alert tone="danger">{formError}</Alert>}
                <Field label="Name" error={nameError}>
                    {(field) => <Input {...field} value={name} onChange={(event) => setName(event.target.value)} maxLength={255} />}
                </Field>
                <Field label="Description" aside="Optional">
                    {(field) => (
                        <Textarea {...field} value={description} onChange={(event) => setDescription(event.target.value)} rows={3} />
                    )}
                </Field>
                {!editing && (
                    <Field label="Inside" hint="Nests can sit one level deep.">
                        {(field) => (
                            <Select {...field} value={parentId} onChange={(event) => setParentId(event.target.value)}>
                                <option value="">Nothing (top level)</option>
                                {parents.map((parent) => (
                                    <option key={parent.id} value={parent.id}>
                                        {parent.name}
                                    </option>
                                ))}
                            </Select>
                        )}
                    </Field>
                )}
            </form>
        </Modal>
    );
}
