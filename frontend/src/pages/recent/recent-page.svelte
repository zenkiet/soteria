<script lang="ts">
	import { goto } from '$app/navigation';
	import { Recent, Reindex, indexStatus, type Entry } from '#/shared/api/index.ts';
	import { ago, bytes, filesHref, iconFor, parent, when } from '#/shared/lib/index.ts';
	import { Icon } from '#/shared/ui/index.ts';

	let items = $state.raw<Entry[]>([]);
	const idx = $derived(indexStatus.current);

	const load = () => Recent(60).then((l) => (items = l ?? []));
	load();
	$effect(() => {
		if (idx?.done) load();
	});

	const day = 86_400_000;
	const today = Date.parse(new Date().toDateString());
	const bucket = (e: Entry) => {
		const t = Date.parse(e.modified);
		return t >= today
			? 'Today'
			: t >= today - day
				? 'Yesterday'
				: t >= today - 6 * day
					? 'Earlier this week'
					: 'Older';
	};
	const groups = $derived(
		['Today', 'Yesterday', 'Earlier this week', 'Older']
			.map((g) => [g, items.filter((e) => bucket(e) === g)] as const)
			.filter(([, l]) => l.length)
	);
	const show = (e: Entry) =>
		goto(filesHref(parent(e.path)) + '?focus=' + encodeURIComponent(e.path));
</script>

<header class="page-header">
	<h1 class="text-[15px] font-semibold tracking-tight">Recent</h1>
	<div class="flex-1"></div>
	{#if idx?.at}
		<span class="text-xs text-fg-3">Recently modified on the server · indexed {ago(idx.at)}</span>
	{/if}
	<button class="btn" onclick={() => Reindex()}><Icon name="refresh" size={15} />Refresh</button>
</header>

<div class="flex flex-col gap-6 overflow-y-auto p-6">
	{#if !items.length}
		<p class="text-fg-3">{idx && !idx.done ? 'Building the index…' : 'Nothing indexed yet.'}</p>
	{/if}
	{#each groups as [label, list] (label)}
		<section class="flex flex-col gap-1">
			<h2 class="font-medium">{label} <span class="font-normal text-fg-3">{list.length}</span></h2>
			<div class="reveal">
				{#each list as e (e.path)}
					<button
						class="grid h-13 w-full grid-cols-[minmax(0,1fr)_90px_170px] items-center gap-3 border-t border-line px-2 text-left hover:bg-surface-2"
						onclick={() => show(e)}
					>
						<div class="flex min-w-0 items-center gap-2.5">
							<Icon name={iconFor(e)} size={18} class="text-fg-2" />
							<div class="flex min-w-0 flex-col">
								<span class="truncate">{e.name}</span>
								<span class="truncate font-mono text-[11px] text-fg-3">{parent(e.path)}</span>
							</div>
						</div>
						<div class="text-right font-mono text-xs text-fg-2">{bytes(e.size)}</div>
						<div class="text-xs text-fg-2">{when(e.modified)}</div>
					</button>
				{/each}
			</div>
		</section>
	{/each}
</div>
