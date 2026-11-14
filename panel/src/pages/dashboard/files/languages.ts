import { json } from '@codemirror/lang-json';
import { StreamLanguage } from '@codemirror/language';
import { javascript } from '@codemirror/legacy-modes/mode/javascript';
import { lua } from '@codemirror/legacy-modes/mode/lua';
import { properties } from '@codemirror/legacy-modes/mode/properties';
import { shell } from '@codemirror/legacy-modes/mode/shell';
import { toml } from '@codemirror/legacy-modes/mode/toml';
import { xml } from '@codemirror/legacy-modes/mode/xml';
import { yaml } from '@codemirror/legacy-modes/mode/yaml';
import type { Extension } from '@uiw/react-codemirror';
import { extensionOf } from './util';

export interface EditorLanguage {
    /** Shown under the editor. */
    label: string;
    extensions: Extension[];
}

/** Picks syntax highlighting from the file extension; plain text otherwise. */
export function languageFor(fileName: string): EditorLanguage {
    switch (extensionOf(fileName)) {
        case 'json':
            return { label: 'JSON', extensions: [json()] };
        case 'properties':
        case 'ini':
        case 'cfg':
            return { label: 'Properties', extensions: [StreamLanguage.define(properties)] };
        case 'yaml':
        case 'yml':
            return { label: 'YAML', extensions: [StreamLanguage.define(yaml)] };
        case 'lua':
            return { label: 'Lua', extensions: [StreamLanguage.define(lua)] };
        case 'sh':
        case 'bash':
            return { label: 'Shell', extensions: [StreamLanguage.define(shell)] };
        case 'toml':
            return { label: 'TOML', extensions: [StreamLanguage.define(toml)] };
        case 'xml':
            return { label: 'XML', extensions: [StreamLanguage.define(xml)] };
        case 'js':
            return { label: 'JavaScript', extensions: [StreamLanguage.define(javascript)] };
        default:
            return { label: 'Plain text', extensions: [] };
    }
}
