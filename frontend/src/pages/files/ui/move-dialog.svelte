<script lang="ts">
	import { List, type Entry } from '@/shared/api';
	import { parent } from '@/shared/lib';
	import { Dialog, Icon } from '@/shared/ui';

	let {
		items,
		mode = 'move',
		onmove,
		onclose
	}: {
		items: Entry[];
		mode?: 'move' | 'copy';
		onmove: (dir: string) => void;
		onclose: () => void;
	} = $props();

	let cur = $state('/');
	let dirs = $state<Entry[]>([]);
	const from = $derived(parent(items[0].path));
	const verb = $derived(mode === 'copy' ? 'Copy' : 'Move');
	const title = $derived(
		items.length === 1 ? `${verb} “${items[0].name}” to…` : `${verb} ${items.length} items to…`
	);
	const blocked = $derived(
		cur === from || items.some((e) => e.dir && cur.startsWith(e.path + '/'))
	);

	$effect(() => {
		List(cur).then(
			(l) => (dirs = (l ?? []).filter((d) => d.dir && !items.some((e) => e.path === d.path)))
		);
	});
</script>

<Dialog open {title} {onclose}>
	<div class="flex items-center gap-2 text-xs text-fg-2">
		<button
			class="btn btn-ghost h-7 w-7 px-0"
			onclick={() => (cur = parent(cur))}
			disabled={cur === '/'}
			aria-label="Up"><Icon name="arrowLeft" size={14} /></button
		>
		<span class="truncate font-mono">{cur}</span>
	</div>
	<div
		class="mt-2 flex max-h-60 flex-col gap-px overflow-y-auto rounded-md border border-line p-1.5"
	>
		{#each dirs as d (d.path)}
			<button class="menu-item" onclick={() => (cur = d.path)}>
				<Icon name="folderFill" />{d.name}
				<Icon name="chevronRight" size={14} class="ml-auto text-fg-3" />
			</button>
		{:else}
			<p class="px-2.5 py-3 text-xs text-fg-3">No folders here.</p>
		{/each}
	</div>
	<div class="mt-4 flex justify-end gap-2">
		<button class="btn" onclick={onclose}>Cancel</button>
		<button class="btn btn-primary" disabled={blocked} onclick={() => onmove(cur)}
			>{verb} here</button
		>
	</div>
</Dialog>
