<script lang="ts">
	import { EmptyTrash, ListTrash, Purge, Restore, type TrashItem } from '#/shared/api/index.ts';
	import { bytes, filesHref, iconFor, msg, plural, toast, when } from '#/shared/lib/index.ts';
	import { Dialog, Icon } from '#/shared/ui/index.ts';

	let items = $state.raw<TrashItem[]>([]);
	let loading = $state(true);
	let asking = $state(false);
	let doomed = $state<TrashItem | 'all'>('all');

	const load = () =>
		ListTrash()
			.then((l) => (items = l ?? []))
			.catch((e) => toast(msg(e), 'error'))
			.finally(() => (loading = false));
	load();

	const total = $derived(items.reduce((s, t) => s + (t.dir ? 0 : t.size), 0));
	const cols =
		'grid-cols-[minmax(0,1fr)_90px_190px] @4xl:grid-cols-[minmax(0,1fr)_200px_170px_90px_190px]';

	async function act(f: () => Promise<unknown>, done: string) {
		try {
			await f();
			toast(done);
		} catch (e) {
			toast(msg(e), 'error');
		}
		load();
	}
	const restore = (t: TrashItem) => act(() => Restore(t.path), `Restored ${t.name} to ${t.from}`);
	const ask = (t: TrashItem | 'all') => {
		doomed = t;
		asking = true;
	};
	function purge() {
		const t = doomed;
		asking = false;
		if (t === 'all') act(EmptyTrash, 'Trash emptied');
		else act(() => Purge(t.path), `Deleted ${t.name}`);
	}
</script>

<header class="page-header">
	<h1 class="text-[15px] font-semibold tracking-tight">Trash</h1>
	<div class="flex-1"></div>
	<button class="btn btn-danger" disabled={!items.length} onclick={() => ask('all')}>
		<Icon name="trash" size={15} />Empty Trash
	</button>
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
				<div>Name</div>
				<div class="hidden @4xl:block">Original location</div>
				<div class="hidden @4xl:block">Deleted</div>
				<div class="text-right">Size</div>
				<div></div>
			</div>
			<div>
				{#each items as t (t.path)}
					<div
						class="group grid h-11 items-center gap-3 rounded-md border-t border-line px-2 hover:bg-surface-2 {cols}"
					>
						<div class="flex min-w-0 items-center gap-2.5">
							<Icon name={iconFor(t)} size={18} class={t.dir ? '' : 'text-fg-2'} />
							<span class="truncate">{t.name}</span>
						</div>
						<a
							class="hidden truncate font-mono text-xs text-fg-2 hover:text-fg @4xl:block"
							href={filesHref(t.from)}
						>
							{t.from}
						</a>
						<div class="hidden text-xs text-fg-2 @4xl:block">{when(t.deleted)}</div>
						<div class="text-right font-mono text-xs text-fg-2">{t.dir ? '—' : bytes(t.size)}</div>
						<div
							class="flex justify-end gap-1.5 opacity-0 group-focus-within:opacity-100 group-hover:opacity-100"
						>
							<button class="btn h-7" onclick={() => restore(t)}>Restore</button>
							<button class="btn btn-danger h-7" onclick={() => ask(t)}>Delete now</button>
						</div>
					</div>
				{/each}
			</div>
		</section>
	{/if}
</div>

<Dialog
	open={asking}
	title={doomed === 'all' ? 'Empty Trash?' : `Delete “${doomed.name}”?`}
	onclose={() => (asking = false)}
>
	<p class="text-fg-2">
		{doomed === 'all' ? plural(items.length, 'item') : 'It'} will be removed from the server for good.
		This can't be undone.
	</p>
	<div class="mt-4 flex justify-end gap-2">
		<button class="btn" onclick={() => (asking = false)}>Cancel</button>
		<button class="btn btn-danger" onclick={purge}
			>{doomed === 'all' ? 'Empty Trash' : 'Delete'}</button
		>
	</div>
</Dialog>
