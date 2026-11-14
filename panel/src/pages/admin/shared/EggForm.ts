import { eggStartupDone, type Egg } from '../../../lib/types';

// The parsers a config file can be rewritten with. The API takes "yml" as another spelling of "yaml".
const PARSERS = ['properties', 'ini', 'json', 'yaml', 'yml', 'xml', 'file'];
const PARSER_LIST = 'properties, ini, json, yaml, xml or file';

/** An egg's startup detection as the editor shows it: one console text per line. */
export function startupDoneLines(egg: Pick<Egg, 'startup_done'>): string {
    return eggStartupDone(egg).join('\n');
}

/** Turns the lines typed into the editor into what is stored: a JSON list, or "" when there are none. */
export function startupDoneValue(text: string): string {
    const lines = text
        .split('\n')
        .map((line) => line.trim())
        .filter(Boolean);
    return lines.length > 0 ? JSON.stringify(lines) : '';
}

/**
 * Lays JSON out over several lines. Only the spacing changes: the text is not
 * parsed and written again, because that could reorder its keys, and the
 * order of config file rules matters.
 */
function indentJSON(text: string): string {
    let result = '';
    let depth = 0;
    let inString = false;
    const newline = () => `\n${'  '.repeat(depth)}`;

    for (let index = 0; index < text.length; index++) {
        const char = text[index];
        if (inString) {
            result += char;
            if (char === '\\') result += text[++index] ?? '';
            else if (char === '"') inString = false;
        } else if (char === '"') {
            inString = true;
            result += char;
        } else if (char === '{' || char === '[') {
            let next = index + 1;
            while (/\s/.test(text[next] ?? '')) next++;
            if (text[next] === '}' || text[next] === ']') {
                // Nothing inside: keep it on one line.
                result += char + text[next];
                index = next;
            } else {
                depth++;
                result += char + newline();
            }
        } else if (char === '}' || char === ']') {
            depth = Math.max(0, depth - 1);
            result += newline() + char;
        } else if (char === ',') {
            result += char + newline();
        } else if (char === ':') {
            result += ': ';
        } else if (!/\s/.test(char)) {
            result += char;
        }
    }
    return result;
}

/** An egg's config file rules as the editor shows them: indented JSON, or nothing. */
export function configFilesText(egg: Pick<Egg, 'config_files'>): string {
    const stored = (egg.config_files ?? '').trim();
    try {
        JSON.parse(stored);
    } catch {
        // Empty, or not JSON at all: shown as it is.
        return stored;
    }
    return indentJSON(stored);
}

/** Checks config file rules typed into the editor. Returns what is wrong with them, or null. */
export function configFilesProblem(text: string): string | null {
    if (!text.trim()) return null;

    let parsed: unknown;
    try {
        parsed = JSON.parse(text);
    } catch {
        return 'This is not valid JSON. Check for a missing comma, quote or bracket.';
    }
    if (typeof parsed !== 'object' || parsed === null || Array.isArray(parsed)) {
        return 'Config files are one JSON object, starting with { and ending with }, with an entry for each file.';
    }

    for (const [name, file] of Object.entries(parsed)) {
        const parser = typeof file === 'object' && file !== null ? (file as { parser?: unknown }).parser : undefined;
        if (typeof parser !== 'string' || !parser) {
            return `${name} needs a "parser": ${PARSER_LIST}.`;
        }
        if (!PARSERS.includes(parser)) {
            return `${name} uses the parser "${parser}", which Atlas does not have. Use ${PARSER_LIST}.`;
        }
    }
    return null;
}
