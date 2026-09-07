<script lang="ts">
	import type { Entry } from '@/shared/api';
	import {
		bytes,
		iconFor,
		kind,
		parent,
		previewKind,
		previewUrl,
		slideIn,
		when
	} from '@/shared/lib';
	import { Icon } from '@/shared/ui';

	export type Action = 'preview' | 'download' | 'copy' | 'rename' | 'move' | 'delete';

	let { e, onaction, onclose }: { e: Entry; onaction: (a: Action) => void; onclose: () => void } =
		$props();

	const mode = $derived(previewKind(e.name));

	const meta = $derived([
		['Modified', when(e.modified)],
		['Location', parent(e.path)],
		['Size', `${e.size.toLocaleString()} bytes`],
		['Content-Type', e.contentType || '—'],
		['ETag', e.etag || '—']
	]);
</script>

<aside
	use:slideIn
	class="flex w-80 shrink-0 flex-col gap-4 overflow-hidden border-l border-line p-5"
>
	<div class="relative h-45 overflow-hidden rounded-lg bg-surface-2 text-fg-3">
		<button
			class="flex h-full w-full items-center justify-center disabled:cursor-default"
			onclick={() => onaction('preview')}
			disabled={!mode}
			aria-label="Preview"
		>
			{#if mode === 'image'}
				<img src={previewUrl(e.path)} alt="" class="h-full w-full object-cover" />
			{:else}
				<Icon name={iconFor(e)} size={32} class="opacity-70" />
			{/if}
		</button>
		<button
			class="btn btn-ghost absolute top-2 right-2 h-7 w-7 px-0"
			onclick={onclose}
			aria-label="Close"><Icon name="x" size={14} /></button
		>
	</div>
	<div>
		<div class="text-[15px] font-semibold tracking-tight break-all">{e.name}</div>
		<div class="mt-1 text-xs text-fg-2">{kind(e)} · {bytes(e.size)}</div>
	</div>
	<button class="btn btn-primary h-9" onclick={() => onaction('download')}>
		<Icon name="download" size={15} />Download
	</button>
	<dl class="text-xs">
		{#each meta as [k, v] (k)}
			<div class="flex justify-between gap-4 border-t border-line py-2.25">
				<dt class="shrink-0 text-fg-3">{k}</dt>
				<dd class="truncate font-mono">{v}</dd>
			</div>
		{/each}
	</dl>
	<div class="flex-1"></div>
	<div class="-mx-2.5 flex flex-col gap-0.5">
		<button class="menu-item" onclick={() => onaction('copy')}>
			<Icon name="link" size={15} class="text-fg-2" />Copy WebDAV URL
		</button>
		<button class="menu-item" onclick={() => onaction('rename')}>
			<Icon name="pencil" size={15} class="text-fg-2" />Rename
		</button>
		<button class="menu-item" onclick={() => onaction('move')}>
			<Icon name="folderMove" size={15} class="text-fg-2" />Move to…
		</button>
		<button class="menu-item text-danger" onclick={() => onaction('delete')}>
			<Icon name="trash" size={15} />Move to Trash
		</button>
	</div>
</aside>
