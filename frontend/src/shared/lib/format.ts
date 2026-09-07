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
export const when = (iso: string) => (iso.startsWith('0001') ? '—' : date.format(new Date(iso)));

import { OFFLINE, lost } from './net.svelte';

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

const ext = (name: string) => name.slice(name.lastIndexOf('.') + 1).toLowerCase();

const icons: Record<string, 'image' | 'video' | 'archive' | 'sheet' | 'doc'> = {
	jpg: 'image',
	jpeg: 'image',
	png: 'image',
	gif: 'image',
	webp: 'image',
	heic: 'image',
	svg: 'image',
	mp4: 'video',
	mov: 'video',
	mkv: 'video',
	webm: 'video',
	zip: 'archive',
	gz: 'archive',
	tar: 'archive',
	rar: 'archive',
	'7z': 'archive',
	xlsx: 'sheet',
	xls: 'sheet',
	csv: 'sheet',
	numbers: 'sheet',
	pdf: 'doc',
	md: 'doc',
	txt: 'doc',
	doc: 'doc',
	docx: 'doc'
};

type Named = { name: string; dir: boolean; contentType?: string };

export const iconFor = (e: Named) => (e.dir ? 'folderFill' : (icons[ext(e.name)] ?? 'file'));

export const kind = (e: Named) =>
	e.dir ? 'Folder' : e.name.includes('.') ? ext(e.name).toUpperCase() : e.contentType || 'File';

export const parent = (p: string) => p.slice(0, p.lastIndexOf('/')) || '/';
export const join = (dir: string, name: string) => (dir === '/' ? '' : dir) + '/' + name;
export const filesHref = (p: string) => '/files' + p.split('/').map(encodeURIComponent).join('/');

export type Preview = 'image' | 'pdf' | 'video' | 'audio' | 'text' | 'docx';

const previews = Object.fromEntries(
	Object.entries({
		image: 'jpg jpeg png gif webp heic bmp avif svg',
		pdf: 'pdf',
		video: 'mp4 mov m4v webm',
		audio: 'mp3 m4a wav aac flac',
		text: 'txt md json log csv xml yaml yml toml ini conf js ts go py css html sh sql',
		docx: 'docx'
	}).flatMap(([k, v]) => v.split(' ').map((x) => [x, k]))
) as Record<string, Preview>;

export const previewKind = (name: string): Preview | null => previews[ext(name)] ?? null;
export const previewUrl = (path: string) => '/preview?path=' + encodeURIComponent(path);
export const thumbUrl = (path: string, v: string) =>
	'/thumb?path=' + encodeURIComponent(path) + '&v=' + encodeURIComponent(v);

export const ago = (s: number) => {
	const m = Math.round(Date.now() / 60000 - s / 60);
	return m < 1 ? 'just now' : m < 60 ? `${m} min ago` : `${Math.round(m / 60)} h ago`;
};
