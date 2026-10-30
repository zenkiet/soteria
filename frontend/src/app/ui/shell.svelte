<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { untrack, type Snippet } from 'svelte';
	import { SvelteSet } from 'svelte/reactivity';
	import {
		changes,
		Disconnect,
		indexStatus,
		List,
		live,
		Mount,
		Ping,
		Quota,
		SignOut,
		transfers,
		update,
		type Entry,
		type QuotaInfo,
		type Server
	} from '#/shared/api/index.ts';
	import {
		base,
		bytes,
		collator,
		desktop,
		filesHref,
		focusHref,
		home,
		listed,
		mac,
		net,
		prefs,
		setPrefs,
		starred
	} from '#/shared/lib/index.ts';
	import { FileIcon, Icon, Split } from '#/shared/ui/index.ts';
	import DriveIntro from './drive-intro.svelte';
	import UploadPanel from './upload-panel.svelte';

	let { server, children }: { server: Server; children: Snippet } = $props();

	let kids = $state.raw<Record<string, Entry[]>>({});
	const open = new SvelteSet<string>();
	const fetchKids = (p: string) =>
		List(p).then(
			(l) =>
				(kids = {
					...kids,
					[p]: (l ?? []).filter((e) => e.dir).sort((a, b) => collator.compare(a.name, b.name))
				}),
			() => {}
		);
	const toggle = (p: string) => {
		auto.delete(p);
		if (open.delete(p)) return;
		open.add(p);
		fetchKids(p);
	};

	let quota = $state<QuotaInfo | null>(null);
	const refresh = () => {
		for (const p of ['/', ...open]) fetchKids(p);
		Quota().then((q) => (quota = q));
	};
	refresh();
	$effect(() => {
		if (indexStatus.current?.done) untrack(refresh);
	});
	$effect(() => {
		if (changes.n) untrack(refresh);
	});

	const route = $derived(page.route.id ?? '');
	const inFiles = $derived(route.startsWith('/(app)/files'));
	const current = $derived(
		inFiles ? '/' + ((page.params as { path?: string }).path ?? '').replace(/\/+$/, '') : ''
	);
	const stars = $derived(starred(server.id));

	// The open folder's ancestors stay expanded; branches opened only for that close again once you leave them.
	const auto = new SvelteSet<string>();
	$effect(() => {
		const parts = current.split('/').filter(Boolean);
		const path = parts.slice(1).map((_, i) => '/' + parts.slice(0, i + 1).join('/'));
		untrack(() => {
			for (const p of auto)
				if (!path.includes(p)) {
					open.delete(p);
					auto.delete(p);
				}
			for (const p of path)
				if (!open.has(p)) {
					toggle(p);
					auto.add(p);
				}
		});
	});
	const active = $derived(transfers.list.filter(live).length);
	const used = $derived(quota && quota.used >= 0 ? quota.used : 0);
	const total = $derived(quota && quota.available >= 0 ? used + quota.available : 0);

	let menu = $state(false);

	const toggleSidebar = () => setPrefs({ sidebarHidden: !prefs.sidebarHidden });
	const keys = (e: KeyboardEvent) => {
		if (e.key === 'Escape') menu = false;
		// Finder's ⌃⌘S.
		else if (e.code === 'KeyS' && e.ctrlKey && (mac ? e.metaKey : e.shiftKey)) toggleSidebar();
	};
	const leave = async (fn: () => Promise<void>) => {
		await fn();
		await goto('/');
	};

	$effect(() => {
		if (desktop && prefs.drive[server.id]) Mount().catch(() => {});
	});

	let retryIn = $state(0);
	let delay = 2;
	let timer: ReturnType<typeof setTimeout>;
	let back: ReturnType<typeof setTimeout>;
	const countdown = () => {
		timer = setTimeout(() => (--retryIn > 0 ? countdown() : probe()), 1000);
	};
	function probe() {
		clearTimeout(timer);
		Ping().then(
			() => {
				net.offline = false;
				net.back = true;
				delay = 2;
				back = setTimeout(() => (net.back = false), 3000);
			},
			() => {
				delay = Math.min(delay * 2, 30);
				retryIn = delay;
				countdown();
			}
		);
	}
	$effect(() => {
		if (!net.offline) return;
		retryIn = delay;
		countdown();
		return () => clearTimeout(timer);
	});
	$effect(() => () => {
		clearTimeout(back);
		net.back = false;
	});
</script>

<svelte:window
	ononline={() => net.offline && probe()}
	onclick={() => (menu = false)}
	onkeydown={keys}
/>

{#snippet heading(text: string)}
	<div class="px-2.5 pt-5 pb-1.5 text-[11px] font-medium tracking-[0.06em] text-fg-3 uppercase">
		{text}
	</div>
{/snippet}

{#snippet node(f: Entry, depth: number)}
	<div
		class="nav-item h-7.5 gap-1.5 pr-2"
		class:active={current === f.path}
		style="padding-left: {depth * 14 + 4}px"
		data-drop={f.path}
		{@attach (el) => {
			if (current === f.path) el.scrollIntoView({ block: 'nearest' });
		}}
	>
		<button
			class="shrink-0 text-fg-3 hover:text-fg"
			onclick={() => toggle(f.path)}
			aria-label={open.has(f.path) ? 'Collapse' : 'Expand'}
		>
			<Icon
				name="chevronRight"
				size={12}
				class="motion-safe:transition-transform {open.has(f.path) ? 'rotate-90' : ''}"
			/>
		</button>
		<a href={filesHref(f.path)} class="flex min-w-0 flex-1 items-center gap-2" title={f.name}>
			<Icon name="folderFill" /><span class="truncate">{f.name}</span>
		</a>
	</div>
	{#if open.has(f.path)}
		{#each (kids[f.path] ?? []).filter(listed) as c (c.path)}{@render node(c, depth + 1)}{/each}
	{/if}
{/snippet}

<Split
	class="h-screen"
	bind:hidden={() => prefs.sidebarHidden, (v) => setPrefs({ sidebarHidden: v })}
	bind:width={() => prefs.sidebarWidth, (v) => setPrefs({ sidebarWidth: v })}
	min={180}
	max={360}
>
	{#snippet pane()}
		<aside class="flex h-full flex-col bg-bg">
			<div class="titlebar flex h-13 shrink-0 items-center justify-end pr-2">
				<button
					class="btn btn-ghost h-7 w-7 px-0"
					title="Hide sidebar ({mac ? '⌃⌘S' : 'Ctrl+Shift+S'})"
					onclick={toggleSidebar}
				>
					<Icon name="sidebar" size={17} />
				</button>
			</div>
			<nav class="flex flex-col gap-0.5 px-3">
				<a
					href={inFiles ? '/files' : home(server)}
					class="nav-item"
					class:active={inFiles}
					data-drop="/"><Icon name="folder" />Files</a
				>
				<a href="/recent" class="nav-item" class:active={route === '/(app)/recent'}
					><Icon name="clock" />Recent</a
				>
				<a href="/transfers" class="nav-item" class:active={route === '/(app)/transfers'}>
					<Icon name="transfers" />Transfers
					{#if active}
						<span
							class="ml-auto rounded-full bg-accent-soft px-1.5 font-mono text-[11px] text-accent-fg"
							>{active}</span
						>
					{/if}
				</a>
				<a href="/trash" class="nav-item" class:active={route === '/(app)/trash'}
					><Icon name="trash" />Trash</a
				>
			</nav>
			<div class="flex min-h-0 flex-col gap-px overflow-y-auto px-3">
				{#if stars.length}
					{@render heading('Starred')}
					{#each stars as s (s.path)}
						<a
							href={s.dir ? filesHref(s.path) : focusHref(s.path) + '&open'}
							class="nav-item h-7.5"
							title={s.path}
							class:active={s.dir && current === s.path}
							data-drop={s.dir ? s.path : undefined}
						>
							<FileIcon name={base(s.path)} dir={s.dir} /><span class="truncate"
								>{base(s.path)}</span
							>
						</a>
					{/each}
				{/if}
				{#if kids['/']?.length}
					{@render heading('Folders')}
					{#each kids['/'].filter(listed) as f (f.path)}{@render node(f, 0)}{/each}
				{/if}
			</div>
			<div class="flex-1"></div>
			{#if update.s.state === 'ready'}
				<button
					class="mx-3 mb-1.5 flex items-center gap-2 rounded-md border border-line bg-surface px-2.5 py-2 text-left text-xs"
					onclick={() => (update.open = true)}
				>
					<span class="h-1.75 w-1.75 rounded-full bg-ok"></span>
					<span class="flex-1 truncate text-fg-2">Soteria {update.s.version} is ready</span>
					<span class="font-medium text-accent-fg">Restart</span>
				</button>
			{/if}
			{#if total}
				{@const low = used / total >= 0.9}
				<div class="flex flex-col gap-2 border-t border-line px-4 pt-3 pb-2 text-xs">
					<div class="flex justify-between">
						<span class="text-fg-2">Storage</span>
						<span class="font-mono {low ? 'text-warn' : 'text-fg-3'}"
							>{low ? `${bytes(total - used)} left` : `${bytes(used)} of ${bytes(total)}`}</span
						>
					</div>
					<progress class="bar h-1.5" class:warn={low} value={used} max={total}></progress>
				</div>
			{/if}
			<div class="relative px-3 pb-3">
				{#if menu}
					<div class="menu pop absolute right-3 bottom-full left-3 z-20 mb-1.5 w-auto" role="menu">
						<div class="flex items-center gap-2.5 px-2.5 pt-2 pb-2.5">
							<span
								class="flex h-8.5 w-8.5 shrink-0 items-center justify-center rounded-full bg-accent-soft text-sm font-semibold text-accent-fg"
								>{server.username.charAt(0).toUpperCase()}</span
							>
							<span class="flex min-w-0 flex-col">
								<span class="truncate font-semibold">{server.username}</span>
								<span class="truncate font-mono text-[11px] text-fg-3">
									{server.name} · {server.url.replace(/^https?:\/\//, '')}
								</span>
							</span>
						</div>
						<hr class="my-1 border-line" />
						<a class="menu-item" href="/settings"
							><Icon name="gear" size={15} class="text-fg-2" />Settings</a
						>
						<button class="menu-item" onclick={() => leave(Disconnect)}>
							<Icon name="server" size={15} class="text-fg-2" />Switch server…
						</button>
						<hr class="my-1 border-line" />
						<button class="menu-item" onclick={() => leave(SignOut)}>
							<Icon name="signOut" size={15} class="text-fg-2" />Sign out
						</button>
					</div>
				{/if}
				<button
					class="flex w-full items-center gap-2.5 rounded-lg border px-1.5 py-1.5 text-left {menu
						? 'border-line bg-surface'
						: 'border-transparent hover:bg-surface'}"
					onclick={(e) => {
						e.stopPropagation();
						menu = !menu;
					}}
				>
					<span
						class="flex h-6.5 w-6.5 items-center justify-center rounded-full bg-accent-soft text-xs font-semibold text-accent-fg"
						>{server.username.charAt(0).toUpperCase()}</span
					>
					<span class="flex min-w-0 flex-1 flex-col">
						<span class="truncate font-medium">{server.username}</span>
						<span class="truncate text-[11px] text-fg-3">{server.name}</span>
					</span>
					<Icon name="chevronDown" size={14} class="text-fg-3 {menu ? 'rotate-180' : ''}" />
				</button>
			</div>
		</aside>
	{/snippet}
	<main
		class="relative flex h-full min-w-0 flex-col bg-surface"
		style="--lead: {mac ? '76px' : '12px'}"
	>
		<!-- The upload panel floats over the page, above the offline banner rather than on it. -->
		<div
			class="relative flex min-h-0 flex-1 flex-col"
			data-sidebar={prefs.sidebarHidden ? 'hidden' : undefined}
		>
			{@render children()}
			{#if route !== '/(app)/transfers'}<UploadPanel />{/if}
		</div>
		{#if prefs.sidebarHidden}
			<button
				class="btn btn-ghost absolute top-3 z-10 h-7 w-7 px-0"
				style="left: var(--lead)"
				title="Show sidebar ({mac ? '⌃⌘S' : 'Ctrl+Shift+S'})"
				onclick={toggleSidebar}
			>
				<Icon name="sidebarOff" size={17} />
				{#if active || net.offline}
					<span
						class="absolute -top-0.5 -right-0.5 h-1.75 w-1.75 rounded-full border border-surface {net.offline
							? 'bg-warn'
							: 'bg-accent'}"
					></span>
				{/if}
			</button>
		{/if}
		{#if net.offline}
			<div
				class="flex shrink-0 items-center gap-2.5 border-t border-line bg-warn-soft px-6 py-2 text-xs"
			>
				<Icon name="info" size={15} class="text-warn" />
				<span class="flex-1">
					<b class="font-semibold">Can't reach {server.name}.</b> Showing what was loaded last.
					Retrying in {retryIn} s…
				</span>
				<button class="font-medium text-accent-fg hover:underline" onclick={probe}>Retry now</button
				>
			</div>
		{:else if net.back}
			<div
				class="flex shrink-0 items-center gap-2.5 border-t border-line bg-ok-soft px-6 py-2 text-xs"
			>
				<Icon name="check" size={15} class="text-ok" />Back online.
			</div>
		{/if}
		{#if desktop && !prefs.intro[server.id]}
			<DriveIntro {server} />
		{/if}
	</main>
</Split>
