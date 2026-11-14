import { useState, type FormEvent } from 'react';
import { Link } from 'react-router-dom';
import { ArrowLeft } from 'lucide-react';
import { Alert, Button, Card, CardHeader, PageHeader } from '../../components/ui';
import api from '../../lib/api';
import { errorMessage } from '../../lib/errors';
import type { Node } from '../../lib/types';
import { NodeFormFields } from './shared/NodeFormFields';
import {
    emptyNodeForm,
    nodeFormToPayload,
    validateNodeForm,
    type NodeFormErrors,
    type NodeFormValues,
    type NodePayload,
} from './shared/NodeFormState';
import { NodeToken, NodeTokenSteps } from './shared/NodeToken';

const primaryLink =
    'inline-flex h-9 items-center justify-center gap-2 whitespace-nowrap rounded-control bg-primary px-4 text-sm font-medium ' +
    'text-primary-foreground shadow-card transition-colors hover:bg-primary-hover';

const backLink = (
    <Link to="/admin/nodes" className="inline-flex items-center gap-1.5 hover:text-foreground">
        <ArrowLeft size={16} aria-hidden />
        Nodes
    </Link>
);

export default function CreateNodePage() {
    const [values, setValues] = useState<NodeFormValues>(emptyNodeForm);
    const [errors, setErrors] = useState<NodeFormErrors>({});
    const [formError, setFormError] = useState<string | null>(null);
    const [saving, setSaving] = useState(false);
    const [created, setCreated] = useState<{ name: string; token: string } | null>(null);

    const change = (field: keyof NodeFormValues, value: string) => {
        setValues((current) => ({ ...current, [field]: value }));
        setErrors((current) => ({ ...current, [field]: undefined }));
    };

    const submit = async (event: FormEvent<HTMLFormElement>) => {
        event.preventDefault();
        if (saving) return;

        const problems = validateNodeForm(values);
        setErrors(problems);
        if (Object.keys(problems).length > 0) return;

        // An empty location is left out so the node keeps the API's default.
        const { location, ...rest } = nodeFormToPayload(values);
        const payload: Partial<NodePayload> = location ? { ...rest, location } : rest;

        setSaving(true);
        setFormError(null);
        try {
            const res = await api.post<{ node: Node; token: string }>('/admin/nodes', payload);
            setCreated({ name: res.data.node.name, token: res.data.token });
        } catch (err) {
            setFormError(errorMessage(err, 'Could not add the node.'));
        } finally {
            setSaving(false);
        }
    };

    if (created) {
        return (
            <>
                <PageHeader title={`${created.name} was added`} description="One more step: give the node's daemon its token." eyebrow={backLink} />

                <Card className="max-w-2xl">
                    <CardHeader
                        title="Node token"
                        description="Copy it now. It is shown only this once and cannot be shown again."
                        className="mb-4"
                    />
                    <div className="space-y-5">
                        <NodeToken token={created.token} />
                        <div>
                            <h3 className="mb-2 text-sm font-medium text-foreground">Next steps</h3>
                            <NodeTokenSteps lastStep="The node should show Online within a few seconds." />
                        </div>
                        <Alert tone="info">
                            If you lose the token, use "Regenerate token" on the nodes page to get a new one.
                        </Alert>
                        <div className="flex flex-wrap gap-2">
                            <Link to="/admin/nodes" className={primaryLink}>
                                Go to nodes
                            </Link>
                        </div>
                    </div>
                </Card>
            </>
        );
    }

    return (
        <>
            <PageHeader title="Add node" description="Register a machine that runs the Atlas daemon." eyebrow={backLink} />

            <Card className="max-w-3xl">
                <form onSubmit={submit} noValidate className="space-y-5">
                    {formError && <Alert tone="danger">{formError}</Alert>}
                    <NodeFormFields values={values} errors={errors} onChange={change} />
                    <div className="flex flex-wrap items-center gap-2 border-t border-border pt-5">
                        <Button variant="primary" type="submit" loading={saving}>
                            Add node
                        </Button>
                        <Link
                            to="/admin/nodes"
                            className="inline-flex h-9 items-center rounded-control px-4 text-sm font-medium text-muted transition-colors hover:bg-secondary hover:text-foreground"
                        >
                            Cancel
                        </Link>
                    </div>
                </form>
            </Card>
        </>
    );
}
