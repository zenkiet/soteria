<script lang="ts">
	import { page } from '$app/state';
	import type { Entry, Server, Usage } from '#/shared/api/index.ts';
	import {
		base,
		bytes,
		isStarred,
		kind,
		parent,
		plural,
		previewKind,
		sum,
		thumbUrl,
		toggleStar,
		when
	} from '#/shared/lib/index.ts';
	import { FileIcon, Icon } from '#/shared/ui/index.ts';
	import { downloadAll, openFile } from '#/widgets/preview/index.ts';

	let {
		items,
		rows,
		dir,
		usage,
		size,
		onopen,
		oncopylink
	}: {
		items: Entry[];
		rows: Entry[];
		dir: string;
		usage: { [path: string]: Usage | undefined };
		size: (e: Entry) => number | null;
		onopen: (e: Entry) => void;
		oncopylink: (e: Entry) => void;
	} = $props();

	const server = $derived(page.data.server as Server);
	const one = $derived(items.length === 1 ? items[0] : null);
	const total = (xs: Entry[]) => bytes(sum(xs, (e) => size(e) ?? 0));
	const newest = (xs: Entry[]) => xs.reduce((a, e) => (e.modified > a ? e.modified : a), '');
</script>

{#snippet field(label: string, value: string)}
	<dt class="text-fg-3">{label}</dt>
	<dd class="truncate select-text">{value}</dd>
{/snippet}

<aside class="flex h-full flex-col gap-4 overflow-y-auto bg-bg p-5">
	<div
		class="relative flex h-36 shrink-0 items-center justify-center overflow-hidden rounded-lg bg-surface-2"
	>
		{#if one}
			<FileIcon name={one.name} dir={one.dir} size={64} />
			{#if previewKind(one.name) === 'image'}
				<!-- A failed thumbnail removes its img; keying gives the next item a fresh one. -->
				{#key one.path}
					<img
						src={thumbUrl(one.path, one.modified)}
						alt=""
						class="absolute inset-0 h-full w-full bg-surface-2 object-scale-down"
						onerror={(ev) => ev.currentTarget.remove()}
					/>
				{/key}
			{/if}
		{:else if items.length}
			<Icon name="copy" size={48} class="text-fg-3" />
		{:else}
			<FileIcon name="" dir size={64} />
		{/if}
	</div>
	{#if one}
		{@const u = usage[one.path]}
		{@const star = isStarred(server.id, one.path)}
		<div class="flex flex-col gap-0.5">
			<span class="font-semibold break-all select-text">{one.name}</span>
			<span class="text-xs text-fg-3">{kind(one)} · {bytes(size(one) ?? -1)}</span>
		</div>
		<dl class="grid grid-cols-[auto_minmax(0,1fr)] gap-x-3 gap-y-1.5 text-xs">
			{@render field('Where', parent(one.path))}
			{#if one.dir && u?.known}{@render field('Contains', plural(u.items, 'item'))}{/if}
			{@render field('Modified', when(one.modified))}
			{#if !one.created.startsWith('0001')}{@render field('Created', when(one.created))}{/if}
		</dl>
		<div class="flex flex-wrap gap-2">
			{#if one.dir || previewKind(one.name)}
				<button class="btn" onclick={() => onopen(one)}>
					<Icon name={one.dir ? 'open' : 'eye'} size={15} />{one.dir ? 'Open' : 'Preview'}
				</button>
			{:else}
				<button class="btn" onclick={() => openFile(one)}>
					<Icon name="open" size={15} />Open in default app
				</button>
			{/if}
			<button class="btn" onclick={() => downloadAll([one])}>
				<Icon name="download" size={15} />Download
			</button>
			<button class="btn" onclick={() => oncopylink(one)}>
				<Icon name="link" size={15} />Copy link
			</button>
			<button class="btn" onclick={() => toggleStar(server.id, [one])}>
				<Icon name={star ? 'starFill' : 'star'} size={15} class={star ? 'text-warn' : ''} />
				{star ? 'Unstar' : 'Star'}
			</button>
		</div>
	{:else if items.length}
		<div class="flex flex-col gap-0.5">
			<span class="font-semibold">{plural(items.length, 'item')}</span>
			<span class="text-xs text-fg-3">{total(items)}</span>
		</div>
		<dl class="grid grid-cols-[auto_minmax(0,1fr)] gap-x-3 gap-y-1.5 text-xs">
			{@render field('Kinds', [...new Set(items.map(kind))].join(', '))}
			{@render field('Modified', when(newest(items)))}
		</dl>
	{:else}
		<div class="flex flex-col gap-0.5">
			<span class="font-semibold break-all">{base(dir) || 'Files'}</span>
			<span class="text-xs text-fg-3">{plural(rows.length, 'item')} · {total(rows)}</span>
		</div>
		{#if dir !== '/'}
			<dl class="grid grid-cols-[auto_minmax(0,1fr)] gap-x-3 gap-y-1.5 text-xs">
				{@render field('Where', parent(dir))}
			</dl>
		{/if}
	{/if}
</aside>
