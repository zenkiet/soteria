<script module lang="ts">
	let restored = false;
</script>

<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { Clipboard, Events } from '@wailsio/runtime';
	import { tick, untrack } from 'svelte';
	import { SvelteSet } from 'svelte/reactivity';
	import {
		Copy,
		Download,
		DownloadDir,
		DragOut,
		FolderUsages,
		Open,
		List,
		Mkdir,
		Move,
		PickFolder,
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
		filesHref,
		iconFor,
		join,
		kind,
		mac,
		msg,
		net,
		parent,
		prefs,
		previewKind,
		previewUrl,
		setPrefs,
		thumbUrl,
		toast,
		validName,
		when,
		type SortKey
	} from '#/shared/lib/index.ts';
	import { Dialog, Icon, type IconName } from '#/shared/ui/index.ts';
	import ConflictDialog, { type Resolution } from './ui/conflict-dialog.svelte';
	import MoveDialog from './ui/move-dialog.svelte';
	import Preview from './ui/preview.svelte';
	import SelectionPanel from './ui/selection-panel.svelte';

	let { path = '' }: { path?: string } = $props();
	const dir = $derived('/' + path.replace(/\/+$/, ''));
	const crumbs = $derived(dir === '/' ? [] : dir.slice(1).split('/'));
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
	let sortOpen = $state(false);
	let dialog = $state<'' | 'mkdir' | 'rename' | 'move' | 'copy'>('');
	let targets = $state<Entry[]>([]);
	let name = $state('');
	let scope = $state<'folder' | 'all'>('folder');
	let results = $state.raw<Entry[]>([]);
	const idx = $derived(indexStatus.current);
	let conflicts = $state<Conflict[]>([]);
	let clip = $state<{ items: Entry[]; cut: boolean } | null>(null);
	let loadedAt = 0;

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

	const collator = new Intl.Collator(undefined, { numeric: true, sensitivity: 'base' });
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

	const shownName = (e: Entry) => prefs.hidden || !e.name.startsWith('.');
	const visible = $derived(
		entries
			.filter((e) => shownName(e) && e.name.toLowerCase().includes(query.toLowerCase()))
			.sort(compare)
	);
	const folders = $derived(visible.filter((e) => e.dir));
	const files = $derived(visible.filter((e) => !e.dir));
	const rows = $derived([...folders, ...files]);
	const searching = $derived(scope === 'all' && query.trim() !== '');
	const hits = $derived(results.filter(shownName).sort(compare));
	const shown = $derived(searching ? hits : rows);
	const viewable = $derived(shown.filter((e) => !e.dir && previewKind(e.name)));
	const picked = $derived(shown.filter((e) => sel.has(e.path)));
	const single = $derived(picked.length === 1 ? picked[0] : null);

	async function loadUsage(d: string) {
		const u = (await FolderUsages(d)) ?? {};
		if (dir === d) usage = u;
	}

	// A newer load cancels the one in flight; the old folder stays on screen until the new one crossfades in.
	let listing: ReturnType<typeof List> | null = null;
	let shownDir = '';
	async function load(d: string) {
		listing?.cancel();
		const call = (listing = List(d));
		loading = true;
		try {
			const list = (await call) ?? [];
			if (call !== listing) return;
			const swap = () => {
				entries = list;
				return tick();
			};
			if (d !== shownDir && 'startViewTransition' in document)
				await document.startViewTransition(swap).updateCallbackDone;
			else await swap();
			shownDir = d;
			loadUsage(d);
			error = '';
			const f = page.url.searchParams.get('focus');
			if (f) {
				select([f]);
				anchor = f;
				await tick();
				document
					.querySelector(`[data-path="${CSS.escape(f)}"]`)
					?.scrollIntoView({ block: 'center' });
			}
		} catch (e) {
			if (call !== listing) return;
			error = msg(e);
		}
		loading = false;
		loadedAt = Date.now();
	}

	// Spin at least one turn so a fast reload still shows feedback.
	let spinning = $state(false);
	async function refresh() {
		spinning = true;
		await Promise.all([load(dir), new Promise((r) => setTimeout(r, 500))]);
		spinning = false;
	}

	const nameError = $derived(
		dialog === 'mkdir' || dialog === 'rename'
			? validName(
					name,
					entries.filter((e) => e.path !== targets[0]?.path).map((e) => e.name)
				)
			: ''
	);

	$effect(() => {
		sel.clear();
		preview = null;
		query = '';
		scope = 'folder';
		const last = untrack(() => prefs.lastDir[server.id]);
		if (!restored) {
			restored = true;
			if (!path && last && last !== '/') {
				goto(filesHref(last), { replace: true });
				return;
			}
		}
		untrack(() => setPrefs({ lastDir: { ...prefs.lastDir, [server.id]: dir } }));
		load(dir);
	});

	$effect(() => {
		if (searching) Search(query).then((r) => (results = r ?? []));
	});

	$effect(() => {
		if (net.back) load(dir);
	});

	$effect(() => {
		if (idx?.done)
			untrack(() => {
				loadUsage(dir);
				if (searching) Search(query).then((r) => (results = r ?? []));
			});
	});

	$effect(() =>
		Events.On('dropped', (ev) =>
			Upload(ev.data.files, ev.data.dir || dir)
				.then(queueConflicts)
				.catch((e) => toast(msg(e), 'error'))
		)
	);

	let reload: ReturnType<typeof setTimeout>;
	$effect(() =>
		Events.On('transfer', (ev) => {
			const t = ev.data;
			if (t.id === opening && t.status === 'done') {
				opening = '';
				Open(t.local).catch((e) => toast(msg(e), 'error'));
			}
			if (t.kind !== 'upload' || t.status !== 'done' || parent(t.remote) !== dir) return;
			clearTimeout(reload);
			reload = setTimeout(() => load(dir), 500);
		})
	);

	function queueConflicts(cs: Conflict[] | null) {
		if (cs?.length) conflicts = [...conflicts, ...cs];
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
			toast(done, 'ok', undo && { label: 'Undo', run: () => run(undo, 'Undone') });
			await load(dir);
		} catch (e) {
			toast(msg(e), 'error');
		}
		dialog = '';
	}

	function submit(e: SubmitEvent) {
		e.preventDefault();
		if (nameError) return;
		const n = name.trim();
		if (dialog === 'mkdir') run(() => Mkdir(join(dir, n)), `Created ${n}`);
		else {
			const t = targets[0];
			const to = join(parent(t.path), n);
			run(
				() => Move(t.path, to),
				`Renamed to ${n}`,
				() => Move(to, t.path)
			);
		}
	}

	// All calls run at once; the first failure is thrown only after every call has settled.
	async function each<T>(items: T[], f: (x: T) => Promise<unknown>) {
		const failed = (await Promise.allSettled(items.map(f))).find((r) => r.status === 'rejected');
		if (failed) throw failed.reason;
	}

	async function trashAll(items: Entry[]) {
		sel.clear();
		menu = null;
		const moved: string[] = [];
		try {
			await each(items, async (e) => moved.push(await Trash(e.path)));
		} catch (e) {
			toast(msg(e), 'error');
		}
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

	function moveAll(dest: string) {
		const items = targets;
		run(
			() => each(items, (e) => Move(e.path, join(dest, e.name))),
			`Moved to ${dest}`,
			() => each(items, (e) => Move(join(dest, e.name), e.path))
		);
	}

	function copyAll(dest: string) {
		const items = targets;
		run(() => each(items, (e) => Copy(e.path, join(dest, e.name))), `Copied to ${dest}`);
	}

	// dup picks "name copy.ext", then "name copy 2.ext"… until the name is free in this folder.
	function dup(e: Entry) {
		const i = e.dir ? -1 : e.name.lastIndexOf('.');
		const stem = i > 0 ? e.name.slice(0, i) : e.name;
		const ext = i > 0 ? e.name.slice(i) : '';
		let n = `${stem} copy${ext}`;
		for (let k = 2; entries.some((x) => x.name === n); k++) n = `${stem} copy ${k}${ext}`;
		return n;
	}

	function duplicate(e: Entry) {
		menu = null;
		run(() => Copy(e.path, join(dir, dup(e))), `Duplicated ${e.name}`);
	}

	function paste() {
		const c = clip;
		if (!c) return;
		if (c.cut) clip = null;
		const items = c.items.filter((e) => !c.cut || parent(e.path) !== dir);
		if (!items.length) return;
		run(
			() =>
				each(items, (e) =>
					c.cut
						? Move(e.path, join(dir, e.name))
						: Copy(e.path, join(dir, entries.some((x) => x.name === e.name) ? dup(e) : e.name))
				),
			`${c.cut ? 'Moved' : 'Pasted'} ${items.length} ${items.length === 1 ? 'item' : 'items'}`
		);
	}

	function open(kind: typeof dialog, items: Entry[]) {
		targets = items;
		name = kind === 'rename' ? items[0].name : '';
		dialog = kind;
		menu = null;
	}

	async function downloadAll(items: Entry[]) {
		menu = null;
		try {
			const dest = prefs.askDownloadDir ? await PickFolder() : prefs.downloadDir;
			if (prefs.askDownloadDir && !dest) return;
			for (const e of items)
				await (e.dir ? DownloadDir(e.path, dest) : Download(e.path, e.size, dest));
			toast(
				items.length === 1 ? `Downloading ${items[0].name}` : `Downloading ${items.length} items`
			);
		} catch (e) {
			toast(msg(e), 'error');
		}
	}

	async function copyLink(e: Entry) {
		// soteria://files/<path> mirrors the SPA route; a file links to its folder, focused.
		const route = e.dir
			? filesHref(e.path)
			: filesHref(parent(e.path)) + '?focus=' + encodeURIComponent(e.path);
		await Clipboard.SetText('soteria:/' + route);
		toast('Link copied');
		menu = null;
	}

	let cursor = '';
	function pick(e: Entry, ev: MouseEvent | KeyboardEvent) {
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
		if (!(ev.metaKey || ev.ctrlKey)) select([e.path]);
		else if (!sel.delete(e.path)) sel.add(e.path);
		anchor = e.path;
	}

	let opening = '';
	function openEntry(e: Entry) {
		if (e.dir) goto(filesHref(e.path));
		else if (previewKind(e.name)) preview = e;
		else
			Download(e.path, e.size, prefs.downloadDir).then(
				(id) => ((opening = id), toast(`Downloading ${e.name}`)),
				(err) => toast(msg(err), 'error')
			);
	}

	function openResult(e: Entry) {
		if (e.dir) goto(filesHref(e.path));
		else if (parent(e.path) === dir) {
			query = '';
			select([e.path]);
		} else goto(filesHref(parent(e.path)) + '?focus=' + encodeURIComponent(e.path));
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
		'data-file-drop-target': e.dir && !draggingOut ? true : undefined,
		draggable: !e.dir,
		onclick: (ev: MouseEvent) => pick(e, ev),
		onkeydown: (ev: KeyboardEvent) => ev.key === 'Enter' && openEntry(e),
		ondblclick: () => openEntry(e),
		ondragstart: (ev: DragEvent) => dragStart(ev, e),
		ondragend: () => (draggingOut = false),
		oncontextmenu: (ev: MouseEvent) => showMenu(ev, e)
	});

	function showMenu(ev: MouseEvent, e: Entry) {
		ev.preventDefault();
		ev.stopPropagation();
		if (!sel.has(e.path)) {
			select([e.path]);
			anchor = e.path;
		}
		const items = sel.size > 1 ? shown.filter((x) => sel.has(x.path)) : [e];
		menu = { x: ev.clientX, y: ev.clientY, items };
	}

	function keys(ev: KeyboardEvent) {
		if (ev.key === 'Escape') {
			if (!menu && !sortOpen && !preview && !dialog) sel.clear();
			menu = null;
			sortOpen = false;
			return;
		}
		if (ev.target instanceof HTMLInputElement || preview || dialog) return;
		const mod = ev.metaKey || ev.ctrlKey;
		const step = { ArrowDown: 1, ArrowUp: -1 }[ev.key] ?? 0;
		const k = ev.key.toLowerCase();
		if (step && mod) {
			if (step < 0 && dir !== '/') goto(filesHref(parent(dir)));
		} else if (step) {
			const i = shown.findIndex((e) => e.path === cursor);
			const e = shown[Math.max(0, Math.min(shown.length - 1, i + step))];
			if (!e) return;
			pick(e, ev);
			document.querySelector<HTMLElement>(`[data-path="${CSS.escape(e.path)}"]`)?.focus();
		} else if (ev.key === ' ' && !(ev.target instanceof HTMLButtonElement)) {
			if (single && !single.dir) preview = single;
			else return;
		} else if (!mod) return;
		else if (k === 'a') select(shown.map((e) => e.path));
		else if ((k === 'c' || k === 'x') && picked.length) clip = { items: picked, cut: k === 'x' };
		else if (k === 'v') paste();
		else if (k === 'r') refresh();
		else if (k === 'u') PickUploads(dir).then(queueConflicts);
		else if (k === 'n' && ev.shiftKey) open('mkdir', []);
		else return;
		ev.preventDefault();
	}

	function clicked(ev: MouseEvent) {
		menu = null;
		sortOpen = false;
		if (!(ev.target as Element).closest('[role=button],button,a,input,label,aside,dialog'))
			sel.clear();
	}

	let draggingOut = $state(false);
	$effect(() => Events.On('dragend', () => (draggingOut = false)));

	function dragStart(ev: DragEvent, e: Entry) {
		const items = (sel.has(e.path) ? picked : [e]).filter((x) => !x.dir);
		if (!items.length) {
			ev.preventDefault();
			return;
		}
		draggingOut = true;
		if (mac) {
			ev.preventDefault();
			DragOut(items).catch((err) => {
				draggingOut = false;
				toast(msg(err), 'error');
			});
		} else {
			ev.dataTransfer?.setData(
				'DownloadURL',
				items
					.map((x) => `application/octet-stream:${x.name}:${location.origin}${previewUrl(x.path)}`)
					.join('\n')
			);
		}
	}

	const cols =
		'grid-cols-[minmax(0,1fr)_90px_170px_36px] @4xl:grid-cols-[minmax(0,1fr)_120px_90px_170px_170px_36px]';
	const on = (e: Entry) => sel.has(e.path);
	const mod = mac ? '⌘' : 'Ctrl+';
</script>

<svelte:window
	onclick={clicked}
	onkeydown={keys}
	onfocus={() => Date.now() - loadedAt > 5000 && load(dir)}
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

{#snippet item(icon: IconName, label: string, run: () => unknown, key = '', disabled = false)}
	<button class="menu-item" {disabled} onclick={run}>
		<Icon name={icon} size={15} class="text-fg-2" />{label}{#if key}<kbd class="ml-auto">{key}</kbd
			>{/if}
	</button>
{/snippet}

{#snippet dots(e: Entry)}
	<button
		class="btn btn-ghost h-7 w-7 px-0 text-fg-3"
		onclick={(ev) => showMenu(ev, e)}
		aria-label="Actions"><Icon name="dots" size={15} /></button
	>
{/snippet}

{#snippet row(e: Entry)}
	<div
		class="grid h-11 items-center gap-3 rounded-md border-t border-line px-2 [contain-intrinsic-size:auto_2.75rem] [content-visibility:auto] hover:bg-surface-2 {cols} {on(
			e
		)
			? 'bg-accent-soft hover:bg-accent-soft'
			: ''}"
		{...entryProps(e)}
	>
		<div class="flex min-w-0 items-center gap-2.5">
			<Icon name={iconFor(e)} size={18} class={e.dir ? '' : 'text-fg-2'} />
			<span class="truncate">{e.name}</span>
		</div>
		<div class="hidden truncate text-xs text-fg-3 @4xl:block">{kind(e)}</div>
		<div class="text-right font-mono text-xs text-fg-2">{shownSize(e)}</div>
		<div class="text-xs text-fg-2">{when(e.modified)}</div>
		<div class="hidden text-xs text-fg-2 @4xl:block">{when(e.created)}</div>
		{@render dots(e)}
	</div>
{/snippet}

<header class="page-header">
	<div class="flex gap-0.5">
		<button class="btn btn-ghost btn-icon" onclick={() => history.back()} aria-label="Back">
			<Icon name="arrowLeft" />
		</button>
		<button class="btn btn-ghost btn-icon" onclick={() => history.forward()} aria-label="Forward">
			<Icon name="arrowRight" />
		</button>
	</div>
	<nav class="flex min-w-0 flex-1 items-center gap-2 font-medium whitespace-nowrap">
		<a href={filesHref('')} class={crumbs.length ? 'text-fg-2 hover:text-fg' : ''}>Files</a>
		{#each crumbs as c, i (i)}
			<span class="text-fg-3">/</span>
			<a
				href={filesHref('/' + crumbs.slice(0, i + 1).join('/'))}
				class="truncate {i < crumbs.length - 1 ? 'text-fg-2 hover:text-fg' : ''}">{c}</a
			>
		{/each}
	</nav>
	{#if sel.size > 1}
		<button
			class="flex h-7 items-center gap-1.5 rounded-full bg-accent-soft px-2.5 text-xs font-medium text-accent-fg"
			onclick={() => sel.clear()}>{sel.size} selected<Icon name="x" size={12} /></button
		>
	{/if}
	<label
		class="flex h-8 w-44 shrink items-center gap-2 rounded-md border border-line bg-surface-2 px-2.5 text-fg-3 focus-within:border-accent"
	>
		<Icon name="search" size={15} />
		<input
			class="w-full bg-transparent text-fg outline-none"
			placeholder="Search"
			bind:value={query}
			onkeydown={(e) => e.key === 'Enter' && (scope = 'all')}
		/>
	</label>
	<div class="relative">
		<button
			class="btn"
			onclick={(ev) => {
				ev.stopPropagation();
				sortOpen = !sortOpen;
			}}
		>
			<Icon name="transfers" size={15} class="text-fg-2" />{sortLabel}
			<Icon name="chevronDown" size={13} class="text-fg-3" />
		</button>
		{#if sortOpen}
			<div class="menu pop absolute top-9 right-0 z-10 w-52" role="menu">
				<div
					class="px-2.5 pt-1.5 pb-1 text-[11px] font-medium tracking-[0.06em] text-fg-3 uppercase"
				>
					Sort by
				</div>
				{#each SORTS as [k, l] (k)}
					<button class="menu-item" onclick={() => setPrefs({ sort: k })}>
						<span class="flex-1">{l}</span>
						{#if prefs.sort === k}<Icon name="check" size={14} class="text-accent-fg" />{/if}
					</button>
				{/each}
				<hr class="my-1 border-line" />
				<button class="menu-item" onclick={() => setPrefs({ asc: true })}>
					<span class="flex-1">Ascending</span>
					{#if prefs.asc}<Icon name="check" size={14} class="text-accent-fg" />{/if}
				</button>
				<button class="menu-item" onclick={() => setPrefs({ asc: false })}>
					<span class="flex-1">Descending</span>
					{#if !prefs.asc}<Icon name="check" size={14} class="text-accent-fg" />{/if}
				</button>
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
	<div class="inline-flex gap-0.5 rounded-md border border-line p-0.5">
		<button
			class="flex h-6.5 w-7 items-center justify-center rounded {prefs.view === 'list'
				? 'bg-surface-2 text-fg'
				: 'text-fg-3 hover:text-fg'}"
			onclick={() => setPrefs({ view: 'list' })}
			aria-label="List view"><Icon name="list" size={15} /></button
		>
		<button
			class="flex h-6.5 w-7 items-center justify-center rounded {prefs.view === 'card'
				? 'bg-surface-2 text-fg'
				: 'text-fg-3 hover:text-fg'}"
			onclick={() => setPrefs({ view: 'card' })}
			aria-label="Card view"><Icon name="grid" size={15} /></button
		>
	</div>
	<button class="btn btn-ghost btn-icon" onclick={refresh} disabled={spinning} aria-label="Refresh">
		<Icon name="refresh" size={15} class={spinning ? 'animate-spin' : ''} />
	</button>
	<button class="btn" onclick={() => open('mkdir', [])}>
		<Icon name="folderPlus" size={15} />New folder
	</button>
	<button class="btn btn-primary" onclick={() => PickUploads(dir).then(queueConflicts)}>
		<Icon name="upload" size={15} />Upload
	</button>
</header>

<div class="flex min-h-0 flex-1">
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<div
		class="relative flex min-w-0 flex-1 flex-col"
		data-file-drop-target={draggingOut ? undefined : true}
		data-drop-label="Drop to upload to {dir}"
		oncontextmenu={showAreaMenu}
	>
		<div class="@container flex min-h-0 flex-1 flex-col gap-7 overflow-y-auto p-6">
			{#if query.trim()}
				<div class="flex items-center gap-3.5">
					<div class="inline-flex gap-0.5 rounded-md bg-surface-2 p-0.5">
						<button
							class="rounded px-3 py-1 text-xs font-medium {scope === 'folder'
								? 'bg-surface text-fg shadow-[0_1px_2px_rgba(0,0,0,0.08)]'
								: 'text-fg-2'}"
							onclick={() => (scope = 'folder')}>This folder</button
						>
						<button
							class="rounded px-3 py-1 text-xs font-medium {scope === 'all'
								? 'bg-surface text-fg shadow-[0_1px_2px_rgba(0,0,0,0.08)]'
								: 'text-fg-2'}"
							onclick={() => (scope = 'all')}>Everywhere</button
						>
					</div>
					{#if searching}
						<span class="text-fg-2">
							{hits.length}
							{hits.length === 1 ? 'result' : 'results'} for “{query.trim()}”
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
					{:else}
						<span class="text-xs text-fg-3">Press Enter to search everywhere</span>
					{/if}
				</div>
			{/if}
			{#if error}
				<p class="text-danger">{error}</p>
			{:else if searching}
				<div class="reveal">
					{#each hits as e (e.path)}
						{@const i = e.name.toLowerCase().indexOf(query.trim().toLowerCase())}
						<div
							class="grid h-13 grid-cols-[minmax(0,1fr)_90px_170px_36px] items-center gap-3 rounded-md border-t border-line px-2 [contain-intrinsic-size:auto_3.25rem] [content-visibility:auto] hover:bg-surface-2 {on(
								e
							)
								? 'bg-accent-soft hover:bg-accent-soft'
								: ''}"
							{...entryProps(e)}
							draggable={false}
							onclick={() => openResult(e)}
							onkeydown={(ev) => ev.key === 'Enter' && openResult(e)}
						>
							<div class="flex min-w-0 items-center gap-2.5">
								<Icon name={iconFor(e)} size={18} class={e.dir ? '' : 'text-fg-2'} />
								<div class="flex min-w-0 flex-col">
									<span class="truncate">
										{#if i >= 0}{e.name.slice(0, i)}<mark
												class="rounded-[3px] bg-accent-soft text-inherit"
												>{e.name.slice(i, i + query.trim().length)}</mark
											>{e.name.slice(i + query.trim().length)}{:else}{e.name}{/if}
									</span>
									<span class="truncate font-mono text-[11px] text-fg-3">{parent(e.path)}</span>
								</div>
							</div>
							<div class="text-right font-mono text-xs text-fg-2">
								{shownSize(e)}
							</div>
							<div class="text-xs text-fg-2">{when(e.modified)}</div>
							{@render dots(e)}
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
							{#each [1, 2, 3, 4, 5, 6, 7, 8, 9, 10] as i (i)}
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
						{#each [1, 2, 3, 4, 5, 6, 7, 8] as i (i)}
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
						<p class="mt-1.5 max-w-90 text-fg-2">Drop files here to upload them to {dir}.</p>
					</div>
					<div class="flex gap-2">
						<button class="btn btn-primary" onclick={() => PickUploads(dir).then(queueConflicts)}>
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
						<div class="reveal grid grid-cols-[repeat(auto-fill,minmax(180px,1fr))] gap-3">
							{#each folders as e (e.path)}
								<div
									class="flex flex-col gap-3.5 rounded-lg border bg-surface p-3.5 hover:bg-surface-2 {on(
										e
									)
										? 'border-accent bg-accent-soft hover:bg-accent-soft'
										: 'border-line'}"
									{...entryProps(e)}
								>
									<Icon name="folderFill" size={36} />
									<span class="flex flex-col gap-0.5">
										<span class="truncate font-medium">{e.name}</span>
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
						<div class="reveal grid grid-cols-[repeat(auto-fill,minmax(180px,1fr))] gap-3">
							{#each files as e (e.path)}
								<div
									class="flex flex-col overflow-hidden rounded-lg border bg-surface hover:bg-surface-2 {on(
										e
									)
										? 'border-accent bg-accent-soft hover:bg-accent-soft'
										: 'border-line'}"
									{...entryProps(e)}
								>
									<div
										class="relative flex h-24 items-center justify-center bg-surface-2 text-fg-3"
									>
										<Icon name={iconFor(e)} size={28} />
										{#if previewKind(e.name) === 'image'}
											<img
												src={thumbUrl(e.path, e.modified)}
												alt=""
												loading="lazy"
												class="absolute inset-0 h-full w-full object-cover"
												onerror={(ev) => ev.currentTarget.remove()}
											/>
										{/if}
									</div>
									<div class="flex flex-col gap-0.5 px-3 py-2.5">
										<span class="truncate font-medium">{e.name}</span>
										<span class="text-xs text-fg-3">{bytes(e.size)} · {when(e.modified)}</span>
									</div>
								</div>
							{/each}
						</div>
					</section>
				{/if}
			{:else}
				<section class="flex flex-col gap-1">
					<div class="grid h-8.5 items-center gap-3 px-2 text-xs font-medium text-fg-3 {cols}">
						{@render head('name', 'Name')}
						{@render head('kind', 'Kind', 'hidden @4xl:flex')}
						{@render head('size', 'Size', 'flex justify-end')}
						{@render head('modified', 'Modified')}
						{@render head('created', 'Created', 'hidden @4xl:flex')}
						<div></div>
					</div>
					<div class="reveal">
						{#each rows as e (e.path)}{@render row(e)}{/each}
					</div>
				</section>
			{/if}
		</div>
	</div>
	{#if picked.length > 1}
		<SelectionPanel
			items={picked}
			onaction={(a) =>
				a === 'download'
					? downloadAll(picked)
					: a === 'delete'
						? trashAll(picked)
						: open(a, picked)}
			onclose={() => sel.clear()}
		/>
	{/if}
</div>

{#if preview}
	<Preview
		e={preview}
		items={viewable}
		onchange={(e) => {
			preview = e;
			select([e.path]);
		}}
		onclose={() => (preview = null)}
		ondownload={() => preview && downloadAll([preview])}
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
	>
		{#if !m.items.length}
			{@render item('folderPlus', 'New folder', () => open('mkdir', []), `⇧${mod}N`)}
			{@render item(
				'upload',
				'Upload files…',
				() => PickUploads(dir).then(queueConflicts),
				`${mod}U`
			)}
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
		{:else}
			{@const n = one ? '' : ` ${m.items.length} items`}
			{#if one?.dir}
				<a class="menu-item" href={filesHref(one.path)}
					><Icon name="open" size={15} class="text-fg-2" />Open</a
				>
			{:else if one && previewKind(one.name)}
				{@render item('eye', 'Preview', () => (preview = one))}
			{/if}
			{@render item('download', `Download${n}`, () => downloadAll(m.items))}
			{#if one}
				{@render item('pencil', 'Rename', () => open('rename', [one]))}
				{@render item('link', 'Copy Link', () => copyLink(one))}
			{/if}
			<hr class="my-1 border-line" />
			{@render item('copy', 'Copy', () => (clip = { items: m.items, cut: false }), `${mod}C`)}
			{@render item('cut', 'Cut', () => (clip = { items: m.items, cut: true }), `${mod}X`)}
			{#if one}{@render item('copy', 'Duplicate', () => duplicate(one))}{/if}
			{@render item('folderMove', `Copy${n} to…`, () => open('copy', m.items))}
			{@render item('folderMove', `Move${n} to…`, () => open('move', m.items))}
			<hr class="my-1 border-line" />
			<button class="menu-item text-danger" onclick={() => trashAll(m.items)}>
				<Icon name="trash" size={15} />Move{n} to Trash
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
