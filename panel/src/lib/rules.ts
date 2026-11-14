// Egg variable rules, checked the same way the API checks them when a value
// is changed. The rules use the Laravel syntax that Pterodactyl eggs are
// written in ("required|string|max:20"). Rules Atlas does not know are
// ignored, and whatever cannot be checked here is left to the API.

import type { EggVariable } from './types';

export type RuleKind = 'text' | 'number' | 'integer' | 'boolean';

/** What a variable's rules say about the control it needs. */
export interface ParsedRules {
    /** An empty value is refused, unless `nullable` is set as well. */
    required: boolean;
    nullable: boolean;
    kind: RuleKind;
    /** The choices of an `in:` rule, or null when the value is free text. */
    options: string[] | null;
    /** The limits of `min`, `max` and `between`: on the number for number kinds, on the length of anything else. */
    min?: number;
    max?: number;
}

interface Rule {
    name: string;
    argument: string;
}

const NULLABLE_RULES = ['nullable', 'sometimes'];
const NUMBER_RULES = ['numeric', 'integer', 'int', 'digits'];
const REGEX_FLAGS = 'imsxuADSUXJ';

const DECIMAL = /^[+-]?(\d+\.?\d*|\.\d+)(e[+-]?\d+)?$/i;
const IPV4 = /^(25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)(\.(25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)){3}$/;
// A scheme, "//" and a host, which is all the API asks of a URL.
const URL_SHAPE = /^[a-z][a-z0-9+.-]*:\/\/([^\s/?#]*@)?[^\s/?#@]+(?=[/?#]|$)/i;

/**
 * Splits on "|", except inside a regex rule, whose pattern may contain that
 * character.
 */
function splitRules(rules: string): Rule[] {
    const result: Rule[] = [];
    let rest = rules.trim();
    while (rest !== '') {
        let part: string;
        if (rest.startsWith('regex:') || rest.startsWith('not_regex:')) {
            [part, rest] = regexPart(rest);
        } else {
            const cut = rest.indexOf('|');
            [part, rest] = cut >= 0 ? [rest.slice(0, cut), rest.slice(cut + 1)] : [rest, ''];
        }

        part = part.trim();
        if (part === '') continue;
        const colon = part.indexOf(':');
        result.push({
            name: (colon >= 0 ? part.slice(0, colon) : part).trim().toLowerCase(),
            argument: colon >= 0 ? part.slice(colon + 1) : '',
        });
    }
    return result;
}

/**
 * Takes a regex rule off the front of a rule string. The pattern is delimited
 * ("/…/flags"), so it ends at the last delimiter that is followed by a "|" or
 * the end of the string.
 */
function regexPart(rules: string): [part: string, rest: string] {
    const pattern = rules.slice(rules.indexOf(':') + 1);
    if (pattern === '') return [rules, ''];

    const delimiter = pattern[0];
    for (let end = pattern.length - 1; end > 0; end--) {
        if (pattern[end] !== delimiter) continue;
        const tail = pattern.slice(end + 1);
        const bar = tail.indexOf('|');
        const flags = bar >= 0 ? tail.slice(0, bar) : tail;
        if (![...flags].every((flag) => REGEX_FLAGS.includes(flag))) continue;
        const consumed = rules.length - tail.length + flags.length;
        return [rules.slice(0, consumed), bar >= 0 ? tail.slice(bar + 1) : ''];
    }
    return [rules, ''];
}

/** A decimal number, or null when the text is anything else. */
function parseNumber(text: string): number | null {
    if (!DECIMAL.test(text)) return null;
    const parsed = Number(text);
    return Number.isFinite(parsed) ? parsed : null;
}

/** True for a whole number that fits in 64 bits, which is as far as the API reads one. */
function isWholeNumber(text: string): boolean {
    const match = /^([+-]?)(\d+)$/.exec(text);
    if (!match) return false;
    const digits = match[2].replace(/^0+(?=\d)/, '');
    const limit = match[1] === '-' ? '9223372036854775808' : '9223372036854775807';
    return digits.length < limit.length || (digits.length === limit.length && digits <= limit);
}

function isIPv6(text: string): boolean {
    const halves = text.split('::');
    if (halves.length > 2) return false;

    let groups = 0;
    for (const [index, half] of halves.entries()) {
        if (half === '') continue;
        const parts = half.split(':');
        for (const [position, part] of parts.entries()) {
            // Only the very end may be written as an IPv4 address, which stands for two groups.
            const atEnd = index === halves.length - 1 && position === parts.length - 1;
            if (atEnd && IPV4.test(part)) groups += 2;
            else if (/^[0-9a-f]{1,4}$/i.test(part)) groups += 1;
            else return false;
        }
    }
    // "::" stands for at least one group of zeros.
    return halves.length === 2 ? groups <= 7 : groups === 8;
}

/**
 * Turns a PHP-style delimited pattern ("/^a+$/i") into a regular expression.
 * Returns null for a pattern that cannot be checked here.
 */
function compilePattern(pattern: string): RegExp | null {
    if (pattern.length < 2) return null;
    const end = pattern.lastIndexOf(pattern[0]);
    if (end <= 0) return null;

    const body = pattern.slice(1, end);
    // The API cannot express lookarounds or backreferences, and does not enforce a pattern that has them.
    if (/\(\?<?[=!]|\\[1-9]|\\k</.test(body)) return null;

    const given = pattern.slice(end + 1);
    const flags = ['i', 'm', 's'].filter((flag) => given.includes(flag)).join('');
    try {
        // Unicode mode refuses syntax it does not know instead of guessing at
        // it, so a pattern written for another engine is skipped, not misread.
        return new RegExp(body, `${flags}u`);
    } catch {
        return null;
    }
}

/** The numbers in the argument of a size rule, or null when the rule cannot be applied. */
function sizeLimits(rule: Rule): number[] | null {
    const limits: number[] = [];
    for (const bound of rule.argument.split(',')) {
        const limit = parseNumber(bound.trim());
        if (limit === null) return null;
        limits.push(limit);
    }
    return rule.name === 'between' && limits.length < 2 ? null : limits;
}

/**
 * Applies min, max, between and size. For a number they compare the number;
 * for text, its length in characters.
 */
function checkSize(rule: Rule, value: string, numeric: boolean): string | null {
    const limits = sizeLimits(rule);
    if (!limits) return null;

    let measure = [...value].length;
    if (numeric) {
        const number = parseNumber(value);
        // The numeric rule reports this.
        if (number === null) return null;
        measure = number;
    }
    const format = (limit: number) => (numeric ? String(limit) : `${limit} ${limit === 1 ? 'character' : 'characters'}`);

    switch (rule.name) {
        case 'min':
            return measure < limits[0] ? `must be at least ${format(limits[0])}` : null;
        case 'max':
            return measure > limits[0] ? `must be at most ${format(limits[0])}` : null;
        case 'between':
            return measure < limits[0] || measure > limits[1] ? `must be between ${limits[0]} and ${format(limits[1])}` : null;
        default:
            return measure !== limits[0] ? `must be exactly ${format(limits[0])}` : null;
    }
}

function check(rule: Rule, value: string, numeric: boolean): string | null {
    switch (rule.name) {
        case 'numeric':
            return parseNumber(value) === null ? 'must be a number' : null;
        case 'integer':
        case 'int':
            return isWholeNumber(value) ? null : 'must be a whole number';
        case 'digits':
            if (!/^\d+$/.test(value)) return 'must contain only digits';
            if (/^[+-]?\d+$/.test(rule.argument) && value.length !== Number(rule.argument)) {
                return `must be ${Number(rule.argument)} digits long`;
            }
            return null;
        case 'boolean':
        case 'bool':
            return ['true', 'false', '1', '0'].includes(value.toLowerCase()) ? null : 'must be true or false';
        case 'in': {
            const options = rule.argument.split(',');
            return options.includes(value) ? null : `must be one of: ${options.join(', ')}`;
        }
        case 'not_in':
            return rule.argument.split(',').includes(value) ? `cannot be ${value}` : null;
        case 'min':
        case 'max':
        case 'between':
        case 'size':
            return checkSize(rule, value, numeric);
        case 'regex':
        case 'not_regex': {
            const expression = compilePattern(rule.argument);
            if (!expression) return null;
            return expression.test(value) === (rule.name === 'regex') ? null : 'is not in the expected format';
        }
        case 'alpha':
            return /^[a-z]+$/i.test(value) ? null : 'may only contain letters';
        case 'alpha_num':
            return /^[a-z0-9]+$/i.test(value) ? null : 'may only contain letters and numbers';
        case 'alpha_dash':
            return /^[a-z0-9_-]+$/i.test(value) ? null : 'may only contain letters, numbers, dashes and underscores';
        case 'url':
        case 'active_url':
            return URL_SHAPE.test(value) ? null : 'must be a URL';
        case 'ip':
            return IPV4.test(value) || isIPv6(value) ? null : 'must be an IP address';
        case 'ipv4':
            // The API also takes an IPv4 address written in IPv6 form, so only
            // text that is no address at all is refused here.
            return IPV4.test(value) || isIPv6(value) ? null : 'must be an IPv4 address';
        default:
            return null;
    }
}

/** Reads what a variable's rules mean for the control that edits it. */
export function parseRules(rules: string | null | undefined): ParsedRules {
    const parts = splitRules(rules ?? '');
    const has = (names: string[]) => parts.some((rule) => names.includes(rule.name));
    const choices = parts.find((rule) => rule.name === 'in' && rule.argument !== '');

    const parsed: ParsedRules = {
        required: has(['required']),
        nullable: has(NULLABLE_RULES),
        kind: has(['integer', 'int', 'digits']) ? 'integer' : has(['numeric']) ? 'number' : has(['boolean', 'bool']) ? 'boolean' : 'text',
        options: choices ? choices.argument.split(',') : null,
    };

    for (const rule of parts) {
        if (rule.name !== 'min' && rule.name !== 'max' && rule.name !== 'between') continue;
        const limits = sizeLimits(rule);
        if (!limits) continue;
        if (rule.name === 'min') parsed.min = limits[0];
        else if (rule.name === 'max') parsed.max = limits[0];
        else [parsed.min, parsed.max] = limits;
    }
    return parsed;
}

/**
 * Checks a value against a variable's rules. Returns null when it is fine,
 * and otherwise a sentence fragment that follows the variable's name, the
 * same one the API answers with: "must be a whole number".
 */
export function validateValue(rules: string | null | undefined, value: string): string | null {
    const parts = splitRules(rules ?? '');
    const has = (names: string[]) => parts.some((rule) => names.includes(rule.name));

    if (value === '') {
        // An empty optional value has nothing left to check.
        return has(['required']) && !has(NULLABLE_RULES) ? 'is required' : null;
    }

    const numeric = has(NUMBER_RULES);
    for (const rule of parts) {
        const problem = check(rule, value, numeric);
        if (problem) return problem;
    }
    return null;
}

/** The sentence to show for a variable whose value breaks its rules, or null when the value is fine. */
export function variableError(variable: Pick<EggVariable, 'name' | 'environment_variable' | 'rules'>, value: string): string | null {
    const problem = validateValue(variable.rules, value);
    return problem ? `${variable.name?.trim() || variable.environment_variable} ${problem}.` : null;
}
