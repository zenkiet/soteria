<script lang="ts">
	import type { Entry } from '@/shared/api';
	import { bytes, iconFor, slideIn } from '@/shared/lib';
	import { Icon } from '@/shared/ui';

	export type BulkAction = 'download' | 'move' | 'delete';

	let {
		items,
		onaction,
		onclose
	}: { items: Entry[]; onaction: (a: BulkAction) => void; onclose: () => void } = $props();

	const files = $derived(items.filter((e) => !e.dir));
	const total = $derived(files.reduce((s, e) => s + e.size, 0));
	const plural = (n: number, w: string) => `${n} ${w}${n === 1 ? '' : 's'}`;
</script>

<aside
	use:slideIn
	class="flex w-80 shrink-0 flex-col gap-4 overflow-hidden border-l border-line p-5"
>
	<div
		class="relative flex h-45 items-center justify-center gap-4 rounded-lg bg-surface-2 text-fg-3"
	>
		{#each items.slice(0, 3) as e (e.path)}
			<Icon name={iconFor(e)} size={32} class={e.dir ? '' : 'opacity-70'} />
		{/each}
		<button
			class="btn btn-ghost absolute top-2 right-2 h-7 w-7 px-0"
			onclick={onclose}
			aria-label="Clear selection"><Icon name="x" size={14} /></button
		>
	</div>
	<div>
		<div class="text-[15px] font-semibold tracking-tight">{items.length} items selected</div>
		<div class="mt-1 text-xs text-fg-2">
			{plural(items.length - files.length, 'folder')}, {plural(files.length, 'file')} · {bytes(
				total
			)}
		</div>
	</div>
	<button class="btn btn-primary h-9" onclick={() => onaction('download')}>
		<Icon name="download" size={15} />Download {plural(items.length, 'item')}
	</button>
	<div class="min-h-0 overflow-y-auto text-xs">
		{#each items as e (e.path)}
			<div class="flex items-center gap-2.5 border-t border-line py-2.25">
				<Icon name={iconFor(e)} size={16} class={e.dir ? '' : 'text-fg-2'} />
				<span class="flex-1 truncate">{e.name}</span>
				<span class="font-mono text-fg-3">{e.dir ? '—' : bytes(e.size)}</span>
			</div>
		{/each}
	</div>
	<div class="flex-1"></div>
	<div class="-mx-2.5 flex flex-col gap-0.5">
		<button class="menu-item" onclick={() => onaction('move')}>
			<Icon name="folderMove" size={15} class="text-fg-2" />Move {items.length} items to…
		</button>
		<button class="menu-item text-danger" onclick={() => onaction('delete')}>
			<Icon name="trash" size={15} />Move {items.length} items to Trash
		</button>
	</div>
</aside>
