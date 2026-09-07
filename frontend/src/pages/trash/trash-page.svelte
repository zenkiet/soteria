<script lang="ts">
	import { EmptyTrash, ListTrash, Purge, Restore, type TrashItem } from '@/shared/api';
	import { bytes, filesHref, iconFor, msg, reveal, toast, when } from '@/shared/lib';
	import { Dialog, Icon } from '@/shared/ui';

	let items = $state<TrashItem[]>([]);
	let loading = $state(true);
	let confirm = $state(false);

	const load = () =>
		ListTrash()
			.then((l) => (items = l ?? []))
			.catch((e) => toast(msg(e), 'error'))
			.finally(() => (loading = false));
	load();

	const total = $derived(items.reduce((s, t) => s + (t.dir ? 0 : t.size), 0));
	const cols = 'grid-cols-[minmax(0,1fr)_200px_170px_90px_190px]';

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
	const purge = (t: TrashItem) => act(() => Purge(t.path), `Deleted ${t.name}`);
	function empty() {
		confirm = false;
		act(EmptyTrash, 'Trash emptied');
	}
</script>

<header
	class="flex h-13 shrink-0 items-center gap-3 border-b border-line px-5"
	style="--wails-draggable: drag"
>
	<h1 class="text-[15px] font-semibold tracking-tight">Trash</h1>
	<div class="flex-1"></div>
	<button class="btn btn-danger" disabled={!items.length} onclick={() => (confirm = true)}>
		<Icon name="trash" size={15} />Empty Trash
	</button>
</header>

<div class="flex flex-col gap-4 overflow-y-auto p-6">
	<div
		class="flex items-center gap-2.5 rounded-lg border border-line px-3.5 py-2.5 text-xs text-fg-2"
	>
		<Icon name="info" size={15} class="text-fg-3" />
		<span class="flex-1">
			Deleted items stay here for 30 days, then they are removed from the server. They are hidden
			from Files and search.
		</span>
		{#if items.length}
			<span class="font-mono text-fg-3">{items.length} items · {bytes(total)}</span>
		{/if}
	</div>
	{#if !loading && !items.length}
		<p class="py-10 text-center text-fg-3">Trash is empty.</p>
	{:else}
		<section class="flex flex-col gap-1">
			<div class="grid h-8.5 items-center gap-3 px-2 text-xs font-medium text-fg-3 {cols}">
				<div>Name</div>
				<div>Original location</div>
				<div>Deleted</div>
				<div class="text-right">Size</div>
				<div></div>
			</div>
			<div use:reveal>
				{#each items as t (t.path)}
					<div
						class="group grid h-11 items-center gap-3 rounded-md border-t border-line px-2 hover:bg-surface-2 {cols}"
					>
						<div class="flex min-w-0 items-center gap-2.5">
							<Icon name={iconFor(t)} size={18} class={t.dir ? '' : 'text-fg-2'} />
							<span class="truncate">{t.name}</span>
						</div>
						<a class="truncate font-mono text-xs text-fg-2 hover:text-fg" href={filesHref(t.from)}>
							{t.from}
						</a>
						<div class="text-xs text-fg-2">{when(t.deleted)}</div>
						<div class="text-right font-mono text-xs text-fg-2">{t.dir ? '—' : bytes(t.size)}</div>
						<div class="flex justify-end gap-1.5 opacity-0 group-hover:opacity-100">
							<button class="btn h-7" onclick={() => restore(t)}>Restore</button>
							<button class="btn btn-danger h-7" onclick={() => purge(t)}>Delete now</button>
						</div>
					</div>
				{/each}
			</div>
		</section>
	{/if}
</div>

<Dialog open={confirm} title="Empty Trash?" onclose={() => (confirm = false)}>
	<p class="text-fg-2">
		{items.length} items will be removed from the server for good. This can't be undone.
	</p>
	<div class="mt-4 flex justify-end gap-2">
		<button class="btn" onclick={() => (confirm = false)}>Cancel</button>
		<button class="btn btn-danger" onclick={empty}>Empty Trash</button>
	</div>
</Dialog>
