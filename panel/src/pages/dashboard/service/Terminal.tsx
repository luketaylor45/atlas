import { useEffect, useImperativeHandle, useRef, type Ref } from 'react';
import { Terminal as XTerm } from '@xterm/xterm';
import { FitAddon } from '@xterm/addon-fit';
import '@xterm/xterm/css/xterm.css';
import { cn } from '../../../lib/cn';

export interface TerminalHandle {
    /** Writes one line of output; ANSI colour codes are rendered. */
    writeln: (text: string) => void;
    clear: () => void;
}

const FALLBACK_FONT = 'ui-monospace, "SF Mono", "Cascadia Code", "JetBrains Mono", Menlo, Consolas, monospace';
const HIDE_CURSOR = String.fromCharCode(27) + '[?25l';
const MAX_PENDING_LINES = 2000;

function cssVariable(name: string): string | undefined {
    const value = getComputedStyle(document.documentElement).getPropertyValue(name).trim();
    return value || undefined;
}

/** The terminal keeps its dark colours in both themes; they come from the --terminal tokens. */
function terminalTheme() {
    const background = cssVariable('--terminal');
    const accent = cssVariable('--primary');
    return {
        background,
        foreground: cssVariable('--terminal-foreground'),
        // Output only: the cursor is hidden by painting it in the background colour.
        cursor: background,
        cursorAccent: background,
        selectionBackground: accent && /^#[0-9a-f]{6}$/i.test(accent) ? `${accent}66` : cssVariable('--terminal-muted'),
    };
}

/**
 * A read-only terminal for server output. It is created once, follows the
 * size of its container and is disposed when unmounted.
 */
export function Terminal({ ref, className, label }: { ref?: Ref<TerminalHandle>; className?: string; label: string }) {
    const container = useRef<HTMLDivElement>(null);
    const terminal = useRef<XTerm | null>(null);
    // Lines written before the terminal exists are kept and shown once it does.
    const pending = useRef<string[]>([]);

    useImperativeHandle(
        ref,
        () => ({
            writeln: (text) => {
                if (terminal.current) {
                    terminal.current.writeln(text);
                    return;
                }
                pending.current.push(text);
                if (pending.current.length > MAX_PENDING_LINES) pending.current.shift();
            },
            clear: () => {
                pending.current = [];
                terminal.current?.clear();
            },
        }),
        [],
    );

    useEffect(() => {
        const node = container.current;
        if (!node) return;

        let xterm: XTerm | undefined;
        let observer: ResizeObserver | undefined;
        let fitFrame = 0;

        // The terminal is created on the next frame. A mount that is undone
        // straight away (React re-running effects in development, or a quick
        // tab switch) then never opens one. xterm does not tolerate being
        // disposed in the same tick it was opened: its deferred layout throws.
        const startFrame = requestAnimationFrame(() => {
            const created = new XTerm({
                disableStdin: true,
                convertEol: true,
                scrollback: 10000,
                cursorBlink: false,
                cursorInactiveStyle: 'none',
                fontFamily: cssVariable('--font-mono') ?? FALLBACK_FONT,
                fontSize: 13,
                lineHeight: 1.25,
                theme: terminalTheme(),
            });
            const fitAddon = new FitAddon();
            created.loadAddon(fitAddon);
            created.open(node);
            // Hide the text cursor: nothing is typed here.
            created.write(HIDE_CURSOR);

            const fit = () => {
                if (node.clientWidth === 0 || node.clientHeight === 0) return;
                try {
                    fitAddon.fit();
                } catch {
                    // The terminal is not measurable yet; the next resize will fit it.
                }
            };
            fit();

            for (const line of pending.current) created.writeln(line);
            pending.current = [];

            observer = new ResizeObserver(() => {
                cancelAnimationFrame(fitFrame);
                fitFrame = requestAnimationFrame(fit);
            });
            observer.observe(node);

            xterm = created;
            terminal.current = created;
        });

        return () => {
            cancelAnimationFrame(startFrame);
            cancelAnimationFrame(fitFrame);
            observer?.disconnect();
            terminal.current = null;
            xterm?.dispose();
        };
    }, []);

    return <div ref={container} role="log" aria-label={label} className={cn('min-w-0 overflow-hidden', className)} />;
}
