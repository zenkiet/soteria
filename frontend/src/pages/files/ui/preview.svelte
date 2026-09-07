<script lang="ts">
	import type { Entry } from '@/shared/api';
	import { bytes, kind, parent, pop, previewKind, previewUrl } from '@/shared/lib';
	import { Icon } from '@/shared/ui';

	let {
		e,
		items,
		onchange,
		onclose,
		ondownload
	}: {
		e: Entry;
		items: Entry[];
		onchange: (e: Entry) => void;
		onclose: () => void;
		ondownload: () => void;
	} = $props();

	const i = $derived(items.findIndex((x) => x.path === e.path));
	const url = $derived(previewUrl(e.path));
	const mode = $derived(previewKind(e.name));

	function go(d: number) {
		const next = items[i + d];
		if (next) onchange(next);
	}

	function onkeydown(ev: KeyboardEvent) {
		if (ev.key === 'Escape') onclose();
		else if (ev.key === 'ArrowLeft') go(-1);
		else if (ev.key === 'ArrowRight') go(1);
	}

	async function text(u: string) {
		return (await fetch(u)).text();
	}

	async function docx(u: string) {
		const [{ default: mammoth }, arrayBuffer] = await Promise.all([
			import('mammoth'),
			fetch(u).then((r) => r.arrayBuffer())
		]);
		return (await mammoth.convertToHtml({ arrayBuffer })).value;
	}

	const content = $derived(mode === 'text' ? text(url) : mode === 'docx' ? docx(url) : null);
</script>

<svelte:window {onkeydown} />

<div class="fixed inset-0 z-20 flex flex-col bg-bg" use:pop>
	<header
		class="flex h-13 shrink-0 items-center gap-3 border-b border-line bg-surface pr-5 pl-21"
		style="--wails-draggable: drag"
	>
		<div class="min-w-0 flex-1">
			<div class="truncate font-medium">{e.name}</div>
			<div class="truncate text-xs text-fg-3">{parent(e.path)} · {kind(e)} · {bytes(e.size)}</div>
		</div>
		{#if items.length > 1}
			<div class="flex items-center gap-1">
				<button
					class="btn btn-ghost btn-icon"
					onclick={() => go(-1)}
					disabled={i <= 0}
					aria-label="Previous"
				>
					<Icon name="chevronLeft" />
				</button>
				<span class="font-mono text-xs text-fg-3">{i + 1} / {items.length}</span>
				<button
					class="btn btn-ghost btn-icon"
					onclick={() => go(1)}
					disabled={i >= items.length - 1}
					aria-label="Next"
				>
					<Icon name="chevronRight" />
				</button>
			</div>
		{/if}
		<button class="btn" onclick={ondownload}><Icon name="download" size={15} />Download</button>
		<div class="flex items-center gap-1.5">
			<kbd>Esc</kbd>
			<button class="btn btn-ghost btn-icon" onclick={onclose} aria-label="Close"
				><Icon name="x" /></button
			>
		</div>
	</header>
	<div class="flex min-h-0 flex-1 items-center justify-center overflow-auto p-6">
		{#key e.path}
			{#if mode === 'image'}
				<img src={url} alt={e.name} class="max-h-full max-w-full rounded-lg object-contain" />
			{:else if mode === 'pdf'}
				<iframe src={url} title={e.name} class="h-full w-full rounded-lg border border-line"
				></iframe>
			{:else if mode === 'video'}
				<!-- svelte-ignore a11y_media_has_caption -->
				<video src={url} controls class="max-h-full max-w-full rounded-lg"></video>
			{:else if mode === 'audio'}
				<audio src={url} controls></audio>
			{:else if content}
				{#await content}
					<p class="text-fg-3">Loading…</p>
				{:then body}
					{#if mode === 'docx'}
						<article
							class="doc h-full w-full max-w-200 overflow-auto rounded-lg border border-line bg-surface px-14 py-12 select-text"
						>
							<!-- eslint-disable-next-line svelte/no-at-html-tags -->
							{@html body}
						</article>
					{:else}
						<pre
							class="h-full w-full max-w-240 overflow-auto rounded-lg border border-line bg-surface p-5 font-mono text-xs leading-relaxed whitespace-pre-wrap select-text">{body}</pre>
					{/if}
				{:catch err}
					<p class="text-danger">{err.message}</p>
				{/await}
			{/if}
		{/key}
	</div>
</div>
