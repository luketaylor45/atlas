import { parseRules } from '../lib/rules';
import type { EggVariable } from '../lib/types';
import { Input, Select, Switch } from './ui';

export interface VariableInputProps {
    variable: EggVariable;
    value: string;
    onChange: (next: string) => void;
    disabled?: boolean;
    /**
     * The value before any editing: what is stored, or the default on a new
     * server. A list keeps offering it even when the rules no longer allow
     * it, so it only changes when the user picks something else.
     */
    original?: string;
    /** Passed on from the surrounding `Field`. */
    id?: string;
    'aria-describedby'?: string;
    invalid?: boolean;
}

/**
 * What a switch writes when it is turned on and off: "1" and "0", unless the
 * variable already says "true" or "false", in which case it keeps doing so.
 */
function switchValues(examples: string[]): [on: string, off: string] {
    const example = examples.find((text) => ['true', 'false', '1', '0'].includes(text.toLowerCase())) ?? '';
    const lower = example.toLowerCase();
    if (lower !== 'true' && lower !== 'false') return ['1', '0'];
    if (example === lower) return ['true', 'false'];
    return example === example.toUpperCase() ? ['TRUE', 'FALSE'] : ['True', 'False'];
}

/**
 * The control for one egg variable, chosen from its rules: a list for `in:`,
 * a switch for `boolean`, a number pad for numbers and a text or password
 * input for everything else. The value is always text, as the API stores it.
 *
 *   <Field label={variable.name} error={error}>
 *     {(field) => <VariableInput {...field} variable={variable} value={value} onChange={setValue} />}
 *   </Field>
 */
export default function VariableInput({ variable, value, onChange, disabled, original, ...field }: VariableInputProps) {
    const rules = parseRules(variable.rules);

    if (rules.options) {
        const listed = rules.options;
        // Also offered: nothing at all where the rules allow that, and the
        // value from before editing or the one set now when the list lacks it.
        const unlisted = [!rules.required || rules.nullable ? '' : undefined, original, value].filter(
            (option): option is string => option !== undefined && !listed.includes(option),
        );
        return (
            <Select {...field} value={value} disabled={disabled} onChange={(event) => onChange(event.target.value)}>
                {[...new Set([...unlisted, ...listed])].map((option) => (
                    <option key={option} value={option}>
                        {option === '' ? 'Not set' : option}
                    </option>
                ))}
            </Select>
        );
    }

    if (rules.kind === 'boolean') {
        const [on, off] = switchValues([value, original ?? '', variable.default_value ?? '']);
        const checked = ['1', 'true'].includes(value.toLowerCase());
        return (
            <Switch
                id={field.id}
                aria-describedby={field['aria-describedby']}
                checked={checked}
                onChange={(next) => onChange(next ? on : off)}
                label={checked ? 'On' : 'Off'}
                disabled={disabled}
                className="h-9 w-fit flex-row-reverse items-center gap-3 py-0"
            />
        );
    }

    const password = variable.input_type === 'password';
    const number = rules.kind === 'number' || rules.kind === 'integer';
    // A phone's number pad has no minus key, so it is not asked for where a negative number is expected.
    const negative = (rules.min ?? 0) < 0 || [original ?? value, variable.default_value ?? ''].some((text) => text.startsWith('-'));

    return (
        <Input
            {...field}
            type={password ? 'password' : 'text'}
            inputMode={number && !negative ? (rules.kind === 'integer' ? 'numeric' : 'decimal') : undefined}
            autoComplete={password ? 'new-password' : 'off'}
            spellCheck={false}
            value={value}
            disabled={disabled}
            onChange={(event) => onChange(event.target.value)}
        />
    );
}
