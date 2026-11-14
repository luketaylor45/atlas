import { Component, type ErrorInfo, type ReactNode } from 'react';
import { TriangleAlert } from 'lucide-react';
import { Button } from './ui';

interface Props {
    children: ReactNode;
}

interface State {
    error: Error | null;
}

/**
 * Catches a rendering error in a page so the rest of the panel keeps working,
 * instead of the whole screen going blank.
 */
export default class ErrorBoundary extends Component<Props, State> {
    state: State = { error: null };

    static getDerivedStateFromError(error: Error): State {
        return { error };
    }

    componentDidCatch(error: Error, info: ErrorInfo) {
        console.error('Atlas page error:', error, info.componentStack);
    }

    render() {
        if (!this.state.error) return this.props.children;

        return (
            <div role="alert" className="mx-auto flex max-w-md flex-col items-center px-6 py-20 text-center">
                <div className="mb-4 flex h-11 w-11 items-center justify-center rounded-full border border-danger/25 bg-danger/10 text-danger">
                    <TriangleAlert size={20} aria-hidden />
                </div>
                <h1 className="text-lg font-semibold tracking-tight text-foreground">This page hit a problem</h1>
                <p className="mt-2 text-sm text-muted">
                    Something went wrong while showing this page. Your servers are not affected. Try again, or reload the panel.
                </p>
                <div className="mt-6 flex gap-2">
                    <Button variant="primary" onClick={() => this.setState({ error: null })}>
                        Try again
                    </Button>
                    <Button onClick={() => window.location.reload()}>Reload</Button>
                </div>
                <p className="mt-6 max-w-full break-words font-mono text-xs text-subtle">{this.state.error.message}</p>
            </div>
        );
    }
}
