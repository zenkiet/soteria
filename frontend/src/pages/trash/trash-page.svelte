<script lang="ts">
	import { SvelteSet } from 'svelte/reactivity';
	import {
		changes,
		EmptyTrash,
		ListTrash,
		Purge,
		Restore,
		type TrashItem
	} from '#/shared/api/index.ts';
	import { bytes, filesHref, msg, plural, toast, when } from '#/shared/lib/index.ts';
	import { Dialog, FileIcon, Icon, SelectionBar } from '#/shared/ui/index.ts';

	let items = $state.raw<TrashItem[]>([]);
	let loading = $state(true);
	let asking = $state(false);
	let doomed = $state<TrashItem[] | 'all'>('all');
	const sel = new SvelteSet<string>();
	const picked = $derived(items.filter((t) => sel.has(t.path)));
	let last = '';

	const load = () =>
		ListTrash()
			.then((l) => (items = l ?? []))
			.catch((e) => toast(msg(e), 'error'))
			.finally(() => (loading = false));
	load();

	const total = $derived(items.reduce((s, t) => s + (t.dir ? 0 : t.size), 0));
	const cols =
		'grid-cols-[1rem_minmax(0,1fr)_90px_90px] @4xl:grid-cols-[1rem_minmax(0,1fr)_200px_170px_90px_90px]';

	async function act(f: () => Promise<unknown>, done: string) {
		try {
			await f();
			changes.n++;
			toast(done);
		} catch (e) {
			toast(msg(e), 'error');
		}
		sel.clear();
		load();
	}
	const restore = (ts: TrashItem[]) =>
		act(
			() => Promise.all(ts.map((t) => Restore(t.path))),
			ts.length === 1
				? `Restored ${ts[0].name} to ${ts[0].from}`
				: `Restored ${plural(ts.length, 'item')}`
		);
	const ask = (t: TrashItem[] | 'all') => {
		doomed = t;
		asking = true;
	};
	function purge() {
		const t = doomed;
		asking = false;
		if (t === 'all') act(EmptyTrash, 'Trash emptied');
		else
			act(
				() => Promise.all(t.map((x) => Purge(x.path))),
				t.length === 1 ? `Deleted ${t[0].name}` : `Deleted ${plural(t.length, 'item')}`
			);
	}

	// Same click model as Files: a row click picks only it, ⌘ toggles, Shift extends, the checkbox toggles.
	function pick(t: TrashItem, ev: MouseEvent, toggle = false) {
		const a = items.findIndex((x) => x.path === last);
		if (ev.shiftKey && a >= 0) {
			const b = items.indexOf(t);
			sel.clear();
			for (const x of items.slice(Math.min(a, b), Math.max(a, b) + 1)) sel.add(x.path);
		} else if (toggle || ev.metaKey || ev.ctrlKey) {
			if (!sel.delete(t.path)) sel.add(t.path);
		} else {
			sel.clear();
			sel.add(t.path);
		}
		last = t.path;
	}
</script>

<svelte:window
	onkeydown={(e) => e.key === 'Escape' && !document.querySelector('dialog[open]') && sel.clear()}
/>

<header class="page-header">
	{#if picked.length}
		<SelectionBar count={picked.length} onclear={() => sel.clear()}>
			<button class="btn h-8" onclick={() => restore(picked)}>Restore</button>
			<button class="btn btn-danger h-8" onclick={() => ask(picked)}>
				<Icon name="trash" size={15} />Delete forever
			</button>
		</SelectionBar>
	{:else}
		<h1 class="text-[15px] font-semibold tracking-tight">Trash</h1>
		<div class="flex-1"></div>
		<button class="btn" disabled={!items.length} onclick={() => ask('all')}>Empty Trash</button>
	{/if}
</header>

<div class="@container flex flex-col gap-4 overflow-y-auto p-6">
	<div
		class="flex items-center gap-2.5 rounded-lg border border-line px-3.5 py-2.5 text-xs text-fg-2"
	>
		<Icon name="info" size={15} class="text-fg-3" />
		<span class="flex-1">
			Deleted items stay here for 30 days, then they are removed from the server. They are hidden
			from Files and search.
		</span>
		{#if items.length}
			<span class="font-mono text-fg-3">{plural(items.length, 'item')} · {bytes(total)}</span>
		{/if}
	</div>
	{#if !loading && !items.length}
		<p class="py-10 text-center text-fg-3">Trash is empty.</p>
	{:else}
		<section class="flex flex-col gap-1">
			<div class="grid h-8.5 items-center gap-3 px-2 text-xs font-medium text-fg-3 {cols}">
				<input
					type="checkbox"
					class="check"
					checked={!!items.length && picked.length === items.length}
					indeterminate={!!picked.length && picked.length < items.length}
					onclick={() =>
						picked.length === items.length ? sel.clear() : items.forEach((t) => sel.add(t.path))}
					aria-label="Select all"
				/>
				<div>Name</div>
				<div class="hidden @4xl:block">Original location</div>
				<div class="hidden @4xl:block">Deleted</div>
				<div class="text-right">Size</div>
				<div></div>
			</div>
			<div>
				{#each items as t (t.path)}
					<!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
					<div
						class="group grid h-11 items-center gap-3 rounded-md border-t border-line px-2 hover:bg-surface-2 {cols}"
						class:bg-accent-soft={sel.has(t.path)}
						onclick={(ev) => pick(t, ev)}
					>
						<input
							type="checkbox"
							class="check {sel.size
								? ''
								: 'opacity-0 group-focus-within:opacity-100 group-hover:opacity-100'}"
							checked={sel.has(t.path)}
							onclick={(ev) => {
								ev.stopPropagation();
								pick(t, ev, true);
							}}
							aria-label="Select {t.name}"
						/>
						<div class="flex min-w-0 items-center gap-2.5">
							<FileIcon name={t.name} dir={t.dir} size={18} />
							<span class="truncate" title={t.name}>{t.name}</span>
						</div>
						<div class="hidden min-w-0 items-center gap-2 text-xs text-fg-2 @4xl:flex">
							<span class="truncate font-mono" title={t.from}>{t.from}</span>
							<a
								class="shrink-0 text-fg-3 opacity-0 group-focus-within:opacity-100 group-hover:opacity-100 hover:text-fg"
								href={filesHref(t.from)}
								onclick={(ev) => ev.stopPropagation()}>Show</a
							>
						</div>
						<div class="hidden text-xs text-fg-2 @4xl:block">{when(t.deleted)}</div>
						<div class="text-right font-mono text-xs text-fg-2">{t.dir ? '—' : bytes(t.size)}</div>
						<div
							class="flex justify-end opacity-0 group-focus-within:opacity-100 group-hover:opacity-100"
						>
							<button
								class="btn h-7"
								onclick={(ev) => {
									ev.stopPropagation();
									restore([t]);
								}}>Restore</button
							>
						</div>
					</div>
				{/each}
			</div>
		</section>
	{/if}
</div>

<Dialog
	open={asking}
	title={doomed === 'all'
		? 'Empty Trash?'
		: doomed.length === 1
			? `Delete “${doomed[0].name}”?`
			: `Delete ${plural(doomed.length, 'item')}?`}
	onclose={() => (asking = false)}
>
	<p class="text-fg-2">
		{doomed === 'all' ? plural(items.length, 'item') : doomed.length === 1 ? 'It' : 'They'} will be removed
		from the server for good. This can't be undone.
	</p>
	<div class="mt-4 flex justify-end gap-2">
		<button class="btn" onclick={() => (asking = false)}>Cancel</button>
		<button class="btn btn-danger" onclick={purge}
			>{doomed === 'all' ? 'Empty Trash' : 'Delete forever'}</button
		>
	</div>
</Dialog>
