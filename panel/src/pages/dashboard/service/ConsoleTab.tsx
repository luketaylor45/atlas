import { useEffect, useImperativeHandle, useRef, useState, type FormEvent, type KeyboardEvent, type Ref } from 'react';
import { SendHorizontal } from 'lucide-react';
import api from '../../../lib/api';
import { errorMessage } from '../../../lib/errors';
import type { Service } from '../../../lib/types';
import { Badge, Button, Card, Field, Input, useToast } from '../../../components/ui';
import { ResourceCards } from './ResourceCards';
import { Terminal, type TerminalHandle } from './Terminal';
import { isLiveStatus, type ServiceAccess } from './shared';

export interface ConsoleHandle {
    /** Writes one of Atlas's own messages into the console, apart from server output. */
    notice: (text: string, tone?: 'info' | 'error') => void;
    /** Drops the console connection and opens a new one. */
    reconnect: () => void;
}

// Docker prefixes each line with a timestamp: 2024-03-21T12:00:00.123456789Z message
const TIMESTAMPED = /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?Z\s?([\s\S]*)$/;
const MAX_REMEMBERED_LINES = 10000;
const MAX_HISTORY = 100;

const NOTICE = '\x1b[1;96m';
const ERROR = '\x1b[1;91m';
const DIM = '\x1b[90m';
const RESET = '\x1b[0m';

export function ConsoleTab({ service, access, ref }: { service: Service; access: ServiceAccess; ref?: Ref<ConsoleHandle> }) {
    const uuid = service.uuid;
    const toast = useToast();
    const terminal = useRef<TerminalHandle>(null);
    // Timestamped lines already shown, so a reconnect's replay is not printed twice.
    const seen = useRef<Set<string> | null>(null);

    const [openSocket, setOpenSocket] = useState<WebSocket | null>(null);
    const [hasConnected, setHasConnected] = useState(false);
    const [version, setVersion] = useState(0);

    const [command, setCommand] = useState('');
    const [sending, setSending] = useState(false);
    const [history, setHistory] = useState<string[]>([]);
    const [historyIndex, setHistoryIndex] = useState<number | null>(null);

    const live = isLiveStatus(service.status);
    // A server can stay "starting" for minutes while the game loads, and it reads its console the whole time.
    const up = service.status === 'running' || service.status === 'starting';

    useImperativeHandle(
        ref,
        () => ({
            notice: (text, tone = 'info') => terminal.current?.writeln(`${tone === 'error' ? ERROR : NOTICE}[Atlas] ${text}${RESET}`),
            reconnect: () => setVersion((current) => current + 1),
        }),
        [],
    );

    // Console stream. The socket is relayed by Core and authorised with a
    // short-lived ticket, so the browser never talks to a node directly.
    useEffect(() => {
        let stopped = false;
        let attempt = 0;
        let reconnectTimer: ReturnType<typeof setTimeout> | undefined;
        let socket: WebSocket | null = null;

        // An offline server is connected to once, to show its last output;
        // only a live one is worth retrying.
        const scheduleReconnect = () => {
            if (stopped || !live) return;
            attempt += 1;
            reconnectTimer = setTimeout(connect, Math.min(1000 * 2 ** attempt, 15000));
        };

        const connect = async () => {
            if (stopped) return;

            let ticket = '';
            try {
                const res = await api.post<{ ticket: string }>(`/services/${uuid}/console/ticket`);
                ticket = res.data.ticket;
            } catch {
                scheduleReconnect();
                return;
            }
            if (stopped) return;

            const wsBase = String(api.defaults.baseURL || '').replace(/^http/i, 'ws');
            const ws = new WebSocket(`${wsBase}/services/${uuid}/console/ws?ticket=${encodeURIComponent(ticket)}`);
            socket = ws;

            ws.onopen = () => {
                attempt = 0;
                setOpenSocket(ws);
                setHasConnected(true);
            };

            ws.onmessage = (event) => {
                const raw = String(event.data || '');
                if (!raw) return;

                let line = raw;
                const match = raw.match(TIMESTAMPED);
                if (match) {
                    // A reconnect replays recent history; timestamped lines are
                    // unique, so they are used to drop the replayed copies.
                    const remembered = (seen.current ??= new Set<string>());
                    if (remembered.has(raw)) return;
                    remembered.add(raw);
                    if (remembered.size > MAX_REMEMBERED_LINES) {
                        const oldest = remembered.values().next().value;
                        if (oldest !== undefined) remembered.delete(oldest);
                    }
                    line = match[1];
                }
                terminal.current?.writeln(line.replace(/\r?\n$/, ''));
            };

            ws.onclose = () => {
                setOpenSocket((current) => (current === ws ? null : current));
                if (socket === ws) socket = null;
                scheduleReconnect();
            };

            ws.onerror = () => {
                ws.close();
            };
        };

        void connect();

        return () => {
            stopped = true;
            clearTimeout(reconnectTimer);
            socket?.close();
        };
    }, [uuid, live, version]);

    const connection = !live
        ? { label: 'Server offline', tone: 'neutral' as const, pulse: false }
        : openSocket
          ? { label: 'Connected', tone: 'success' as const, pulse: false }
          : { label: hasConnected ? 'Reconnecting' : 'Connecting', tone: 'warning' as const, pulse: true };

    const blocked = !access.canCommands
        ? 'You do not have permission to send commands to this server.'
        : !up
          ? 'Commands can be sent once the server is running.'
          : null;

    const submit = async (event: FormEvent) => {
        event.preventDefault();
        const text = command.trim();
        if (!text || blocked || sending) return;

        setSending(true);
        terminal.current?.writeln(`${DIM}> ${text}${RESET}`);
        setHistory((current) => [...current.filter((entry) => entry !== text), text].slice(-MAX_HISTORY));
        setHistoryIndex(null);
        setCommand('');
        try {
            await api.post(`/services/${uuid}/command`, { command: text });
        } catch (err) {
            toast.error(errorMessage(err, 'Could not send the command.'));
        } finally {
            setSending(false);
        }
    };

    // Up and down step through the commands sent in this session.
    const onKeyDown = (event: KeyboardEvent<HTMLInputElement>) => {
        if (event.key !== 'ArrowUp' && event.key !== 'ArrowDown') return;
        if (history.length === 0) return;
        event.preventDefault();

        if (event.key === 'ArrowUp') {
            const next = historyIndex === null ? history.length - 1 : Math.max(0, historyIndex - 1);
            setHistoryIndex(next);
            setCommand(history[next]);
            return;
        }
        if (historyIndex === null) return;
        const next = historyIndex + 1;
        if (next >= history.length) {
            setHistoryIndex(null);
            setCommand('');
        } else {
            setHistoryIndex(next);
            setCommand(history[next]);
        }
    };

    return (
        <div className="space-y-4">
            <ResourceCards service={service} />

            <Card flush className="overflow-hidden">
                <div className="flex flex-wrap items-center justify-between gap-2 border-b border-border px-4 py-3 sm:px-5">
                    <h2 className="text-sm font-medium text-foreground">Console</h2>
                    <Badge tone={connection.tone} dot pulse={connection.pulse}>
                        {connection.label}
                    </Badge>
                </div>

                <div className="bg-terminal p-3">
                    <Terminal ref={terminal} label="Server console output" className="h-[24rem] sm:h-[30rem]" />
                </div>

                <form onSubmit={submit} className="border-t border-border p-4 sm:p-5">
                    <div className="flex items-end gap-2">
                        <Field label="Command" className="min-w-0 flex-1">
                            {(field) => (
                                <Input
                                    {...field}
                                    value={command}
                                    onChange={(event) => {
                                        setCommand(event.target.value);
                                        setHistoryIndex(null);
                                    }}
                                    onKeyDown={onKeyDown}
                                    disabled={Boolean(blocked)}
                                    placeholder={blocked ? '' : 'Type a command and press Enter'}
                                    className="font-mono"
                                    autoComplete="off"
                                    autoCapitalize="off"
                                    spellCheck={false}
                                />
                            )}
                        </Field>
                        <Button
                            type="submit"
                            variant="primary"
                            loading={sending}
                            disabled={Boolean(blocked)}
                            icon={<SendHorizontal size={16} aria-hidden />}
                        >
                            Send
                        </Button>
                    </div>
                    <p className="mt-2 text-xs text-muted">{blocked ?? 'Use the up and down arrows to recall earlier commands.'}</p>
                </form>
            </Card>
        </div>
    );
}
