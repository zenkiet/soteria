import { prefs, setPrefs } from './prefs.svelte';

type Star = { path: string; dir: boolean };

const under = (p: string, root: string) => p === root || p.startsWith(root + '/');
const save = (id: string, list: Star[]) => setPrefs({ starred: { ...prefs.starred, [id]: list } });

export const starred = (id: string) => prefs.starred[id] ?? [];
export const isStarred = (id: string, p: string) => starred(id).some((s) => s.path === p);

// A mixed selection gets starred rather than flipped item by item.
export function toggleStar(id: string, items: Star[]) {
	const all = items.every((e) => isStarred(id, e.path));
	const rest = starred(id).filter((s) => !items.some((e) => e.path === s.path));
	save(id, all ? rest : [...rest, ...items.map(({ path, dir }) => ({ path, dir }))]);
}

export const restar = (id: string, from: string, to: string) =>
	save(
		id,
		starred(id).map((s) =>
			under(s.path, from) ? { ...s, path: to + s.path.slice(from.length) } : s
		)
	);

export const unstar = (id: string, paths: string[]) =>
	save(
		id,
		starred(id).filter((s) => !paths.some((p) => under(s.path, p)))
	);
