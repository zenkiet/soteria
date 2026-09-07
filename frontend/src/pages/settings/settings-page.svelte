<script lang="ts">
	import { goto } from '$app/navigation';
	import {
		Drive,
		LogPath,
		SignOut,
		connectDrive,
		Open,
		PickFolder,
		Quota,
		RestartToUpdate,
		Unmount,
		check,
		update,
		type DriveInfo,
		type QuotaInfo,
		type Server
	} from '@/shared/api';
	import { ago, bytes, msg, prefs, setPrefs, setTheme, theme, type Theme } from '@/shared/lib';
	import { Icon } from '@/shared/ui';
	import { Browser, System } from '@wailsio/runtime';

	let { server }: { server: Server } = $props();

	let quota = $state<QuotaInfo | null>(null);
	Quota().then((q) => (quota = q));

	const themes: Theme[] = ['system', 'light', 'dark'];

	async function signOut() {
		await SignOut();
		await goto('/');
	}

	async function pickDir() {
		const d = await PickFolder();
		if (d) setPrefs({ downloadDir: d });
	}

	let logPath = $state('');
	LogPath().then((p) => (logPath = p));
	const desktop = System.IsMac() || System.IsWindows();
	const where = System.IsWindows() ? 'File Explorer' : 'Finder';
	const bar = System.IsWindows() ? 'system tray' : 'menu bar';
	let info = $state<DriveInfo | null>(null);
	let busy = $state(false);
	let driveError = $state('');
	const refreshDrive = () => Drive().then((d) => (info = d));
	if (desktop) refreshDrive();

	async function toggleDrive(on: boolean) {
		busy = true;
		driveError = '';
		try {
			await (on ? connectDrive() : Unmount());
		} catch (e) {
			driveError = msg(e);
		}
		await refreshDrive();
		busy = false;
	}

	const status = $derived(
		driveError
			? { text: driveError, cls: 'text-danger', dot: 'bg-danger' }
			: busy
				? { text: 'Connecting…', cls: 'text-fg-2', dot: 'bg-warn' }
				: info?.mounted
					? { text: `Connected · ${info.path}`, cls: 'text-fg-2', dot: 'bg-ok' }
					: { text: 'Not connected', cls: 'text-fg-3', dot: 'bg-line-2' }
	);

	const upd = $derived.by(() => {
		const s = update.s;
		switch (s.state) {
			case 'unconfigured':
				return { dot: 'bg-line-2', cls: 'text-fg-3', text: 'Development build · updates are off' };
			case 'checking':
				return { dot: 'bg-warn', cls: 'text-fg-2', text: 'Checking…' };
			case 'available':
				return {
					dot: 'bg-accent',
					cls: 'text-fg-2',
					text: `${s.version} available · ${bytes(s.size)}`,
					btn: s.blocked ? 'Download…' : 'Update…',
					primary: true,
					run: s.blocked ? () => Browser.OpenURL(s.url) : () => (update.open = true)
				};
			case 'downloading':
			case 'verifying':
			case 'installing':
				return { dot: 'bg-accent', cls: 'text-fg-2', text: 'Downloading…' };
			case 'ready':
				return {
					dot: 'bg-ok',
					cls: 'text-fg-2',
					text: `${s.version} downloaded · restart to finish`,
					btn: 'Restart & update',
					primary: true,
					run: RestartToUpdate
				};
			case 'error':
				return {
					dot: 'bg-danger',
					cls: 'text-danger',
					text: `Couldn’t check: ${update.error}`,
					btn: 'Try again',
					run: check
				};
			default:
				return {
					dot: 'bg-ok',
					cls: 'text-fg-2',
					text: update.checkedAt
						? `Up to date · checked ${ago(update.checkedAt / 1000)}`
						: 'Not checked yet',
					btn: 'Check for updates',
					run: check
				};
		}
	});
</script>

{#snippet updates()}
	<div class="flex items-center gap-3 rounded-lg border border-line p-3">
		<span class="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-surface-2">
			<img src="/bo.svg" alt="" class="h-6.5 w-6.5" />
		</span>
		<div class="flex min-w-0 flex-1 flex-col gap-0.5">
			<div class="font-medium">Soteria {__APP_VERSION__}</div>
			<div class="flex items-center gap-1.5 text-xs {upd.cls}">
				<span class="h-1.5 w-1.5 shrink-0 rounded-full {upd.dot}"></span>
				<span class="truncate" title={upd.text}>{upd.text}</span>
			</div>
		</div>
		{#if upd.btn}
			<button class="btn {upd.primary ? 'btn-primary' : ''}" onclick={upd.run}>
				{#if !upd.primary}<Icon name="refresh" size={15} />{/if}{upd.btn}
			</button>
		{/if}
	</div>
	<label class="flex items-center justify-between gap-6">
		<span class="flex flex-col gap-0.5">
			<span>Check for updates automatically</span>
			<span class="text-xs text-fg-3"
				>At launch and every 6 hours. Nothing installs without asking you.</span
			>
		</span>
		<input
			type="checkbox"
			class="switch"
			checked={prefs.autoUpdate}
			onchange={(e) => setPrefs({ autoUpdate: e.currentTarget.checked })}
		/>
	</label>
{/snippet}

{#snippet drive()}
	<label class="flex items-center justify-between gap-6">
		<span class="flex flex-col gap-0.5">
			<span>Show Soteria in {where}</span>
			<span class="flex items-center gap-1.5 text-xs {status.cls}">
				<span class="h-1.5 w-1.5 rounded-full {status.dot}"></span>{status.text}
			</span>
		</span>
		<input
			type="checkbox"
			class="switch"
			checked={info?.mounted ?? false}
			disabled={busy}
			onchange={(e) => toggleDrive(e.currentTarget.checked)}
		/>
	</label>
	<label class="flex items-center justify-between gap-6">
		<span class="flex flex-col gap-0.5">
			<span>Connect automatically</span>
			<span class="text-xs text-fg-3">Mount the drive each time you sign in to this server.</span>
		</span>
		<input
			type="checkbox"
			class="switch"
			checked={prefs.drive[server.id] ?? false}
			onchange={(e) =>
				setPrefs({ drive: { ...prefs.drive, [server.id]: e.currentTarget.checked } })}
		/>
	</label>
	<div class="flex gap-2">
		{#if info?.mounted}
			{@const path = info.path}
			<button class="btn" onclick={() => Open(path)}
				><Icon name="open" size={15} />Show in {where}</button
			>
		{/if}
		{#if driveError}
			<button class="btn" onclick={() => toggleDrive(true)}
				><Icon name="refresh" size={15} />Try again</button
			>
		{/if}
		<button class="btn" onclick={() => setPrefs({ intro: { ...prefs.intro, [server.id]: false } })}>
			What is this?
		</button>
	</div>
{/snippet}

{#snippet downloads()}
	<div class="flex items-center justify-between gap-6">
		<div class="min-w-0">
			<div>Save to</div>
			<div class="truncate font-mono text-xs text-fg-3">{prefs.downloadDir || '~/Downloads'}</div>
		</div>
		<button class="btn" onclick={pickDir}>Change…</button>
	</div>
	<label class="flex items-center justify-between gap-6">
		<span>Ask where to save each time</span>
		<input
			type="checkbox"
			class="accent-primary"
			checked={prefs.askDownloadDir}
			onchange={(e) => setPrefs({ askDownloadDir: e.currentTarget.checked })}
		/>
	</label>
{/snippet}

{#snippet diagnostics()}
	<div class="flex items-center justify-between gap-6">
		<div class="min-w-0">
			<div>Log file</div>
			<div class="truncate font-mono text-xs text-fg-3">{logPath}</div>
		</div>
		<button class="btn" onclick={() => Open(logPath.replace(/[\\/][^\\/]*$/, ''))}>
			<Icon name="open" size={15} />Open folder
		</button>
	</div>
{/snippet}

{#snippet trash()}
	<div class="flex items-center justify-between gap-6">
		<div>
			<div>Deleted items</div>
			<div class="text-xs text-fg-3">
				Kept in Trash on the server for 30 days, then removed for good.
			</div>
		</div>
		<a class="btn" href="/trash"><Icon name="trash" size={15} />Open Trash</a>
	</div>
{/snippet}

{#snippet background()}
	<label class="flex items-center justify-between gap-6">
		<span class="flex flex-col gap-0.5">
			<span>Keep running in the background</span>
			<span class="text-xs text-fg-3"
				>Shows an icon in the {bar} so the network drive stays connected.</span
			>
		</span>
		<input
			type="checkbox"
			class="switch"
			checked={prefs.background}
			onchange={(e) => setPrefs({ background: e.currentTarget.checked })}
		/>
	</label>
	<label class="flex items-center justify-between gap-6">
		<span class="flex flex-col gap-0.5">
			<span>Notify when transfers finish</span>
			<span class="text-xs text-fg-3">Only while Soteria is in the background.</span>
		</span>
		<input
			type="checkbox"
			class="switch"
			checked={prefs.notify}
			onchange={(e) => setPrefs({ notify: e.currentTarget.checked })}
		/>
	</label>
{/snippet}

{#snippet section(title: string, sub: string, body: import('svelte').Snippet)}
	<section class="grid grid-cols-[250px_minmax(0,1fr)] gap-8 py-6">
		<div>
			<div class="font-semibold">{title}</div>
			<div class="mt-1 text-xs text-fg-3">{sub}</div>
		</div>
		<div class="flex flex-col gap-3.5">{@render body()}</div>
	</section>
{/snippet}

{#snippet account()}
	<div class="flex items-center gap-3 rounded-lg border border-line p-3">
		<span
			class="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-surface-2 text-fg-2"
		>
			<Icon name="server" />
		</span>
		<div class="flex min-w-0 flex-1 flex-col gap-0.5">
			<div class="font-medium">{server.name}</div>
			<div class="truncate font-mono text-xs text-fg-3">
				{server.url} · signed in as {server.username}
			</div>
		</div>
		<button class="btn" onclick={signOut}><Icon name="signOut" size={15} />Sign out</button>
	</div>
{/snippet}

{#snippet appearance()}
	<div class="flex items-center justify-between">
		<span>Theme</span>
		<div class="inline-flex gap-0.5 rounded-md bg-surface-2 p-0.5">
			{#each themes as t (t)}
				<button
					class="rounded px-3 py-1.25 text-xs font-medium capitalize {theme.value === t
						? 'bg-surface text-fg shadow-[0_1px_2px_rgba(0,0,0,0.08)]'
						: 'text-fg-2'}"
					onclick={() => setTheme(t)}>{t}</button
				>
			{/each}
		</div>
	</div>
{/snippet}

{#snippet storage()}
	{#if quota && quota.used >= 0 && quota.available >= 0}
		{@const total = quota.used + quota.available}
		<div class="flex justify-between text-xs">
			<span class="text-fg-2">{bytes(quota.used)} of {bytes(total)} used</span>
			<span class="font-mono text-fg-3">{bytes(quota.available)} free</span>
		</div>
		<div class="h-2 overflow-hidden rounded-full bg-surface-2">
			<div class="h-full rounded-full bg-accent" style="width:{(quota.used / total) * 100}%"></div>
		</div>
	{:else if quota && quota.used >= 0}
		<p class="text-xs text-fg-2">{bytes(quota.used)} used · no limit set</p>
	{:else}
		<p class="text-xs text-fg-3">This server doesn't report a quota.</p>
	{/if}
{/snippet}

<header
	class="flex h-13 shrink-0 items-center border-b border-line px-5"
	style="--wails-draggable: drag"
>
	<h1 class="text-[15px] font-semibold tracking-tight">Settings</h1>
</header>

<div class="flex min-h-0 flex-1 flex-col overflow-y-auto px-6 pb-5">
	<div class="max-w-full divide-y divide-line">
		{@render section('Account', 'The server this window is signed in to.', account)}
		{#if desktop}
			{@render section(
				'Network drive',
				`Shows this server in ${where} as a drive named Soteria, using the same login.`,
				drive
			)}
		{/if}
		{#if desktop}
			{@render section('Background', 'What happens when you close the window.', background)}
			{@render section(
				'Updates',
				'New versions come from GitHub Releases and install in place.',
				updates
			)}
		{/if}
		{@render section('Appearance', 'Follows macOS by default.', appearance)}
		{@render section('Downloads', 'Where downloaded files land.', downloads)}
		{@render section(
			'Trash',
			'Deleting moves items to a hidden .trash folder on the server.',
			trash
		)}
		{@render section('Storage', `Quota is set on the server for ${server.username}.`, storage)}
		{@render section('Diagnostics', 'For when something goes wrong.', diagnostics)}
	</div>
	<p class="mt-auto pt-6 text-center font-mono text-[11px] text-fg-3">
		Soteria {__APP_VERSION__} - Powered by ZenSoftware
	</p>
</div>
