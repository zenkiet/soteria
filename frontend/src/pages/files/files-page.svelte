<script module lang="ts">
	// Back, Forward and the Files link land where each folder was left.
	const scrolls: Record<string, number> = {};
</script>

<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import SelectionArea, { type SelectionEvent } from '@viselect/vanilla';
	import { Clipboard, Events } from '@wailsio/runtime';
	import { tick, untrack } from 'svelte';
	import { prefersReducedMotion } from 'svelte/motion';
	import { SvelteSet } from 'svelte/reactivity';
	import { fade } from 'svelte/transition';
	import {
		changes,
		Copy,
		DragOut,
		FolderUsages,
		List,
		Mkdir,
		Move,
		PickUploads,
		Reindex,
		Restore,
		Search,
		Trash,
		indexStatus,
		Upload,
		UploadAs,
		type Conflict,
		type Entry,
		type Server,
		type Usage
	} from '#/shared/api/index.ts';
	import {
		ago,
		bytes,
		collator,
		filesHref,
		focusHref,
		isStarred,
		listed,
		join,
		kind,
		mac,
		msg,
		net,
		parent,
		plural,
		prefs,
		previewKind,
		restar,
		setPrefs,
		thumbUrl,
		toast,
		toggleStar,
		unstar,
		validName,
		when,
		type SortKey
	} from '#/shared/lib/index.ts';
	import { Dialog, FileIcon, Icon, SelectionBar, Split, type IconName } from '#/shared/ui/index.ts';
	import { Preview, downloadAll, openFile } from '#/widgets/preview/index.ts';
	import ConflictDialog, { type Resolution } from './ui/conflict-dialog.svelte';
	import InfoPanel from './ui/info-panel.svelte';
	import MoveDialog from './ui/move-dialog.svelte';

	let { path = '' }: { path?: string } = $props();
	const dir = $derived('/' + path.replace(/\/+$/, ''));
	const crumbs = $derived(dir === '/' ? [] : dir.slice(1).split('/'));
	const trail = $derived(
		crumbs.map((name, i) => ({ name, path: '/' + crumbs.slice(0, i + 1).join('/') }))
	);
	const server = $derived(page.data.server as Server);

	let entries = $state.raw<Entry[]>([]);
	let usage = $state.raw<{ [path: string]: Usage | undefined }>({});
	let loading = $state(true);
	let error = $state('');
	let query = $state('');
	const sel = new SvelteSet<string>();
	const select = (paths: string[]) => {
		sel.clear();
		for (const p of paths) sel.add(p);
	};
	let anchor = '';
	let preview = $state<Entry | null>(null);
	let menu = $state<{ x: number; y: number; items: Entry[] } | null>(null);
	let pop = $state<'' | 'sort' | 'upload' | 'more' | 'crumbs'>('');
	let help = $state(false);
	let scroller = $state<HTMLElement>();
	let searchInput = $state<HTMLInputElement>();
	let dialog = $state<'' | 'mkdir' | 'rename' | 'move' | 'copy'>('');
	let targets = $state<Entry[]>([]);
	let name = $state('');
	let scope = $state<'folder' | 'all'>('folder');
	let type = $state('');
	let within = $state(0);
	let hits = $state.raw<Entry[]>([]);
	let total = $state(0);
	let flash = $state('');
	let nameInput = $state<HTMLInputElement>();
	const idx = $derived(indexStatus.current);
	let conflicts = $state<Conflict[]>([]);
	let clip = $state<{ items: Entry[]; cut: boolean } | null>(null);
	let loadedAt = 0;
	let loaded = '';

	const SORTS: [SortKey, string][] = [
		['name', 'Name'],
		['kind', 'Kind'],
		['size', 'Size'],
		['modified', 'Date modified'],
		['created', 'Date created']
	];
	const sortLabel = $derived(SORTS.find(([k]) => k === prefs.sort)?.[1] ?? 'Name');

	const entrySize = (e: Entry) => {
		const u = usage[e.path];
		return !e.dir ? e.size : u?.known ? u.bytes : null;
	};
	const shownSize = (e: Entry) => {
		const n = entrySize(e);
		return n === null ? '—' : bytes(n);
	};

	function compare(a: Entry, b: Entry) {
		const k = prefs.sort;
		let r =
			k === 'size'
				? (entrySize(a) ?? -1) - (entrySize(b) ?? -1)
				: k === 'modified'
					? Date.parse(a.modified) - Date.parse(b.modified)
					: k === 'created'
						? Date.parse(a.created) - Date.parse(b.created)
						: k === 'kind'
							? collator.compare(kind(a), kind(b))
							: 0;
		if (!r) r = collator.compare(a.name, b.name);
		return prefs.asc ? r : -r;
	}

	function sortBy(k: SortKey) {
		setPrefs(prefs.sort === k ? { asc: !prefs.asc } : { sort: k, asc: true });
	}

	const sorted = $derived(entries.filter(listed).sort(compare));
	const q = $derived(query.trim());
	const visible = $derived(sorted.filter((e) => e.name.toLowerCase().includes(q.toLowerCase())));
	const folders = $derived(visible.filter((e) => e.dir));
	const files = $derived(visible.filter((e) => !e.dir));
	const rows = $derived([...folders, ...files]);
	const searching = $derived(scope === 'all' && (!!q || !!type || !!within));
	const shown = $derived(searching ? hits : rows);
	const viewable = $derived(shown.filter((e) => !e.dir));
	const picked = $derived(shown.filter((e) => sel.has(e.path)));
	const single = $derived(picked.length === 1 ? picked[0] : null);

	async function loadUsage(d: string) {
		const u = (await FolderUsages(d)) ?? {};
		if (dir === d) usage = u;
	}

	// A newer load cancels the one in flight; the old folder stays on screen until the new one lands.
	let listing: ReturnType<typeof List> | null = null;
	async function load(d: string, f: string | null = null, open = false) {
		listing?.cancel();
		const call = (listing = List(d));
		loading = true;
		try {
			const list = (await call) ?? [];
			if (call !== listing) return;
			entries = list;
			loaded = d;
			const keep = new Set(list.map((x) => x.path));
			if (!searching) for (const p of sel) if (!keep.has(p)) sel.delete(p);
			loadUsage(d);
			error = '';
			// Only a folder that loaded becomes the one the Files link returns to.
			setPrefs({ lastDir: { ...prefs.lastDir, [server.id]: d } });
			if (f) {
				select([f]);
				anchor = cursor = f;
			}
			await tick();
			if (f) {
				document
					.querySelector(`[data-path="${CSS.escape(f)}"]`)
					?.scrollIntoView({ block: 'center' });
				if (open) preview = list.find((x) => x.path === f) ?? null;
			} else if (scroller) scroller.scrollTop = scrolls[d] ?? 0;
		} catch (e) {
			if (call !== listing) return;
			const m = msg(e);
			// Offline, the banner says the last listing stays; only a folder never loaded shows the error.
			error = net.offline && d === loaded ? '' : m;
		}
		loading = false;
		loadedAt = Date.now();
	}

	const refresh = () => load(dir);

	const nameError = $derived(
		dialog === 'mkdir' || dialog === 'rename'
			? validName(
					name,
					entries.filter((e) => e.path !== targets[0]?.path).map((e) => e.name)
				)
			: ''
	);

	// Read here, not in load: a focus link applies once, and one into the open folder still lands.
	$effect(() => {
		const f = page.url.searchParams.get('focus');
		const open = page.url.searchParams.has('open');
		sel.clear();
		preview = null;
		flash = '';
		query = '';
		scope = 'folder';
		type = '';
		within = 0;
		load(dir, f, open);
	});

	let pending: ReturnType<typeof Search> | undefined;
	function search() {
		pending?.cancel();
		pending = Search(q, type, within && Math.floor(Date.now() / 1000) - within, prefs.hidden);
		pending.then(
			(r) => {
				hits = r.entries ?? [];
				total = r.total;
			},
			() => {}
		);
	}

	$effect(() => {
		if (searching) search();
	});

	$effect(() => {
		if (net.back) load(dir);
	});

	$effect(() => {
		if (idx?.done)
			untrack(() => {
				loadUsage(dir);
				if (searching) search();
			});
	});

	$effect(() => Events.On('dropped', (ev) => upload(Upload(ev.data.files, ev.data.dir || dir))));

	let reload: ReturnType<typeof setTimeout>;
	$effect(() => {
		const off = Events.On('transfers', (ev) => {
			for (const t of ev.data ?? []) {
				if (t.kind === 'upload' && t.status === 'done' && parent(t.remote) === dir) {
					clearTimeout(reload);
					reload = setTimeout(() => load(dir), 500);
				}
			}
		});
		return () => {
			off();
			clearTimeout(reload);
		};
	});

	function queueConflicts(cs: Conflict[] | null) {
		if (cs?.length) conflicts = [...conflicts, ...cs];
	}

	function upload(p: Promise<Conflict[] | null>) {
		p.then(queueConflicts, (e) => toast(msg(e), 'error'));
	}

	function resolve(mode: Resolution, all: boolean) {
		const batch = all ? conflicts : conflicts.slice(0, 1);
		conflicts = all ? [] : conflicts.slice(1);
		for (const c of batch) {
			if (mode !== 'skip') UploadAs(c.local, c.remote, mode).catch((e) => toast(msg(e), 'error'));
		}
	}

	async function run(action: () => Promise<unknown>, done: string, undo?: () => Promise<unknown>) {
		try {
			await action();
			changes.n++;
			toast(done, 'ok', undo && { label: 'Undo', run: () => run(undo, 'Undone') });
			await load(dir);
		} catch (e) {
			toast(msg(e), 'error');
		}
		dialog = '';
	}

	const move = (from: string, to: string) => Move(from, to).then(() => restar(server.id, from, to));

	function submit(e: SubmitEvent) {
		e.preventDefault();
		if (nameError) return;
		const n = name.trim();
		if (dialog === 'mkdir')
			run(() => Mkdir(join(dir, n)), `Created ${n}`).then(() => reveal(join(dir, n)));
		else {
			const t = targets[0];
			const to = join(parent(t.path), n);
			run(
				() => move(t.path, to),
				`Renamed to ${n}`,
				() => move(to, t.path)
			).then(() => reveal(to));
		}
	}

	// Like Finder, the result of an action is selected, scrolled to and briefly highlighted.
	async function reveal(p: string) {
		if (!entries.some((e) => e.path === p)) return;
		select([p]);
		anchor = cursor = flash = p;
		await tick();
		document.querySelector(`[data-path="${CSS.escape(p)}"]`)?.scrollIntoView({ block: 'nearest' });
	}

	// All calls run at once; the first failure is thrown only after every call has settled.
	async function each<T>(items: T[], f: (x: T) => Promise<unknown>) {
		const failed = (await Promise.allSettled(items.map(f))).find((r) => r.status === 'rejected');
		if (failed) throw failed.reason;
	}

	async function trashAll(items: Entry[]) {
		sel.clear();
		const moved: string[] = [];
		try {
			await each(items, async (e) => {
				moved.push(await Trash(e.path));
				unstar(server.id, [e.path]);
			});
		} catch (e) {
			toast(msg(e), 'error');
		}
		changes.n++;
		await load(dir);
		if (moved.length)
			toast(
				moved.length === 1
					? `Moved “${items[0].name}” to Trash`
					: `Moved ${moved.length} items to Trash`,
				'ok',
				{
					label: 'Undo',
					run: () => run(() => each(moved, Restore), 'Restored')
				}
			);
	}

	function moveAll(dest: string, items = targets) {
		run(
			() => each(items, (e) => move(e.path, join(dest, e.name))),
			`Moved to ${dest}`,
			() => each(items, (e) => move(join(dest, e.name), e.path))
		);
	}

	async function copyAll(dest: string) {
		const items = targets;
		const taken = ((await List(dest).catch(() => [])) ?? []).map((x) => x.name);
		run(() => each(items, (e) => Copy(e.path, join(dest, free(e, taken)))), `Copied to ${dest}`);
	}

	// The name where free, else "name copy.ext", "name copy 2.ext"…; taken grows so one batch never reuses a name.
	function free(e: Entry, taken: string[]) {
		const i = e.dir ? -1 : e.name.lastIndexOf('.');
		const stem = i > 0 ? e.name.slice(0, i) : e.name;
		const ext = i > 0 ? e.name.slice(i) : '';
		let n = e.name;
		for (let k = 1; taken.includes(n); k++) n = `${stem} copy${k > 1 ? ` ${k}` : ''}${ext}`;
		taken.push(n);
		return n;
	}

	function duplicate(e: Entry) {
		const taken = entries.map((x) => x.name);
		const to = join(dir, free(e, taken));
		run(() => Copy(e.path, to), `Duplicated ${e.name}`).then(() => reveal(to));
	}

	function paste() {
		const c = clip;
		if (!c) return;
		if (c.cut) clip = null;
		const items = c.items.filter((e) => !c.cut || parent(e.path) !== dir);
		if (!items.length) return;
		const taken = entries.map((x) => x.name);
		run(
			() =>
				each(items, (e) =>
					c.cut ? move(e.path, join(dir, e.name)) : Copy(e.path, join(dir, free(e, taken)))
				),
			`${c.cut ? 'Moved' : 'Pasted'} ${plural(items.length, 'item')}`
		);
	}

	function open(kind: typeof dialog, items: Entry[]) {
		targets = items;
		name = kind === 'rename' ? items[0].name : '';
		dialog = kind;
		menu = null;
		// Finder selects the name without its extension, so typing replaces just that.
		const dot = items[0]?.dir ? -1 : name.lastIndexOf('.');
		if (kind === 'rename')
			tick().then(() => nameInput?.setSelectionRange(0, dot > 0 ? dot : name.length));
	}

	async function copyLink(e: Entry) {
		// soteria://files/<path> mirrors the SPA route; a file links to its folder, focused.
		const route = e.dir ? filesHref(e.path) : focusHref(e.path);
		await Clipboard.SetText('soteria:/' + route);
		toast('Link copied');
	}

	let cursor = '';
	function pick(e: Entry, ev: MouseEvent | KeyboardEvent, toggle = false) {
		cursor = e.path;
		if (ev.shiftKey && anchor) {
			const a = shown.findIndex((x) => x.path === anchor);
			if (a >= 0) {
				const b = shown.findIndex((x) => x.path === e.path);
				select(shown.slice(Math.min(a, b), Math.max(a, b) + 1).map((x) => x.path));
				return;
			}
			// the anchored row is gone (moved, deleted, filtered): fall through and re-anchor
		}
		if (!(toggle || ev.metaKey || ev.ctrlKey)) select([e.path]);
		else if (!sel.delete(e.path)) sel.add(e.path);
		anchor = e.path;
	}

	// A double-click's second click would otherwise open whatever the first one put under the pointer.
	let openedAt = -Infinity;
	function openEntry(e: Entry) {
		if (performance.now() - openedAt < 500) return;
		openedAt = performance.now();
		if (e.dir) goto(filesHref(e.path));
		else preview = e;
	}

	function showAreaMenu(ev: MouseEvent) {
		ev.preventDefault();
		menu = { x: ev.clientX, y: ev.clientY, items: [] };
	}

	// A popover keeps the menu above toasts and panels; it can only be clamped once it has a size.
	const at = (x: number, y: number) => (el: HTMLElement) => {
		if (!el.matches(':popover-open')) el.showPopover();
		el.style.left = `${Math.min(x, innerWidth - el.offsetWidth - 8)}px`;
		el.style.top = `${Math.min(y, innerHeight - el.offsetHeight - 8)}px`;
	};

	const entryProps = (e: Entry) => ({
		role: 'button',
		tabindex: 0,
		'data-path': e.path,
		'data-selected': sel.has(e.path) || undefined,
		'data-flash': flash === e.path || undefined,
		'data-dragging': (drag && sel.has(e.path)) || undefined,
		'data-file-drop-target': e.dir && !draggingOut ? true : undefined,
		'data-drop': e.dir ? e.path : undefined,
		onpointerdown: (ev: PointerEvent) => press(ev, e),
		onclick: (ev: MouseEvent) => {
			if (dragged) return;
			if (
				(ev.target as Element).closest('[data-name]') &&
				!(ev.metaKey || ev.ctrlKey || ev.shiftKey)
			)
				openEntry(e);
			else pick(e, ev);
		},
		onkeydown: (ev: KeyboardEvent) => ev.key === 'Enter' && openEntry(e),
		ondblclick: () => openEntry(e),
		oncontextmenu: (ev: MouseEvent) => showMenu(ev, e)
	});

	function showMenu(ev: MouseEvent, e: Entry) {
		ev.preventDefault();
		ev.stopPropagation();
		if (!sel.has(e.path)) {
			select([e.path]);
			anchor = e.path;
		}
		menu = { x: ev.clientX, y: ev.clientY, items: picked };
	}

	function keys(ev: KeyboardEvent) {
		if (drag && ev.key === ' ') {
			clearTimeout(spring);
			springOpen();
			ev.preventDefault();
			return;
		}
		if (ev.key === 'Escape') {
			cancelDrag?.();
			if (!menu && !pop && !document.querySelector('dialog[open]')) {
				if (picked.length) sel.clear();
				else {
					query = '';
					scope = 'folder';
				}
			}
			menu = null;
			pop = '';
			return;
		}
		const t = ev.target as Element;
		if (t.matches('input:not([type=checkbox]),select') || document.querySelector('dialog[open]'))
			return;
		const mod = ev.metaKey || ev.ctrlKey;
		const step = { ArrowDown: 1, ArrowUp: -1 }[ev.key] ?? 0;
		const k = ev.key.toLowerCase();
		if (step && (mod || ev.altKey)) {
			if (step < 0 && dir !== '/') goto(filesHref(parent(dir)));
		} else if (step) {
			const i = shown.findIndex((e) => e.path === cursor);
			const e = shown[Math.max(0, Math.min(shown.length - 1, i + step))];
			if (!e) return;
			pick(e, ev);
			document.querySelector<HTMLElement>(`[data-path="${CSS.escape(e.path)}"]`)?.focus();
		} else if (ev.key === ' ' && !t.matches('button,input')) {
			if (single && !single.dir) preview = single;
			else return;
		} else if (ev.key === 'Delete' || (mod && ev.key === 'Backspace')) {
			if (picked.length) trashAll(picked);
			else return;
		} else if (ev.key === 'F2') {
			if (single) open('rename', [single]);
			else return;
		} else if (ev.key === 'F5') refresh();
		else if (ev.key === '/') searchInput?.focus();
		else if (ev.key === '?') help = true;
		else if (!mod) return;
		else if (k === 'a') select(shown.map((e) => e.path));
		else if (k === 'f') searchInput?.focus();
		else if ((k === 'c' || k === 'x') && picked.length) clip = { items: picked, cut: k === 'x' };
		else if (k === 'v') paste();
		else if (k === 'r') refresh();
		else if (k === 'i') toggleInfo();
		else if (k === 'u') upload(PickUploads(dir, false));
		else if (k === 'n' && ev.shiftKey) open('mkdir', []);
		else return;
		ev.preventDefault();
	}

	function clicked(ev: MouseEvent) {
		menu = null;
		pop = '';
		if (
			!dragged &&
			!(ev.target as Element).closest(
				'[role=button],[role=separator],button,a,input,label,aside,dialog,select'
			)
		)
			sel.clear();
	}

	const toggleInfo = () => setPrefs({ infoHidden: !prefs.infoHidden });
	const vanish = (node: HTMLElement) => {
		node.inert = true;
		return fade(node, { duration: prefersReducedMotion.current ? 0 : 120 });
	};
	const togglePop = (p: typeof pop) => (ev: MouseEvent) => {
		ev.stopPropagation();
		pop = pop === p ? '' : p;
	};

	let dragged = false;
	const pathOf = (el: Element) => (el as HTMLElement).dataset.path!;
	// While dragging only the DOM changes: every SvelteSet write re-runs each unselected row.
	const paint = ({ store: { changed } }: SelectionEvent) => {
		for (const el of changed.added) el.toggleAttribute('data-selected', true);
		for (const el of changed.removed) el.toggleAttribute('data-selected', sel.has(pathOf(el)));
	};

	function marquee(el: HTMLElement) {
		const area = new SelectionArea({
			selectables: ['[data-path]'],
			startAreas: [el],
			boundaries: [el],
			features: { touch: false, singleTap: { allow: false } }
		})
			// Rows start their own drag, and a press on the scrollbar still scrolls.
			.on('beforestart', ({ event }) => {
				const t = event!.target as Element;
				return (
					!t.closest('[data-path],button,a,input,label') &&
					!(t === el && (event as MouseEvent).offsetX >= el.clientWidth)
				);
			})
			.on('start', ({ event }) => {
				// Overlap "invert" would otherwise bring back what the previous drag selected.
				area.clearSelection(true, true);
				if (!(event?.shiftKey || event?.metaKey || event?.ctrlKey)) sel.clear();
			})
			.on('move', paint)
			.on('stop', (e) => {
				paint(e);
				for (const x of e.store.selected) sel.add(pathOf(x));
				const last = e.store.selected.at(-1);
				if (last) anchor = cursor = pathOf(last);
				// The click that ends the drag would reach clicked() and clear the selection.
				dragged = true;
				setTimeout(() => (dragged = false));
			});
		return () => area.destroy();
	}

	let draggingOut = $state(false);
	$effect(() => Events.On('dragend', () => (draggingOut = false)));

	function dragOut(items: Entry[]) {
		draggingOut = true;
		DragOut(items).catch((err) => {
			draggingOut = false;
			toast(msg(err), 'error');
		});
	}

	let drag = $state<{ items: Entry[]; x: number; y: number } | null>(null);
	let into: Element | null = null;
	let cancelDrag: (() => void) | undefined;
	let spring: ReturnType<typeof setTimeout> | undefined;
	let blink: ReturnType<typeof setTimeout> | undefined;

	// Like Finder's spring-loaded folders: resting on a folder mid-drag opens it and the drag carries on.
	const springOpen = () => {
		const to = into?.getAttribute('data-drop');
		if (to && to !== dir) goto(filesHref(to));
	};

	// Nothing moves into itself, the folder it is already in, or one of its own subfolders.
	function target(x: number, y: number, items: Entry[]) {
		const el = document.elementFromPoint(x, y)?.closest('[data-drop]');
		const to = el?.getAttribute('data-drop');
		return el &&
			to &&
			items.every((e) => parent(e.path) !== to && to !== e.path && !to.startsWith(e.path + '/'))
			? el
			: null;
	}

	// Inside the window a drag moves items between folders; past the edge it becomes the OS drag to Finder or Explorer.
	function press(down: PointerEvent, e: Entry) {
		if (down.button || (down.target as Element).closest('button,input')) return;
		const stop = () => {
			removeEventListener('pointermove', track);
			removeEventListener('pointerup', release);
			removeEventListener('mouseout', leave);
			clearTimeout(spring);
			clearTimeout(blink);
			into?.classList.remove('drop-into', 'spring-soon');
			into = null;
			drag = null;
			cancelDrag = undefined;
		};
		const out = () => {
			const items = drag!.items;
			stop();
			dragOut(items);
		};
		const leave = (ev: MouseEvent) => drag && !ev.relatedTarget && out();
		const track = (ev: PointerEvent) => {
			if (!drag) {
				if (Math.hypot(ev.clientX - down.clientX, ev.clientY - down.clientY) < 5) return;
				if (!sel.has(e.path)) select([e.path]);
				drag = { items: picked, x: ev.clientX, y: ev.clientY };
			}
			if (ev.clientX < 0 || ev.clientY < 0 || ev.clientX >= innerWidth || ev.clientY >= innerHeight)
				return out();
			drag.x = ev.clientX;
			drag.y = ev.clientY;
			const el = target(ev.clientX, ev.clientY, drag.items);
			if (el !== into) {
				into?.classList.remove('drop-into', 'spring-soon');
				el?.classList.add('drop-into');
				into = el;
				clearTimeout(spring);
				clearTimeout(blink);
				if (el) {
					spring = setTimeout(springOpen, 3000);
					// Finder flashes a folder just before it springs open.
					blink = setTimeout(() => el.classList.add('spring-soon'), 2200);
				}
			}
		};
		const release = () => {
			const to = into?.getAttribute('data-drop');
			const items = drag?.items;
			stop();
			if (!items) return;
			dragged = true;
			setTimeout(() => (dragged = false));
			if (to) moveAll(to, items);
		};
		cancelDrag = stop;
		addEventListener('pointermove', track);
		addEventListener('pointerup', release);
		addEventListener('mouseout', leave);
	}

	// Size, then Modified, give way before the name does.
	const cols =
		'grid-cols-[1rem_minmax(0,1fr)_170px_90px_7rem] @max-2xl:grid-cols-[1rem_minmax(0,1fr)_170px_7rem] @max-xl:grid-cols-[1rem_minmax(0,1fr)_7rem]';
	const mod = mac ? '⌘' : 'Ctrl+';
	const day = 86_400;
	const TYPES = [
		['', 'Any type'],
		['folders', 'Folders'],
		['images', 'Images'],
		['documents', 'Documents'],
		['spreadsheets', 'Spreadsheets'],
		['presentations', 'Presentations'],
		['videos', 'Videos'],
		['audio', 'Audio'],
		['archives', 'Archives']
	] as const;
	const WITHIN = [
		[0, 'Any time'],
		[day, 'Last 24 hours'],
		[7 * day, 'Last 7 days'],
		[30 * day, 'Last 30 days'],
		[365 * day, 'Last 12 months']
	] as const;
	const SHORTCUTS = [
		['↑ ↓', 'Move between items'],
		['Enter', 'Open'],
		['Space', 'Preview'],
		[`${mod}A`, 'Select all'],
		['/', 'Search'],
		['F2', 'Rename'],
		[mac ? '⌘⌫' : 'Delete', 'Delete (move to Trash)'],
		[`${mod}C ${mod}X ${mod}V`, 'Copy, cut, paste'],
		[mac ? '⇧⌘N' : 'Ctrl+Shift+N', 'New folder'],
		[`${mod}U`, 'Upload files'],
		[`${mod}I`, 'Show info'],
		[mac ? '⌘↑' : 'Alt+↑', 'Enclosing folder'],
		[mac ? '⌘[ ⌘]' : 'Alt+← Alt+→', 'Back, forward'],
		[mac ? '⌃⌘S' : 'Ctrl+Shift+S', 'Show or hide the sidebar'],
		['F5', 'Refresh']
	] as const;
</script>

<svelte:window
	onclick={clicked}
	onkeydown={keys}
	onfocus={() => {
		if (Date.now() - loadedAt > 5000) load(dir);
	}}
/>

{#snippet head(k: SortKey, text: string, cls = 'flex')}
	<button class="items-center gap-1 hover:text-fg {cls}" onclick={() => sortBy(k)}>
		{text}
		{#if prefs.sort === k}<Icon
				name="chevronDown"
				size={12}
				class={prefs.asc ? '' : 'rotate-180'}
			/>{/if}
	</button>
{/snippet}

{#snippet opt(label: string, on: boolean, run: () => void)}
	<button class="menu-item" onclick={run}>
		<span class="flex-1">{label}</span>
		{#if on}<Icon name="check" size={14} class="text-accent-fg" />{/if}
	</button>
{/snippet}

{#snippet item(icon: IconName, label: string, run: () => unknown, key = '', disabled = false)}
	<button class="menu-item" {disabled} onclick={run}>
		<Icon name={icon} size={15} class="text-fg-2" />{label}{#if key}<kbd class="ml-auto">{key}</kbd
			>{/if}
	</button>
{/snippet}

{#snippet act(icon: IconName, label: string, run: () => unknown, cls = '')}
	<button
		class="btn btn-ghost h-8 shrink-0 @max-[46rem]:w-8 @max-[46rem]:px-0 {cls}"
		onclick={run}
		title={label}
		aria-label={label}
	>
		<Icon name={icon} size={15} /><span class="@max-[46rem]:hidden">{label}</span>
	</button>
{/snippet}

{#snippet tool(icon: IconName, label: string, run: (ev: MouseEvent) => unknown)}
	<button
		class="btn btn-ghost h-7 w-7 px-0 text-fg-3"
		onclick={(ev) => {
			ev.stopPropagation();
			run(ev);
		}}
		aria-label={label}
		title={label}><Icon name={icon} size={15} /></button
	>
{/snippet}

{#snippet check(e: Entry)}
	<input
		type="checkbox"
		class="check {picked.length
			? ''
			: 'opacity-0 group-focus-within:opacity-100 group-hover:opacity-100'}"
		checked={sel.has(e.path)}
		onclick={(ev) => {
			ev.stopPropagation();
			pick(e, ev, true);
		}}
		aria-label="Select {e.name}"
	/>
{/snippet}

{#snippet title(e: Entry)}
	<span class="truncate hover:underline" data-name title={e.name}>{e.name}</span>
	{#if isStarred(server.id, e.path)}<Icon name="starFill" size={12} class="text-warn" />{/if}
{/snippet}

{#snippet actions(e: Entry)}
	{@const star = isStarred(server.id, e.path)}
	<div class="flex items-center justify-end">
		<div
			class="flex opacity-0 group-focus-within:opacity-100 group-hover:opacity-100 motion-safe:transition-opacity"
		>
			{@render tool('download', 'Download', () => downloadAll([e]))}
			{@render tool('link', 'Copy link', () => copyLink(e))}
			{@render tool(star ? 'starFill' : 'star', star ? 'Unstar' : 'Star', () =>
				toggleStar(server.id, [e])
			)}
		</div>
		{@render tool('dots', 'Actions', (ev) => showMenu(ev, e))}
	</div>
{/snippet}

{#snippet meta(e: Entry)}
	<div class="truncate text-xs text-fg-2 @max-xl:hidden">{when(e.modified)}</div>
	<div class="text-right font-mono text-xs text-fg-2 @max-2xl:hidden">{shownSize(e)}</div>
	{@render actions(e)}
{/snippet}

{#snippet row(e: Entry)}
	<div
		class="group grid h-11 items-center gap-3 rounded-md border-t border-line px-2 [contain-intrinsic-size:auto_2.75rem] [content-visibility:auto] hover:bg-surface-2 data-selected:bg-accent-soft {cols}"
		{...entryProps(e)}
	>
		{@render check(e)}
		<div class="flex min-w-0 items-center gap-2.5">
			<FileIcon name={e.name} dir={e.dir} size={18} />
			{@render title(e)}
		</div>
		{@render meta(e)}
	</div>
{/snippet}

<header class="page-header">
	<div class="flex gap-0.5">
		<button
			class="btn btn-ghost btn-icon"
			onclick={() => history.back()}
			aria-label="Back"
			title="Back ({mac ? '⌘[' : 'Alt+←'})"
		>
			<Icon name="arrowLeft" />
		</button>
		<button
			class="btn btn-ghost btn-icon"
			onclick={() => history.forward()}
			aria-label="Forward"
			title="Forward ({mac ? '⌘]' : 'Alt+→'})"
		>
			<Icon name="arrowRight" />
		</button>
	</div>
	<nav class="flex min-w-0 flex-1 items-center gap-1.5 font-medium whitespace-nowrap">
		<a
			href={filesHref('/')}
			class="shrink-0 {crumbs.length ? 'text-fg-2 hover:text-fg' : ''}"
			data-drop="/">Files</a
		>
		<!-- The current folder stays whole; deep paths fold their middle into "…" like Finder's path bar. -->
		{#if trail.length > 3}
			<span class="text-fg-3">/</span>
			<div class="relative shrink-0">
				<button
					class="rounded px-1 text-fg-2 hover:bg-surface-2 hover:text-fg"
					onclick={togglePop('crumbs')}
					aria-label="Enclosing folders"
					title="Enclosing folders">…</button
				>
				{#if pop === 'crumbs'}
					<div class="menu pop absolute top-7 left-0 z-20" role="menu" out:vanish>
						{#each trail.slice(0, -2) as c (c.path)}
							<a class="menu-item" href={filesHref(c.path)} data-drop={c.path}>
								<Icon name="folder" size={15} class="text-fg-2" /><span class="truncate"
									>{c.name}</span
								>
							</a>
						{/each}
					</div>
				{/if}
			</div>
		{/if}
		{#each trail.length > 3 ? trail.slice(-2) : trail as c (c.path)}
			<span class="text-fg-3">/</span>
			<a
				href={filesHref(c.path)}
				data-drop={c.path}
				title={c.name}
				class="truncate {c.path === dir ? 'min-w-12' : 'max-w-48 text-fg-2 hover:text-fg'}"
				>{c.name}</a
			>
		{/each}
	</nav>
	<label
		class="flex h-8 w-64 min-w-0 shrink items-center gap-2 rounded-md border border-line bg-surface-2 px-2.5 text-fg-3 focus-within:border-accent"
	>
		<Icon name="search" size={15} />
		<input
			class="w-full min-w-0 bg-transparent text-fg outline-none"
			placeholder="Search"
			bind:value={query}
			bind:this={searchInput}
			onkeydown={(e) => {
				if (e.key === 'Enter') scope = 'all';
				if (e.key === 'Escape') {
					query = '';
					scope = 'folder';
				}
			}}
		/>
		{#if !query}<kbd>/</kbd>{/if}
	</label>
</header>

<div class="@container flex h-11 shrink-0 items-center gap-1.5 border-b border-line px-5">
	{#if picked.length}
		<SelectionBar count={picked.length} onclear={() => sel.clear()}>
			{@const star = picked.every((e) => isStarred(server.id, e.path))}
			{@render act('download', 'Download', () => downloadAll(picked), '@max-[26rem]:hidden')}
			{@render act('folderMove', 'Move', () => open('move', picked), '@max-[26rem]:hidden')}
			{@render act('trash', 'Delete', () => trashAll(picked))}
			<div class="relative shrink-0">
				<button
					class="btn btn-ghost btn-icon"
					onclick={togglePop('more')}
					aria-label="More actions"
					title="More actions"><Icon name="dots" size={15} /></button
				>
				{#if pop === 'more'}
					<div class="menu pop absolute top-9 left-0 z-10" role="menu" out:vanish>
						<div class="@min-[26rem]:hidden">
							{@render item('download', 'Download', () => downloadAll(picked))}
							{@render item('folderMove', 'Move to…', () => open('move', picked))}
						</div>
						{#if single}{@render item('link', 'Copy link', () => copyLink(single))}{/if}
						{@render item(star ? 'starFill' : 'star', star ? 'Unstar' : 'Star', () =>
							toggleStar(server.id, picked)
						)}
						{@render item('copy', 'Copy to…', () => open('copy', picked))}
						{#if single}{@render item(
								'pencil',
								'Rename',
								() => open('rename', [single]),
								'F2'
							)}{/if}
					</div>
				{/if}
			</div>
		</SelectionBar>
	{:else}
		<div class="relative">
			<button class="btn btn-primary" onclick={togglePop('upload')} aria-label="Upload">
				<Icon name="upload" size={15} /><span class="@max-[34rem]:hidden">Upload</span><Icon
					name="chevronDown"
					size={13}
				/>
			</button>
			{#if pop === 'upload'}
				<div class="menu pop absolute top-9 left-0 z-10 w-48" role="menu" out:vanish>
					{@render item('upload', 'Files…', () => upload(PickUploads(dir, false)), `${mod}U`)}
					{@render item('folder', 'Folder…', () => upload(PickUploads(dir, true)))}
				</div>
			{/if}
		</div>
		<button
			class="btn @max-[34rem]:w-8 @max-[34rem]:px-0"
			onclick={() => open('mkdir', [])}
			aria-label="New folder"
			title="New folder ({mac ? '⇧⌘N' : 'Ctrl+Shift+N'})"
		>
			<Icon name="folderPlus" size={15} /><span class="@max-[34rem]:hidden">New folder</span>
		</button>
	{/if}
	<span class="flex-1"></span>
	<!-- While acting on a selection, a narrow toolbar keeps room for the actions. -->
	<div class="relative {picked.length ? '@max-[34rem]:hidden' : ''}">
		<button
			class="btn @max-[46rem]:w-8 @max-[46rem]:px-0"
			onclick={togglePop('sort')}
			aria-label={sortLabel}
			title="Sort by {sortLabel}"
		>
			<Icon name="transfers" size={15} class="text-fg-2" /><span class="@max-[46rem]:hidden"
				>{sortLabel}</span
			>
			<Icon name="chevronDown" size={13} class="text-fg-3 @max-[46rem]:hidden" />
		</button>
		{#if pop === 'sort'}
			<div class="menu pop absolute top-9 right-0 z-10 w-52" role="menu" out:vanish>
				<div
					class="px-2.5 pt-1.5 pb-1 text-[11px] font-medium tracking-[0.06em] text-fg-3 uppercase"
				>
					Sort by
				</div>
				{#each SORTS as [k, l] (k)}
					{@render opt(l, prefs.sort === k, () => setPrefs({ sort: k }))}
				{/each}
				<hr class="my-1 border-line" />
				{@render opt('Ascending', prefs.asc, () => setPrefs({ asc: true }))}
				{@render opt('Descending', !prefs.asc, () => setPrefs({ asc: false }))}
				<hr class="my-1 border-line" />
				<button
					class="menu-item"
					onclick={(ev) => {
						ev.stopPropagation();
						setPrefs({ hidden: !prefs.hidden });
					}}
				>
					<span
						class="flex h-4 w-4 items-center justify-center rounded border {prefs.hidden
							? 'border-primary bg-primary text-on-primary'
							: 'border-line-2'}"
					>
						{#if prefs.hidden}<Icon name="check" size={12} />{/if}
					</span>
					Show hidden files
				</button>
			</div>
		{/if}
	</div>
	<div
		class="inline-flex gap-0.5 rounded-md border border-line p-0.5 {picked.length
			? '@max-[34rem]:hidden'
			: ''}"
	>
		{#each [['list', 'list', 'List view'], ['card', 'grid', 'Card view']] as const as [v, icon, label] (v)}
			<button
				class="flex h-6.5 w-7 items-center justify-center rounded text-fg-3 hover:text-fg aria-pressed:bg-surface-2 aria-pressed:text-fg"
				aria-pressed={prefs.view === v}
				onclick={() => setPrefs({ view: v })}
				aria-label={label}
				title={label}><Icon name={icon} size={15} /></button
			>
		{/each}
	</div>
	<button
		class="btn btn-ghost btn-icon aria-pressed:bg-surface-2 aria-pressed:text-fg"
		aria-pressed={!prefs.infoHidden}
		onclick={toggleInfo}
		aria-label="Info"
		title="Info ({mod}I)"
	>
		<Icon name="info" size={15} />
	</button>
</div>

<Split
	end
	class="min-h-0 flex-1"
	bind:hidden={() => prefs.infoHidden, (v) => setPrefs({ infoHidden: v })}
	bind:width={() => prefs.infoWidth, (v) => setPrefs({ infoWidth: v })}
	min={240}
	max={420}
>
	{#snippet pane()}
		<InfoPanel
			items={picked}
			{rows}
			{dir}
			{usage}
			size={entrySize}
			onopen={openEntry}
			oncopylink={copyLink}
		/>
	{/snippet}
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<div
		class="relative flex h-full min-w-0 flex-col"
		data-file-drop-target={draggingOut ? undefined : true}
		data-drop-label="Drop to upload to {dir}"
		data-drop={dir}
		oncontextmenu={showAreaMenu}
	>
		{#if loading}<div class="loading-bar"></div>{/if}
		<div
			class="@container flex min-h-0 flex-1 flex-col gap-7 overflow-y-auto p-6"
			bind:this={scroller}
			onscroll={(ev) => (scrolls[dir] = ev.currentTarget.scrollTop)}
			{@attach marquee}
		>
			{#if q || scope === 'all'}
				<div class="flex items-center gap-3.5">
					<div class="seg">
						<button aria-pressed={scope === 'folder'} onclick={() => (scope = 'folder')}
							>This folder</button
						>
						<button aria-pressed={scope === 'all'} onclick={() => (scope = 'all')}
							>Everywhere</button
						>
					</div>
					{#if scope === 'all'}
						<select class="btn h-7 text-xs" bind:value={type} aria-label="Type">
							{#each TYPES as [v, l] (v)}<option value={v}>{l}</option>{/each}
						</select>
						<select class="btn h-7 text-xs" bind:value={within} aria-label="Modified">
							{#each WITHIN as [v, l] (v)}<option value={v}>{l}</option>{/each}
						</select>
					{/if}
					{#if searching}
						<span class="text-fg-2">
							{plural(total, 'result')}{q ? ` for “${q}”` : ''}{total > hits.length
								? ` · showing the best ${hits.length}`
								: ''}
						</span>
						<span class="flex-1"></span>
						{#if idx && !idx.done}
							<span class="text-xs text-fg-2">
								Building search index{idx.folders
									? ` · ${idx.folders} folders scanned, ${idx.pending} pending`
									: '…'}
							</span>
						{:else if idx?.at}
							<span class="text-xs text-fg-3"
								>Indexed {idx.count.toLocaleString()} items · {ago(idx.at)}</span
							>
							<button
								class="flex items-center gap-1 text-xs text-accent-fg hover:underline"
								onclick={() => Reindex()}><Icon name="refresh" size={13} />Refresh</button
							>
						{/if}
					{:else if scope === 'folder'}
						<span class="text-xs text-fg-3">Press Enter to search everywhere</span>
					{/if}
				</div>
			{/if}
			{#if error}
				<div class="flex flex-1 flex-col items-center justify-center gap-3 text-center">
					<p class="text-danger">{error}</p>
					{#if dir !== '/'}
						<a class="btn" href={filesHref(parent(dir))}
							><Icon name="arrowLeft" size={15} />Go to the enclosing folder</a
						>
					{/if}
				</div>
			{:else if searching}
				<div>
					{#each hits as e (e.path)}
						{@const i = q ? e.name.toLowerCase().indexOf(q.toLowerCase()) : -1}
						<div
							class="group grid h-13 items-center gap-3 rounded-md border-t border-line px-2 [contain-intrinsic-size:auto_3.25rem] [content-visibility:auto] hover:bg-surface-2 data-selected:bg-accent-soft {cols}"
							{...entryProps(e)}
							onclick={() => dragged || openEntry(e)}
						>
							{@render check(e)}
							<div class="flex min-w-0 items-center gap-2.5">
								<FileIcon name={e.name} dir={e.dir} size={18} />
								<div class="flex min-w-0 flex-col">
									<span class="truncate">
										{#if i >= 0}{e.name.slice(0, i)}<mark
												class="rounded-[3px] bg-accent-soft text-inherit"
												>{e.name.slice(i, i + q.length)}</mark
											>{e.name.slice(i + q.length)}{:else}{e.name}{/if}
									</span>
									<span class="truncate font-mono text-[11px] text-fg-3">{parent(e.path)}</span>
								</div>
							</div>
							{@render meta(e)}
						</div>
					{:else}
						<p class="py-6 text-center text-fg-3">
							{idx && !idx.done ? 'Searching as the index builds…' : 'No matches.'}
						</p>
					{/each}
				</div>
			{:else if loading && !entries.length}
				<div class="skeleton flex flex-col gap-7">
					{#if prefs.view === 'card'}
						<div class="grid grid-cols-[repeat(auto-fill,minmax(180px,1fr))] gap-3">
							{#each { length: 10 }}
								<div class="flex flex-col gap-3 overflow-hidden rounded-lg border border-line">
									<div class="h-24 bg-surface-2"></div>
									<div class="flex flex-col gap-2 px-3 pb-3">
										<div class="h-3 w-3/5 rounded bg-surface-2"></div>
										<div class="h-2.5 w-2/5 rounded bg-surface-2"></div>
									</div>
								</div>
							{/each}
						</div>
					{:else}
						{#each { length: 8 }}
							<div class="flex h-11 items-center gap-3 border-t border-line px-2">
								<div class="h-4.5 w-4.5 rounded bg-surface-2"></div>
								<div class="h-3 max-w-60 flex-1 rounded bg-surface-2"></div>
							</div>
						{/each}
					{/if}
				</div>
			{:else if !loading && !entries.length}
				<div
					class="flex flex-1 flex-col items-center justify-center gap-4 rounded-[10px] border-[1.5px] border-dashed border-line-2 text-center"
				>
					<div
						class="flex h-14 w-14 items-center justify-center rounded-full bg-surface-2 text-fg-2"
					>
						<Icon name="upload" size={24} />
					</div>
					<div>
						<p class="text-[15px] font-semibold tracking-tight">Nothing here yet</p>
						<p class="mt-1.5 max-w-90 text-fg-2">
							{drag ? 'Release to move here.' : `Drop files here to upload them to ${dir}.`}
						</p>
					</div>
					<div class="flex gap-2">
						<button class="btn btn-primary" onclick={() => upload(PickUploads(dir, false))}>
							<Icon name="upload" size={15} />Upload
						</button>
						<button class="btn" onclick={() => open('mkdir', [])}>
							<Icon name="folderPlus" size={15} />New folder
						</button>
					</div>
				</div>
			{:else if prefs.view === 'card'}
				{#if folders.length}
					<section class="flex flex-col gap-3">
						<h2 class="font-medium">
							Folders <span class="font-normal text-fg-3">{folders.length}</span>
						</h2>
						<div class="grid grid-cols-[repeat(auto-fill,minmax(180px,1fr))] gap-3">
							{#each folders as e (e.path)}
								<div
									class="group relative flex flex-col gap-3.5 rounded-lg border border-line bg-surface p-3.5 [contain-intrinsic-size:auto_7.25rem] [content-visibility:auto] hover:bg-surface-2 data-selected:border-accent data-selected:bg-accent-soft"
									{...entryProps(e)}
								>
									<div class="absolute top-2 right-2 flex items-center gap-1">
										{@render check(e)}
										<span
											class="opacity-0 group-focus-within:opacity-100 group-hover:opacity-100 motion-safe:transition-opacity"
										>
											{@render tool('dots', 'Actions', (ev) => showMenu(ev, e))}
										</span>
									</div>
									<Icon name="folderFill" size={36} />
									<span class="flex flex-col gap-0.5">
										<span class="flex min-w-0 items-center gap-1 font-medium"
											>{@render title(e)}</span
										>
										<span class="text-xs text-fg-3">{when(e.modified)}</span>
									</span>
								</div>
							{/each}
						</div>
					</section>
				{/if}
				{#if files.length}
					<section class="flex flex-col gap-3">
						<h2 class="font-medium">
							Files <span class="font-normal text-fg-3">{files.length}</span>
						</h2>
						<div class="grid grid-cols-[repeat(auto-fill,minmax(180px,1fr))] gap-3">
							{#each files as e (e.path)}
								<div
									class="group flex flex-col overflow-hidden rounded-lg border border-line bg-surface [contain-intrinsic-size:auto_9.5rem] [content-visibility:auto] hover:bg-surface-2 data-selected:border-accent data-selected:bg-accent-soft"
									{...entryProps(e)}
								>
									<div
										class="relative flex h-24 items-center justify-center bg-surface-2 text-fg-3"
									>
										<div class="absolute top-2 left-2 z-10">{@render check(e)}</div>
										<div
											class="absolute top-1 right-1 z-10 opacity-0 group-focus-within:opacity-100 group-hover:opacity-100"
										>
											{@render tool('dots', 'Actions', (ev) => showMenu(ev, e))}
										</div>
										<FileIcon name={e.name} size={48} />
										{#if previewKind(e.name) === 'image'}
											<img
												src={thumbUrl(e.path, e.modified)}
												alt=""
												loading="lazy"
												class="absolute inset-0 h-full w-full bg-surface-2 object-scale-down"
												onerror={(ev) => ev.currentTarget.remove()}
											/>
										{/if}
									</div>
									<div class="flex flex-col gap-0.5 px-3 py-2.5">
										<span class="flex min-w-0 items-center gap-1 font-medium"
											>{@render title(e)}</span
										>
										<span class="truncate text-xs text-fg-3" title={when(e.modified)}
											>{bytes(e.size)} · {when(e.modified)}</span
										>
									</div>
								</div>
							{/each}
						</div>
					</section>
				{/if}
			{:else}
				<section class="flex flex-col gap-1">
					<div class="grid h-8.5 items-center gap-3 px-2 text-xs font-medium text-fg-3 {cols}">
						<input
							type="checkbox"
							class="check"
							checked={!!rows.length && picked.length === rows.length}
							indeterminate={!!picked.length && picked.length < rows.length}
							onclick={() =>
								picked.length === rows.length ? sel.clear() : select(rows.map((e) => e.path))}
							aria-label="Select all"
						/>
						{@render head('name', 'Name')}
						{@render head('modified', 'Modified', 'flex @max-xl:hidden')}
						{@render head('size', 'Size', 'flex justify-end @max-2xl:hidden')}
						<div></div>
					</div>
					<div>
						{#each rows as e (e.path)}{@render row(e)}{/each}
					</div>
				</section>
			{/if}
		</div>
	</div>
</Split>

{#if drag}
	<div
		class="pointer-events-none fixed z-50 rounded-md bg-primary px-2.5 py-1 text-xs font-medium text-on-primary shadow-lg"
		style="left: {drag.x + 14}px; top: {drag.y + 10}px"
	>
		{drag.items.length === 1 ? drag.items[0].name : plural(drag.items.length, 'item')}
	</div>
{/if}

{#if preview}
	<Preview
		e={preview}
		items={viewable}
		onchange={(e) => {
			preview = e;
			select([e.path]);
		}}
		onclose={() => (preview = null)}
		onshow={searching ? (e) => goto(focusHref(e.path)) : undefined}
	/>
{/if}

{#if menu}
	{@const m = menu}
	{@const one = m.items.length === 1 ? m.items[0] : null}
	<div
		popover="manual"
		class="menu pop fixed inset-auto m-0 text-fg"
		{@attach at(m.x, m.y)}
		role="menu"
		out:vanish
	>
		{#if !m.items.length}
			{@render item(
				'folderPlus',
				'New folder',
				() => open('mkdir', []),
				mac ? '⇧⌘N' : 'Ctrl+Shift+N'
			)}
			{@render item('upload', 'Upload files…', () => upload(PickUploads(dir, false)), `${mod}U`)}
			{@render item('folder', 'Upload folder…', () => upload(PickUploads(dir, true)))}
			{@render item(
				'copy',
				`Paste${clip ? ` ${clip.items.length} ${clip.items.length === 1 ? 'item' : 'items'}` : ''}`,
				paste,
				`${mod}V`,
				!clip
			)}
			<hr class="my-1 border-line" />
			{@render item('check', 'Select all', () => select(shown.map((e) => e.path)), `${mod}A`)}
			{@render item('refresh', 'Refresh', refresh, `${mod}R`)}
			{@render item('info', 'Keyboard shortcuts', () => (help = true), '?')}
		{:else}
			{@const n = one ? '' : ` ${m.items.length} items`}
			{@const star = m.items.every((e) => isStarred(server.id, e.path))}
			{#if one?.dir}
				<a class="menu-item" href={filesHref(one.path)}
					><Icon name="open" size={15} class="text-fg-2" />Open</a
				>
			{:else if one}
				{@render item('eye', 'Preview', () => (preview = one))}
				{@render item('open', 'Open in default app', () => openFile(one))}
			{/if}
			{@render item('download', `Download${n}`, () => downloadAll(m.items))}
			{#if one}
				{@render item('pencil', 'Rename', () => open('rename', [one]), 'F2')}
				{@render item('link', 'Copy link', () => copyLink(one))}
			{/if}
			{@render item(star ? 'starFill' : 'star', star ? 'Unstar' : 'Star', () =>
				toggleStar(server.id, m.items)
			)}
			<hr class="my-1 border-line" />
			{@render item('copy', 'Copy', () => (clip = { items: m.items, cut: false }), `${mod}C`)}
			{@render item('cut', 'Cut', () => (clip = { items: m.items, cut: true }), `${mod}X`)}
			{#if one}{@render item('copy', 'Duplicate', () => duplicate(one))}{/if}
			{@render item('folderMove', `Copy${n} to…`, () => open('copy', m.items))}
			{@render item('folderMove', `Move${n} to…`, () => open('move', m.items))}
			<hr class="my-1 border-line" />
			<button class="menu-item text-danger" onclick={() => trashAll(m.items)}>
				<Icon name="trash" size={15} />Delete{n}<kbd class="ml-auto">{mac ? '⌘⌫' : 'Del'}</kbd>
			</button>
		{/if}
	</div>
{/if}

<Dialog
	open={dialog === 'mkdir' || dialog === 'rename'}
	title={dialog === 'mkdir' ? 'New folder' : 'Rename'}
	onclose={() => (dialog = '')}
>
	<form class="flex flex-col gap-4" onsubmit={submit}>
		<label class="field">
			<span class="label">Name</span>
			<input
				class="input {nameError && name
					? 'border-danger focus:border-danger focus:ring-danger-soft'
					: ''}"
				bind:value={name}
				bind:this={nameInput}
				required
			/>
			{#if nameError && name}
				<span class="text-xs text-danger">{nameError}</span>
			{:else if dialog === 'mkdir'}
				<span class="hint">Created in {dir}</span>
			{/if}
		</label>
		<div class="flex justify-end gap-2">
			<button type="button" class="btn" onclick={() => (dialog = '')}>Cancel</button>
			<button class="btn btn-primary" disabled={!!nameError}>
				{dialog === 'mkdir' ? 'Create' : 'Rename'}
			</button>
		</div>
	</form>
</Dialog>

{#if (dialog === 'move' || dialog === 'copy') && targets.length}
	<MoveDialog
		items={targets}
		mode={dialog}
		onclose={() => (dialog = '')}
		onmove={dialog === 'copy' ? copyAll : moveAll}
	/>
{/if}

{#if conflicts[0]}
	{#key conflicts[0].local}
		<ConflictDialog c={conflicts[0]} remaining={conflicts.length - 1} onresolve={resolve} />
	{/key}
{/if}

<Dialog open={help} title="Keyboard shortcuts" onclose={() => (help = false)}>
	<dl class="grid grid-cols-[auto_1fr] items-center gap-x-4 gap-y-2 text-xs">
		{#each SHORTCUTS as [k, label] (label)}
			<dt><kbd>{k}</kbd></dt>
			<dd class="text-fg-2">{label}</dd>
		{/each}
	</dl>
</Dialog>
