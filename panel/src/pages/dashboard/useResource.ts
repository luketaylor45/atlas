import { useCallback, useState } from 'react';
import api from '../../lib/api';
import { errorMessage } from '../../lib/errors';
import { usePolling } from '../../lib/usePolling';

export interface Resource<T> {
    /** The last successful response; kept while a later refresh fails. */
    data: T | null;
    /** Why the latest request failed, or null when it succeeded. */
    error: string | null;
    /** True until the first request settles. */
    loading: boolean;
    /** True while a manual retry is running. */
    retrying: boolean;
    retry: () => void;
}

/**
 * Loads one GET endpoint and refreshes it every `intervalMs`. Each resource
 * has its own error, so one failing request never blanks another.
 */
export function useResource<T>(url: string, fallbackError: string, intervalMs: number): Resource<T> {
    const [state, setState] = useState<{ data: T | null; error: string | null; settled: boolean }>({
        data: null,
        error: null,
        settled: false,
    });
    const [retrying, setRetrying] = useState(false);

    const load = useCallback(async () => {
        try {
            const res = await api.get<T>(url);
            setState({ data: res.data, error: null, settled: true });
        } catch (err) {
            setState((current) => ({ data: current.data, error: errorMessage(err, fallbackError), settled: true }));
        }
    }, [url, fallbackError]);

    usePolling(load, intervalMs);

    const retry = useCallback(() => {
        setRetrying(true);
        void load().finally(() => setRetrying(false));
    }, [load]);

    return { data: state.data, error: state.error, loading: !state.settled, retrying, retry };
}
