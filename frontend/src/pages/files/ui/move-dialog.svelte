<script lang="ts">
	import { CancelError } from '@wailsio/runtime';
	import { untrack } from 'svelte';
	import { List, Mkdir, type Entry } from '#/shared/api/index.ts';
	import { join, msg, parent, toast, validName } from '#/shared/lib/index.ts';
	import { Dialog, Icon } from '#/shared/ui/index.ts';

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

	const from = $derived(parent(items[0].path));
	// Dropbox opens the picker where the items are: siblings and the parent are the usual targets.
	let cur = $state(untrack(() => from));
	let dirs = $state<Entry[]>([]);
	let folder = $state<string | null>(null);
	const verb = $derived(mode === 'copy' ? 'Copy' : 'Move');
	const title = $derived(
		items.length === 1 ? `${verb} “${items[0].name}” to…` : `${verb} ${items.length} items to…`
	);
	const why = $derived(
		cur === from
			? 'Already in this folder'
			: items.some((e) => e.dir && (cur === e.path || cur.startsWith(e.path + '/')))
				? 'Can’t go inside itself'
				: ''
	);
	const bad = $derived(
		folder === null
			? ''
			: validName(
					folder,
					dirs.map((d) => d.name)
				)
	);

	$effect(() => {
		const l = List(cur);
		l.then(
			(l) => (dirs = (l ?? []).filter((d) => d.dir && !items.some((e) => e.path === d.path))),
			(e) => e instanceof CancelError || toast(msg(e), 'error')
		);
		return () => l.cancel();
	});

	async function create(ev: SubmitEvent) {
		ev.preventDefault();
		if (folder === null || bad) return;
		const to = join(cur, folder.trim());
		try {
			await Mkdir(to);
			folder = null;
			cur = to;
		} catch (e) {
			toast(msg(e), 'error');
		}
	}
</script>

<Dialog open {title} {onclose}>
	<div class="flex items-center gap-2 text-xs text-fg-2">
		<button
			class="btn btn-ghost h-7 w-7 px-0"
			onclick={() => (cur = parent(cur))}
			disabled={cur === '/'}
			aria-label="Up"
			title="Enclosing folder"><Icon name="arrowLeft" size={14} /></button
		>
		<span class="flex-1 truncate font-mono" title={cur}>{cur}</span>
		<button class="btn btn-ghost h-7 text-xs" onclick={() => (folder = '')}>
			<Icon name="folderPlus" size={14} />New folder
		</button>
	</div>
	{#if folder !== null}
		<form class="mt-2 flex items-center gap-2" onsubmit={create}>
			<!-- svelte-ignore a11y_autofocus -->
			<input class="input h-8" bind:value={folder} placeholder="Folder name" autofocus />
			<button class="btn h-8" disabled={!!bad}>Create</button>
		</form>
		{#if bad && folder}<p class="mt-1 text-xs text-danger">{bad}</p>{/if}
	{/if}
	<div
		class="mt-2 flex max-h-60 flex-col gap-px overflow-y-auto rounded-md border border-line p-1.5"
	>
		{#each dirs as d (d.path)}
			<button class="menu-item" onclick={() => (cur = d.path)}>
				<Icon name="folderFill" /><span class="truncate">{d.name}</span>
				<Icon name="chevronRight" size={14} class="ml-auto text-fg-3" />
			</button>
		{:else}
			<p class="px-2.5 py-3 text-xs text-fg-3">No folders here.</p>
		{/each}
	</div>
	<div class="mt-4 flex items-center justify-end gap-2">
		{#if why}<span class="mr-auto text-xs text-fg-3">{why}</span>{/if}
		<button class="btn" onclick={onclose}>Cancel</button>
		<button class="btn btn-primary" disabled={!!why} onclick={() => onmove(cur)}>{verb} here</button
		>
	</div>
</Dialog>
