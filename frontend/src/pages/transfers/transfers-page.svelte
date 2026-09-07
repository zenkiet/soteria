<script lang="ts">
	import {
		Cancel,
		CancelGroup,
		Clear,
		Retry,
		Reveal,
		Transfers,
		transfers,
		type Transfer
	} from '@/shared/api';
	import { bytes } from '@/shared/lib';
	import { Icon } from '@/shared/ui';

	const live = (t: Transfer) => t.status === 'running' || t.status === 'queued';
	const active = $derived(transfers.list.filter(live));
	const finished = $derived(transfers.list.filter((t) => !live(t)).reverse());
	const groups = $derived([...new Set(active.map((t) => t.group).filter(Boolean))]);
	const pct = (t: Transfer) => (t.total ? Math.round((t.done / t.total) * 100) : 0);
	const sum = (ts: Transfer[], f: (t: Transfer) => number) => ts.reduce((s, t) => s + f(t), 0);

	async function clear() {
		await Clear();
		transfers.list = (await Transfers()) ?? [];
	}
</script>

{#snippet group(g: string)}
	{@const all = transfers.list.filter((t) => t.group === g)}
	{@const done = sum(all, (t) => t.done)}
	{@const total = sum(all, (t) => t.total)}
	<div class="my-1.5 flex items-center gap-3.5 rounded-lg border border-line py-3 pr-3 pl-3.5">
		<Icon name="folderFill" size={28} />
		<div class="flex min-w-0 flex-1 flex-col gap-1.5">
			<div class="flex justify-between gap-3">
				<span class="truncate font-medium"
					>{all[0]?.kind === 'download' ? 'Downloading' : 'Uploading'} folder “{g.slice(
						g.lastIndexOf('/') + 1
					)}”{all[0]?.kind === 'upload' ? ` to ${g.slice(0, g.lastIndexOf('/')) || '/'}` : ''}</span
				>
				<span class="font-mono text-xs text-fg-3">
					{all.filter((t) => t.status === 'done').length} / {all.length} files
				</span>
			</div>
			<div class="h-1 overflow-hidden rounded-full bg-surface-2">
				<div class="h-full bg-accent" style="width:{total ? (done / total) * 100 : 0}%"></div>
			</div>
			<div class="text-xs text-fg-3">{bytes(done)} of {bytes(total)}</div>
		</div>
		<button class="btn" onclick={() => CancelGroup(g)}>Cancel all</button>
	</div>
{/snippet}

{#snippet row(t: Transfer)}
	<div
		class="grid grid-cols-[36px_minmax(0,1fr)_260px_110px] items-center gap-4 border-t border-line py-3 pr-2"
	>
		<div class="flex h-9 w-9 items-center justify-center rounded-lg bg-surface-2 text-fg-2">
			<Icon name={t.kind === 'upload' ? 'upload' : 'download'} />
		</div>
		<div class="flex min-w-0 flex-col gap-0.5">
			<div class="truncate font-medium">{t.name}</div>
			<div class="truncate text-xs text-fg-3">
				{t.kind === 'upload' ? `To ${t.remote}` : `To ${t.local}`} · {bytes(t.total)}
			</div>
		</div>
		{#if t.status === 'running'}
			<div class="flex flex-col gap-1.5">
				<div class="h-1 overflow-hidden rounded-full bg-surface-2">
					<div class="h-full bg-accent" style="width:{pct(t)}%"></div>
				</div>
				<div class="flex justify-between font-mono text-[11px] text-fg-3">
					<span>{pct(t)}%</span><span>{bytes(t.done)} of {bytes(t.total)}</span>
				</div>
			</div>
		{:else if t.status === 'queued'}
			<div class="text-xs text-fg-3">Waiting</div>
		{:else if t.status === 'done'}
			<div class="flex items-center gap-2 text-xs text-fg-3">
				<Icon name="check" size={14} class="text-ok" />Finished
			</div>
		{:else}
			<div class="truncate text-xs text-danger" title={t.error}>{t.error || t.status}</div>
		{/if}
		<div class="flex justify-end">
			{#if live(t)}
				<button class="btn btn-ghost h-7 w-7 px-0" onclick={() => Cancel(t.id)} aria-label="Cancel">
					<Icon name="x" size={14} />
				</button>
			{:else if t.status === 'done' && t.kind === 'download'}
				<button class="text-xs text-accent-fg hover:underline" onclick={() => Reveal(t.local)}>
					Show in Finder
				</button>
			{:else if t.status !== 'done'}<button
					class="text-xs font-medium text-accent-fg hover:underline"
					onclick={() => Retry(t.id)}>Retry</button
				>{/if}
		</div>
	</div>
{/snippet}

<header
	class="flex h-13 shrink-0 items-center gap-3 border-b border-line px-5"
	style="--wails-draggable: drag"
>
	<h1 class="text-[15px] font-semibold tracking-tight">Transfers</h1>
	<span class="text-xs text-fg-3">Transfers can't be paused. Cancel and retry instead.</span>
	<div class="flex-1"></div>
	<button class="btn" onclick={clear} disabled={!finished.length}>Clear finished</button>
</header>

<div class="flex flex-col gap-7 overflow-y-auto p-6">
	{#if !transfers.list.length}
		<p class="text-fg-3">No transfers yet. Uploads and downloads show up here.</p>
	{/if}
	{#if active.length}
		<section class="flex flex-col gap-1">
			<h2 class="font-medium">Active <span class="font-normal text-fg-3">{active.length}</span></h2>
			{#each groups as g (g)}{@render group(g)}{/each}
			{#each active as t (t.id)}{@render row(t)}{/each}
		</section>
	{/if}
	{#if finished.length}
		<section class="flex flex-col gap-1">
			<h2 class="font-medium">
				Finished <span class="font-normal text-fg-3">{finished.length}</span>
			</h2>
			{#each finished as t (t.id)}{@render row(t)}{/each}
		</section>
	{/if}
</div>
