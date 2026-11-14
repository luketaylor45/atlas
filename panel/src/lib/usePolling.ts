import { useEffect, useRef } from 'react';

/**
 * Calls `callback` now and then every `intervalMs` while `enabled`.
 *
 * - Never overlaps: a call is skipped while the previous one is still running,
 *   even across re-mounts and tab visibility changes. Callers that follow a
 *   cursor (a log offset, a page) rely on this.
 * - Pauses while the tab is hidden and refreshes as soon as it is shown again.
 * - Always runs the latest `callback`, so callers do not need to memoise it.
 */
export function usePolling(callback: () => void | Promise<void>, intervalMs: number, enabled = true) {
    const saved = useRef(callback);
    useEffect(() => {
        saved.current = callback;
    });

    // Shared by every run of the effect below, so a call started by an
    // earlier run still blocks a new one from starting.
    const inFlight = useRef(false);

    useEffect(() => {
        if (!enabled) return;

        let cancelled = false;
        let timer: ReturnType<typeof setTimeout> | undefined;

        const tick = async () => {
            if (cancelled) return;
            if (!document.hidden && !inFlight.current) {
                inFlight.current = true;
                try {
                    await saved.current();
                } catch {
                    // Callers handle their own errors; polling carries on.
                } finally {
                    inFlight.current = false;
                }
            }
            if (!cancelled) {
                clearTimeout(timer);
                timer = setTimeout(tick, intervalMs);
            }
        };

        const onVisibility = () => {
            if (document.hidden || cancelled) return;
            clearTimeout(timer);
            void tick();
        };

        void tick();
        document.addEventListener('visibilitychange', onVisibility);
        return () => {
            cancelled = true;
            clearTimeout(timer);
            document.removeEventListener('visibilitychange', onVisibility);
        };
    }, [intervalMs, enabled]);
}
