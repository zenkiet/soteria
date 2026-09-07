<script lang="ts">
	import type { Conflict } from '@/shared/api';
	import { bytes, when } from '@/shared/lib';
	import { Dialog } from '@/shared/ui';

	export type Resolution = 'skip' | 'keep' | 'replace' | 'merge';

	let {
		c,
		remaining,
		onresolve
	}: { c: Conflict; remaining: number; onresolve: (mode: Resolution, all: boolean) => void } =
		$props();

	let all = $state(false);
	const name = $derived(c.remote.slice(c.remote.lastIndexOf('/') + 1));
</script>

{#snippet card(label: string, size: number, date: string)}
	<div class="flex flex-1 flex-col gap-0.5 rounded-md bg-surface-2 px-3 py-2.5">
		<span class="text-[11px] font-medium tracking-[0.06em] text-fg-3 uppercase">{label}</span>
		<span class="font-mono">{c.dir ? 'Folder' : bytes(size)}</span>
		<span class="text-xs text-fg-2">{when(date)}</span>
	</div>
{/snippet}

<Dialog open title="“{name}” already exists" onclose={() => onresolve('skip', all)}>
	<p class="text-fg-2">
		{c.dir
			? 'A folder with this name is already here. Merge adds your files into it and replaces files with the same name.'
			: 'A file with this name is already here. Keep both uploads yours under a new name.'}
	</p>
	<div class="mt-4 flex gap-2.5">
		{@render card('On server', c.size, c.modified)}
		{@render card('Yours', c.localSize, c.localModified)}
	</div>
	{#if remaining}
		<label class="mt-4 flex items-center gap-2.5 text-xs text-fg-2">
			<input type="checkbox" bind:checked={all} class="accent-primary" />
			Do this for the remaining {remaining}
			{remaining === 1 ? 'item' : 'items'}
		</label>
	{/if}
	<div class="mt-4 flex justify-end gap-2">
		<button class="btn" onclick={() => onresolve('skip', all)}>Skip</button>
		<button class="btn" onclick={() => onresolve('keep', all)}>Keep both</button>
		<button class="btn btn-primary" onclick={() => onresolve(c.dir ? 'merge' : 'replace', all)}>
			{c.dir ? 'Merge' : 'Replace'}
		</button>
	</div>
</Dialog>
