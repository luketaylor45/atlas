import type { Egg, Nest } from '../../../lib/types';

/** A root nest with the sub-nests that sit under it. */
export interface NestGroup {
    root: Nest;
    children: Nest[];
}

const byName = (a: { name: string }, b: { name: string }) => a.name.localeCompare(b.name);

/**
 * Turns the flat list from `GET /admin/nests` into root nests with their
 * sub-nests. A nest whose parent is missing is treated as a root so it can
 * never disappear from the page.
 */
export function groupNests(nests: Nest[]): NestGroup[] {
    const ids = new Set(nests.map((nest) => nest.id));
    const isRoot = (nest: Nest) => nest.parent_id == null || !ids.has(nest.parent_id);

    return nests
        .filter(isRoot)
        .sort(byName)
        .map((root) => ({
            root,
            children: nests.filter((nest) => !isRoot(nest) && nest.parent_id === root.id).sort(byName),
        }));
}

/** The eggs that sit directly in a nest, by name. */
export function nestEggs(nest: Nest): Egg[] {
    return [...(nest.eggs ?? [])].sort(byName);
}

/** Root nests only, for choosing a parent. */
export function rootNests(nests: Nest[]): Nest[] {
    return groupNests(nests).map((group) => group.root);
}
