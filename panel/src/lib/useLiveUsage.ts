import { useEffect, useState } from 'react';
import api from './api';
import { errorStatus } from './errors';
import type { ServiceStats } from './types';
import { usePolling } from './usePolling';

const INTERVAL_MS = 15_000;
const observable = typeof IntersectionObserver !== 'undefined';

/**
 * Live CPU and memory use for a server card. Asks the node only while the
 * server is up and the card is on screen, and stops for good if the person
 * may not see this server's usage.
 *
 * Pass `track` as the card's ref.
 */
export function useLiveUsage(uuid: string, active: boolean) {
    const [element, setElement] = useState<Element | null>(null);
    const [visible, setVisible] = useState(!observable);
    const [stats, setStats] = useState<ServiceStats | null>(null);
    const [denied, setDenied] = useState(false);

    useEffect(() => {
        if (!element || !observable) return;
        const observer = new IntersectionObserver(([entry]) => setVisible(entry.isIntersecting), { rootMargin: '200px 0px' });
        observer.observe(element);
        return () => observer.disconnect();
    }, [element]);

    usePolling(
        async () => {
            try {
                const res = await api.get<ServiceStats>(`/services/${uuid}/stats`);
                setStats(res.data);
            } catch (err) {
                setStats(null);
                if (errorStatus(err) === 403) setDenied(true);
            }
        },
        INTERVAL_MS,
        active && visible && !denied,
    );

    return { track: setElement, stats: active ? stats : null };
}
