import { CopyButton } from '../../../components/ui';

/** A node token on a terminal surface with a copy button. Shown once only. */
export function NodeToken({ token }: { token: string }) {
    return (
        <div className="flex items-center gap-2 rounded-control border border-terminal-border bg-terminal py-2 pl-3 pr-2">
            <code className="min-w-0 flex-1 select-all break-all font-mono text-sm text-terminal-foreground">{token}</code>
            <CopyButton
                value={token}
                label="Copy token"
                className="text-terminal-muted hover:bg-terminal-border hover:text-terminal-foreground"
            />
        </div>
    );
}

/** The steps that put a token into a node's daemon. */
export function NodeTokenSteps({ lastStep }: { lastStep: string }) {
    return (
        <ol className="list-decimal space-y-2 pl-5 text-sm text-muted marker:text-subtle">
            <li>
                On the node, open its <span className="font-mono text-foreground">.env</span> file and set{' '}
                <span className="font-mono text-foreground">NODE_TOKEN</span> to this token.
            </li>
            <li>
                Restart the daemon: <span className="font-mono text-foreground">docker compose up -d daemon</span>
            </li>
            <li>{lastStep}</li>
        </ol>
    );
}
