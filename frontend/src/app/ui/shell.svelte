<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import type { Snippet } from 'svelte';
	import {
		Disconnect,
		List,
		Mount,
		Ping,
		Quota,
		SignOut,
		transfers,
		type Entry,
		type QuotaInfo,
		type Server
	} from '@/shared/api';
	import { bytes, filesHref, net, pop, prefs, setPrefs } from '@/shared/lib';
	import { Icon } from '@/shared/ui';
	import { Events, System } from '@wailsio/runtime';
	import DriveIntro from './drive-intro.svelte';
	import UploadPanel from './upload-panel.svelte';

	let { server, children }: { server: Server; children: Snippet } = $props();

	let folders = $state<Entry[]>([]);
	let quota = $state<QuotaInfo | null>(null);
	List('/').then((l) => (folders = (l ?? []).filter((e) => e.dir)));
	const refreshQuota = () => Quota().then((q) => (quota = q));
	refreshQuota();
	$effect(() => Events.On('index', (ev) => ev.data.done && refreshQuota()));

	const route = $derived(page.route.id ?? '');
	const inFiles = $derived(route.startsWith('/(app)/files'));
	const top = $derived(((page.params as { path?: string }).path ?? '').split('/')[0]);
	const active = $derived(
		transfers.list.filter((t) => t.status === 'running' || t.status === 'queued').length
	);
	const used = $derived(quota && quota.used >= 0 ? quota.used : 0);
	const total = $derived(quota && quota.available >= 0 ? used + quota.available : 0);

	let menu = $state(false);

	// Collapsible sidebar: instant, no animation.
	let hidden = $state(prefs.sidebarHidden);
	function toggleSidebar() {
		hidden = !hidden;
		setPrefs({ sidebarHidden: hidden });
	}
	const leave = async (fn: () => Promise<void>) => {
		await fn();
		await goto('/');
	};

	const mac = System.IsMac();
	const desktop = mac || System.IsWindows();
	$effect(() => {
		if (desktop && prefs.drive[server.id]) Mount().catch(() => {});
	});

	let retryIn = $state(0);
	let delay = 2;
	let timer: ReturnType<typeof setTimeout>;
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
				setTimeout(() => (net.back = false), 3000);
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
</script>

<svelte:window ononline={() => net.offline && probe()} onclick={() => (menu = false)} />

<div class="flex h-screen">
	<aside class="sidebar flex w-60 shrink-0 flex-col border-r border-line bg-bg" class:gone={hidden}>
		<div class="flex h-13 shrink-0 items-center justify-end pr-2" style="--wails-draggable: drag">
			<button class="btn btn-ghost h-7 w-7 px-0" title="Hide sidebar" onclick={toggleSidebar}>
				<Icon name="sidebar" size={17} />
			</button>
		</div>
		<nav class="flex flex-col gap-0.5 px-3">
			<a href="/files" class="nav-item" class:active={inFiles && !top}
				><Icon name="folder" />Files</a
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
		{#if folders.length}
			<div class="px-5.5 pt-5 pb-1.5 text-[11px] font-medium tracking-[0.06em] text-fg-3 uppercase">
				Folders
			</div>
			<div class="flex flex-col gap-px overflow-y-auto px-3">
				{#each folders as f (f.path)}
					<a
						href={filesHref(f.path)}
						class="nav-item h-7.5"
						class:active={inFiles && top === f.name}
					>
						<Icon name="folderFill" /><span class="truncate">{f.name}</span>
					</a>
				{/each}
			</div>
		{/if}
		<div class="flex-1"></div>
		{#if total}
			{@const low = used / total >= 0.9}
			<div class="flex flex-col gap-2 border-t border-line px-4 pt-3 pb-2 text-xs">
				<div class="flex justify-between">
					<span class="text-fg-2">Storage</span>
					<span class="font-mono {low ? 'text-warn' : 'text-fg-3'}"
						>{low ? `${bytes(total - used)} left` : `${bytes(used)} / ${bytes(total)}`}</span
					>
				</div>
				<div class="h-1.5 overflow-hidden rounded-full bg-surface-2">
					<div
						class="h-full rounded-full {low ? 'bg-warn' : 'bg-accent'}"
						style="width:{(used / total) * 100}%"
					></div>
				</div>
			</div>
		{/if}
		<div class="relative px-3 pb-3">
			{#if menu}
				<div class="menu absolute bottom-full left-3 z-20 mb-1.5 w-67" use:pop role="menu">
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
	<main
		class="relative flex min-w-0 flex-1 flex-col bg-surface"
		data-sidebar={hidden ? 'hidden' : undefined}
		style="--lead: {desktop ? '76px' : '12px'}"
	>
		{@render children()}
		{#if hidden}
			<button
				class="btn btn-ghost absolute top-3 z-10 h-7 w-7 px-0"
				style="left: var(--lead)"
				title="Show sidebar"
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
				class="absolute inset-x-0 top-13 z-10 flex items-center gap-2.5 border-b border-line bg-warn-soft px-6 py-2 text-xs"
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
				class="absolute inset-x-0 top-13 z-10 flex items-center gap-2.5 border-b border-line bg-ok-soft px-6 py-2 text-xs"
			>
				<Icon name="check" size={15} class="text-ok" />Back online.
			</div>
		{/if}
		{#if route !== '/(app)/transfers'}<UploadPanel />{/if}
		{#if desktop && !prefs.intro[server.id]}
			<DriveIntro {server} folders={folders.map((f) => f.name)} />
		{/if}
	</main>
</div>
