import { Field, Input } from '../../../components/ui';
import type { NodeFormErrors, NodeFormValues } from './NodeFormState';

interface NodeFormFieldsProps {
    values: NodeFormValues;
    errors: NodeFormErrors;
    onChange: (field: keyof NodeFormValues, value: string) => void;
    disabled?: boolean;
}

/** The node settings shared by the create page and the edit dialog. */
export function NodeFormFields({ values, errors, onChange, disabled }: NodeFormFieldsProps) {
    return (
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <Field label="Name" error={errors.name}>
                {(field) => (
                    <Input
                        {...field}
                        value={values.name}
                        onChange={(event) => onChange('name', event.target.value)}
                        placeholder="Node 1"
                        maxLength={255}
                        autoComplete="off"
                        disabled={disabled}
                    />
                )}
            </Field>

            <Field label="Location" aside="Optional" error={errors.location} hint="A label for you, such as a city or data centre.">
                {(field) => (
                    <Input
                        {...field}
                        value={values.location}
                        onChange={(event) => onChange('location', event.target.value)}
                        placeholder="London"
                        maxLength={255}
                        autoComplete="off"
                        disabled={disabled}
                    />
                )}
            </Field>

            <Field
                label="Public address"
                error={errors.address}
                hint="The hostname or IP address players and SFTP users connect to."
            >
                {(field) => (
                    <Input
                        {...field}
                        value={values.address}
                        onChange={(event) => onChange('address', event.target.value)}
                        placeholder="node1.example.com"
                        autoComplete="off"
                        autoCapitalize="none"
                        spellCheck={false}
                        disabled={disabled}
                    />
                )}
            </Field>

            <Field
                label="Internal address"
                aside="Optional"
                error={errors.internal_address}
                hint={
                    <>
                        How the panel reaches the daemon. Use <span className="font-mono">daemon</span> for the node in the same Docker
                        Compose stack; leave empty to use the public address.
                    </>
                }
            >
                {(field) => (
                    <Input
                        {...field}
                        value={values.internal_address}
                        onChange={(event) => onChange('internal_address', event.target.value)}
                        placeholder="daemon"
                        autoComplete="off"
                        autoCapitalize="none"
                        spellCheck={false}
                        disabled={disabled}
                    />
                )}
            </Field>

            <Field label="Daemon port" error={errors.port}>
                {(field) => (
                    <Input
                        {...field}
                        value={values.port}
                        onChange={(event) => onChange('port', event.target.value)}
                        inputMode="numeric"
                        placeholder="8081"
                        autoComplete="off"
                        disabled={disabled}
                    />
                )}
            </Field>

            <Field label="SFTP port" error={errors.sftp_port}>
                {(field) => (
                    <Input
                        {...field}
                        value={values.sftp_port}
                        onChange={(event) => onChange('sftp_port', event.target.value)}
                        inputMode="numeric"
                        placeholder="2022"
                        autoComplete="off"
                        disabled={disabled}
                    />
                )}
            </Field>

            <Field label="Total memory (MB)" error={errors.total_ram} hint="0 means no limit.">
                {(field) => (
                    <Input
                        {...field}
                        value={values.total_ram}
                        onChange={(event) => onChange('total_ram', event.target.value)}
                        inputMode="numeric"
                        placeholder="0"
                        autoComplete="off"
                        disabled={disabled}
                    />
                )}
            </Field>

            <Field label="Total disk (MB)" error={errors.total_disk} hint="0 means no limit.">
                {(field) => (
                    <Input
                        {...field}
                        value={values.total_disk}
                        onChange={(event) => onChange('total_disk', event.target.value)}
                        inputMode="numeric"
                        placeholder="0"
                        autoComplete="off"
                        disabled={disabled}
                    />
                )}
            </Field>
        </div>
    );
}
