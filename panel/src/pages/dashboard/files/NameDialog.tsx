import { useId, useState, type FormEvent } from 'react';
import { Alert, Button, Field, Input, Modal } from '../../../components/ui';
import { validateName, type FileEntry } from './util';

export type NameDialogKind = 'file' | 'folder';

interface NameDialogProps {
    kind: NameDialogKind;
    /** Shown so the user knows where the item will be created. */
    location: string;
    /** Entries already in the folder, to refuse duplicates. */
    existing: FileEntry[];
    /** Creates the item. Throwing an Error shows its message in the dialog. */
    onCreate: (name: string) => Promise<void>;
    onClose: () => void;
}

/** Asks for the name of a new file or folder. Mount it only while it is open. */
export default function NameDialog({ kind, location, existing, onCreate, onClose }: NameDialogProps) {
    const formId = useId();
    const [name, setName] = useState('');
    const [fieldError, setFieldError] = useState<string | null>(null);
    const [submitError, setSubmitError] = useState<string | null>(null);
    const [submitting, setSubmitting] = useState(false);

    const submit = async (event: FormEvent) => {
        event.preventDefault();
        if (submitting) return;

        const problem = validateName(name, existing);
        setFieldError(problem);
        if (problem) return;

        setSubmitting(true);
        setSubmitError(null);
        try {
            await onCreate(name.trim());
        } catch (err) {
            setSubmitError(err instanceof Error ? err.message : 'Something went wrong. Please try again.');
            setSubmitting(false);
        }
    };

    return (
        <Modal
            open
            onClose={onClose}
            size="sm"
            dismissible={!submitting}
            title={kind === 'file' ? 'New file' : 'New folder'}
            description={`Created in ${location}`}
            footer={
                <>
                    <Button variant="ghost" onClick={onClose} disabled={submitting}>
                        Cancel
                    </Button>
                    <Button type="submit" form={formId} variant="primary" loading={submitting}>
                        {kind === 'file' ? 'Create file' : 'Create folder'}
                    </Button>
                </>
            }
        >
            <form id={formId} onSubmit={submit} className="space-y-4" noValidate>
                <Field
                    label={kind === 'file' ? 'File name' : 'Folder name'}
                    error={fieldError}
                    hint={kind === 'file' ? 'Include the extension, for example server.properties.' : undefined}
                >
                    {(field) => (
                        <Input
                            {...field}
                            value={name}
                            onChange={(event) => {
                                setName(event.target.value);
                                if (fieldError) setFieldError(null);
                            }}
                            placeholder={kind === 'file' ? 'config.yml' : 'plugins'}
                            autoComplete="off"
                            spellCheck={false}
                            maxLength={255}
                        />
                    )}
                </Field>
                {submitError && <Alert tone="danger">{submitError}</Alert>}
            </form>
        </Modal>
    );
}
