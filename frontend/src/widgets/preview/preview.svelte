<script lang="ts">
	import { page } from '$app/state';
	import Panzoom from '@panzoom/panzoom';
	import { Browser } from '@wailsio/runtime';
	import type { Entry, Server } from '#/shared/api/index.ts';
	import {
		bytes,
		ext,
		isStarred,
		kind,
		mac,
		parent,
		previewKind,
		toggleStar
	} from '#/shared/lib/index.ts';
	import { FileIcon, Icon } from '#/shared/ui/index.ts';
	import { downloadAll, openFile } from './download.ts';

	let {
		e,
		items,
		onchange,
		onclose,
		onshow
	}: {
		e: Entry;
		items: Entry[];
		onchange: (e: Entry) => void;
		onclose: () => void;
		onshow?: (e: Entry) => void;
	} = $props();

	const server = $derived(page.data.server as Server);
	const star = $derived(isStarred(server.id, e.path));

	const i = $derived(items.findIndex((x) => x.path === e.path));
	const url = $derived('/preview?path=' + encodeURIComponent(e.path));
	const mode = $derived(previewKind(e.name));

	function go(d: number) {
		const next = items[i + d];
		if (next) onchange(next);
	}

	function onkeydown(ev: KeyboardEvent) {
		// Players keep their own arrow and space keys.
		if (ev.target instanceof HTMLMediaElement) return;
		if (ev.key === 'ArrowLeft') go(-1);
		else if (ev.key === 'ArrowRight') go(1);
		// Like Quick Look, the Space that opened the preview closes it.
		else if (ev.key === ' ') {
			ev.preventDefault();
			onclose();
		}
	}

	async function get(u: string, init?: RequestInit) {
		const r = await fetch(u, init);
		if (!r.ok) throw new Error(await r.text());
		return r;
	}

	// Only the first MiB: a multi-gigabyte log would otherwise hang the webview.
	async function head(u: string) {
		const r = await get(u, { headers: { Range: 'bytes=0-1048575' } });
		const more = Number(r.headers.get('Content-Range')?.split('/')[1]) > 1 << 20;
		return [await r.text(), more] as const;
	}

	// The document is untrusted and this page holds the Wails bridge: only web and mail links survive.
	async function safe(html: string) {
		const { default: DOMPurify } = await import('dompurify');
		return { html: DOMPurify.sanitize(html, { ALLOWED_URI_REGEXP: /^(?:https?|mailto):/i }) };
	}

	async function code(u: string, x: string) {
		const [[t, more], { highlight }] = await Promise.all([head(u), import('./highlight.ts')]);
		const text = more ? t + '\n…' : t;
		return { text, code: highlight(text, x) };
	}

	async function md(u: string) {
		const [[t], { marked }] = await Promise.all([head(u), import('marked')]);
		return safe(await marked.parse(t));
	}

	// A MiB of CSV can be tens of thousands of rows, more than a table lays out smoothly.
	async function csv(u: string) {
		const [[t, more], { default: Papa }] = await Promise.all([head(u), import('papaparse')]);
		const rows = Papa.parse<string[]>(t, { skipEmptyLines: true }).data;
		if (more) rows.pop();
		return { head: rows[0] ?? [], rows: rows.slice(1, 1001), more: more || rows.length > 1001 };
	}

	async function docx(u: string) {
		const [{ default: mammoth }, arrayBuffer] = await Promise.all([
			import('mammoth'),
			get(u).then((r) => r.arrayBuffer())
		]);
		return safe((await mammoth.convertToHtml({ arrayBuffer })).value);
	}

	// Panzoom also styles the parent, which destroy() leaves behind for the next preview.
	function zoom(img: HTMLImageElement) {
		const pz = Panzoom(img, { minScale: 1, maxScale: 8, panOnlyWhenZoomed: true });
		const box = img.parentElement!;
		// Pinch and ⌘-scroll zoom; a plain scroll pans a zoomed image, as in Preview.
		const wheel = (ev: WheelEvent) => {
			if (ev.ctrlKey || ev.metaKey) pz.zoomWithWheel(ev);
			else if (pz.getScale() > 1) {
				ev.preventDefault();
				pz.pan(-ev.deltaX, -ev.deltaY, { relative: true });
			}
		};
		box.addEventListener('wheel', wheel);
		img.addEventListener('dblclick', (ev) =>
			pz.getScale() > 1 ? pz.reset() : pz.zoomToPoint(2, ev)
		);
		return () => {
			box.removeEventListener('wheel', wheel);
			pz.destroy();
			pz.resetStyle();
		};
	}

	function onclick(ev: MouseEvent) {
		const a = (ev.target as Element).closest<HTMLAnchorElement>('.doc a[href]');
		if (!a) return;
		ev.preventDefault();
		Browser.OpenURL(a.href);
	}

	const content = $derived(
		mode === 'text'
			? code(url, ext(e.name))
			: mode === 'md'
				? md(url)
				: mode === 'csv'
					? csv(url)
					: mode === 'docx'
						? docx(url)
						: null
	);
</script>

<svelte:window {onkeydown} {onclick} />

<!-- Focus starts on the dialog itself, not on its first button. -->
<dialog
	class="m-0 h-full max-h-none w-full max-w-none flex-col bg-bg p-0 text-fg open:flex"
	aria-label={e.name}
	tabindex="-1"
	{onclose}
	{@attach (d) => {
		d.showModal();
		d.focus();
	}}
>
	<header class="page-header bg-surface pr-5 {mac ? 'pl-25' : ''}">
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
		<div class="min-w-0 flex-1">
			<div class="truncate font-medium">{e.name}</div>
			<div class="truncate text-xs text-fg-3">{parent(e.path)} · {kind(e)} · {bytes(e.size)}</div>
		</div>
		{#if onshow}
			<button class="btn btn-ghost" onclick={() => onshow(e)}
				><Icon name="open" size={15} />Show in folder</button
			>
		{/if}
		<button
			class="btn btn-ghost btn-icon"
			onclick={() => toggleStar(server.id, [e])}
			aria-label={star ? 'Unstar' : 'Star'}
		>
			<Icon name={star ? 'starFill' : 'star'} class={star ? 'text-warn' : ''} />
		</button>
		<!-- A file the window can't show gets Download and Open in default app in its place. -->
		{#if mode}
			<button class="btn" onclick={() => downloadAll([e])}
				><Icon name="download" size={15} />Download</button
			>
		{/if}
		<div class="flex items-center gap-1.5">
			<kbd>Esc</kbd>
			<button class="btn btn-ghost btn-icon" onclick={onclose} aria-label="Close"
				><Icon name="x" /></button
			>
		</div>
	</header>
	<div class="flex min-h-0 flex-1 items-center justify-center overflow-auto p-6">
		{#key e.path}
			<div
				class="flex h-full w-full items-center justify-center motion-safe:animate-[appear_.15s_ease-out]"
			>
				{#if mode === 'image'}
					<img
						src={url}
						alt={e.name}
						class="max-h-full max-w-full rounded-lg object-contain"
						{@attach zoom}
					/>
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
						<!-- eslint-disable svelte/no-at-html-tags -- the HTML comes from DOMPurify or highlight.js -->
						{#if 'rows' in body}
							<div
								class="doc h-full w-full overflow-auto rounded-lg border border-line bg-surface p-5 select-text"
							>
								<table>
									<thead>
										<tr>
											{#each body.head as cell, c (c)}<th>{cell}</th>{/each}
										</tr>
									</thead>
									<tbody>
										{#each body.rows as row, r (r)}
											<tr>
												{#each row as cell, c (c)}<td>{cell}</td>{/each}
											</tr>
										{/each}
									</tbody>
								</table>
								{#if body.more}<p class="hint">Showing the first 1,000 rows.</p>{/if}
							</div>
						{:else if 'html' in body}
							<article
								class="doc h-full w-full max-w-200 overflow-auto rounded-lg border border-line bg-surface px-14 py-12 select-text"
							>
								{@html body.html}
							</article>
						{:else}
							<pre
								class="h-full w-full max-w-240 overflow-auto rounded-lg border border-line bg-surface p-5 font-mono text-xs leading-relaxed whitespace-pre-wrap select-text">{#if body.code}{@html body.code}{:else}{body.text}{/if}</pre>
						{/if}
						<!-- eslint-enable svelte/no-at-html-tags -->
					{:catch err}
						<p class="text-danger">{err.message}</p>
					{/await}
				{:else}
					<div class="flex flex-col items-center gap-4 text-center">
						<FileIcon name={e.name} size={96} />
						<p class="text-fg-2">This file can't be previewed here.</p>
						<div class="flex gap-2">
							<button class="btn" onclick={() => downloadAll([e])}
								><Icon name="download" size={15} />Download</button
							>
							<button class="btn btn-primary" onclick={() => openFile(e)}
								><Icon name="open" size={15} />Open in default app</button
							>
						</div>
					</div>
				{/if}
			</div>
		{/key}
	</div>
</dialog>
