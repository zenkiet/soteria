// parent is inlined so this file imports nothing and `node --test` can run the rules directly;
// the copy in shared/lib/format.ts pulls in a rune store.
const parent = (p: string) => p.slice(0, p.lastIndexOf('/')) || '/';

// movable returns the dragged items that a drop on folder `to` would actually move: nothing when
// `to` is one of the dragged folders or sits inside one, and never the items already living there.
export function movable<T extends { path: string; dir: boolean }>(items: T[], to: string): T[] {
	if (items.some((e) => e.dir && (to === e.path || to.startsWith(e.path + '/')))) return [];
	return items.filter((e) => parent(e.path) !== to);
}
