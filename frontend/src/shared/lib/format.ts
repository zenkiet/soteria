export function bytes(n: number) {
	if (n < 0) return '—';
	const units = ['B', 'KB', 'MB', 'GB', 'TB'];
	let i = 0;
	while (n >= 1000 && i < units.length - 1) {
		n /= 1000;
		i++;
	}
	return `${i ? n.toFixed(n < 10 ? 1 : 0) : n} ${units[i]}`;
}

const date = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' });
const time = new Intl.DateTimeFormat(undefined, { timeStyle: 'short' });
// Finder's wording: "Today at 9:41 AM", "Yesterday at …", then the full date.
export const when = (iso: string) => {
	if (iso.startsWith('0001')) return '—';
	const d = new Date(iso);
	const days = Math.round(
		(new Date().setHours(0, 0, 0, 0) - new Date(d).setHours(0, 0, 0, 0)) / 864e5
	);
	return days === 0
		? `Today at ${time.format(d)}`
		: days === 1
			? `Yesterday at ${time.format(d)}`
			: date.format(d);
};

import { OFFLINE, lost } from './net.svelte';
import { mac } from './platform';
import { prefs } from './prefs.svelte';

export const msg = (e: unknown) => {
	const m = e instanceof Error ? e.message : String(e);
	if (m === OFFLINE) lost();
	return m;
};

// validName returns why a file name can't be used, or '' when it can.
export const validName = (name: string, taken: string[]) => {
	const n = name.trim();
	if (!n) return 'Enter a name.';
	if (n.includes('/')) return 'Names can’t contain “/”.';
	if (n === '.' || n === '..') return 'That name is reserved.';
	if (new TextEncoder().encode(n).length > 255) return 'That name is too long.';
	if (taken.includes(n)) return `Something named “${n}” already exists here.`;
	return '';
};

export const ext = (name: string) => name.slice(name.lastIndexOf('.') + 1).toLowerCase();

const table = <K extends string>(o: Record<K, string>) =>
	Object.fromEntries(
		Object.entries<string>(o).flatMap(([k, v]) => v.split(' ').map((x) => [x, k]))
	) as Record<string, K>;

type Named = { name: string; dir: boolean; contentType?: string };

export const kind = (e: Named) =>
	e.dir ? 'Folder' : e.name.includes('.') ? ext(e.name).toUpperCase() : e.contentType || 'File';

export const parent = (p: string) => p.slice(0, p.lastIndexOf('/')) || '/';
export const base = (p: string) => p.slice(p.lastIndexOf('/') + 1);
export const join = (dir: string, name: string) => (dir === '/' ? '' : dir) + '/' + name;
export const filesHref = (p: string) =>
	'/files' + (p === '/' ? '' : p.split('/').map(encodeURIComponent).join('/'));
export const focusHref = (p: string) => filesHref(parent(p)) + '?focus=' + encodeURIComponent(p);
export const home = (s: { id: string }) => filesHref(prefs.lastDir[s.id] || '/');
// Finder and Dropbox keep these out of sight: dotfiles, Office lock files and Windows thumbnail caches.
export const listed = (e: { name: string }) =>
	prefs.hidden || !/^(\.|~\$)|^(thumbs\.db|desktop\.ini)$/i.test(e.name);
export const plural = (n: number, w: string) => `${n} ${w}${n === 1 ? '' : 's'}`;
export const collator = new Intl.Collator(undefined, { numeric: true, sensitivity: 'base' });
export const sum = <T>(xs: T[], f: (x: T) => number) => xs.reduce((s, x) => s + f(x), 0);

const previews = table({
	// Only WebKit decodes HEIC; WebView2 shows a broken image.
	image: `jpg jpeg png gif webp bmp avif svg${mac ? ' heic' : ''}`,
	pdf: 'pdf',
	video: 'mp4 mov m4v webm',
	audio: 'mp3 m4a wav aac flac',
	text: 'txt json log xml yaml yml toml ini conf js ts go py css html sh sql',
	md: 'md',
	csv: 'csv',
	docx: 'docx'
});

export const previewKind = (name: string) => previews[ext(name)] ?? null;
export const thumbUrl = (path: string, v: string) =>
	'/thumb?path=' + encodeURIComponent(path) + '&v=' + encodeURIComponent(v);

export const ago = (s: number) => {
	const m = Math.round(Date.now() / 60000 - s / 60);
	return m < 1 ? 'just now' : m < 60 ? `${m} min ago` : `${Math.round(m / 60)} h ago`;
};
