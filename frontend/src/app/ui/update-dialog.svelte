<script lang="ts">
	import { InstallUpdate, RestartToUpdate, check, transfers, update } from '@/shared/api';
	import { bytes, prefs, setPrefs, when } from '@/shared/lib';
	import { Dialog, Icon } from '@/shared/ui';
	import { Browser, System } from '@wailsio/runtime';

	const s = $derived(update.s);
	const running = $derived(
		transfers.list.filter((t) => t.status === 'running' || t.status === 'queued').length
	);
	const busy = $derived(['downloading', 'verifying', 'installing'].includes(s.state));
	const mode = $derived(s.state === 'available' && s.blocked ? 'blocked' : s.state);

	// Show the dialog once per new version; Skip remembers the version, Later lasts until the next launch.
	let seen = '';
	$effect(() => {
		if (s.state === 'available' && s.version !== prefs.skipVersion && s.version !== seen) {
			seen = s.version;
			update.open = true;
		}
	});

	const close = () => (update.open = false);
	const skip = () => {
		setPrefs({ skipVersion: s.version });
		close();
	};
	const site = () => Browser.OpenURL(s.url);

	const title = $derived(
		(
			{
				available: `Soteria ${s.version} is available`,
				blocked: 'Can’t update automatically here',
				ready: `Ready to update to ${s.version}`,
				error: `Couldn’t update to ${s.version}`
			} as Record<string, string>
		)[mode] ?? `Downloading Soteria ${s.version}`
	);
	const sub = $derived(
		(
			{
				available: `You have ${s.current} · ${bytes(s.size)} · Released ${when(s.date).split(',')[0]}`,
				blocked: `Soteria ${s.version} is available`,
				ready: 'Downloaded and verified',
				error: 'Download failed'
			} as Record<string, string>
		)[mode] ?? 'You can keep working. Nothing changes until you restart.'
	);
	// GitHub's generated notes: keep headings and bullets, drop links, bold marks and the changelog line.
	const notes = $derived(
		s.notes
			.split('\n')
			.map((l) =>
				l
					.replace(/\s+in\s+https?:\S+$/, '')
					.replace(/\*\*|\[([^\]]+)\]\([^)]*\)|https?:\S+/g, '$1')
					.trim()
			)
			.filter((l) => l && !/^full changelog/i.test(l))
			.map((l) => ({ h: l.startsWith('#'), text: l.replace(/^[#*-]+\s*/, '') }))
	);
	const p = $derived(update.progress);
	const pct = $derived(
		p && p.total ? (p.written / p.total) * 100 : s.state === 'downloading' ? 0 : 100
	);
	const left = $derived(
		p && p.rate ? `${bytes(p.rate)}/s · ${Math.ceil((p.total - p.written) / p.rate)} s left` : ''
	);
	const badge = $derived(
		({ ready: 'bg-ok', error: 'bg-danger', blocked: 'bg-warn' } as Record<string, string>)[mode]
	);
</script>

{#snippet bo()}
	<span
		class="relative flex h-11 w-11 shrink-0 items-center justify-center rounded-[11px] bg-surface-2"
	>
		<img src="/bo.svg" alt="" class="h-8 w-8" />
		{#if badge}
			<span
				class="absolute -right-1 -bottom-1 flex h-4.5 w-4.5 items-center justify-center rounded-full border-2 border-surface {badge} text-[#fff]"
			>
				<Icon name={mode === 'ready' ? 'check' : 'info'} size={10} />
			</span>
		{/if}
	</span>
{/snippet}

<Dialog open={update.open} {title} {sub} lead={bo} onclose={close}>
	<div class="flex w-100 flex-col gap-4">
		{#if mode === 'available' || busy}
			{#if notes.length}
				<div
					class="flex flex-col gap-1 rounded-lg border border-line bg-bg px-3.5 py-3 text-[12.5px] leading-relaxed text-fg-2"
				>
					{#each notes as n, i (i)}
						{#if n.h}
							<div class="pt-1 text-xs font-semibold text-fg first:pt-0">{n.text}</div>
						{:else}
							<div class="pl-4 -indent-4">• {n.text}</div>
						{/if}
					{/each}
					<a
						class="mt-1 inline-flex items-center gap-1 text-xs text-accent-fg"
						href={s.url}
						onclick={(e) => {
							e.preventDefault();
							site();
						}}
					>
						Full release notes <Icon name="open" size={12} />
					</a>
				</div>
			{/if}
			{#if busy}
				<div class="flex flex-col gap-2">
					<div class="h-1.5 overflow-hidden rounded-full bg-surface-2">
						<div class="h-full rounded-full bg-accent" style="width:{pct}%"></div>
					</div>
					<div class="flex justify-between text-xs">
						<span class="text-fg-2">
							{#if s.state === 'downloading' && p}Downloading · {bytes(p.written)} of {bytes(
									p.total
								)}
							{:else if s.state === 'verifying'}Verifying…{:else}Preparing…{/if}
						</span>
						<span class="font-mono text-fg-3">{left}</span>
					</div>
				</div>
			{/if}
		{:else if mode === 'ready'}
			<p class="text-fg-2">Soteria quits and reopens as {s.version}. It takes a few seconds.</p>
			{#if running}
				<div
					class="flex gap-2.5 rounded-md bg-warn-soft px-3 py-2.5 text-xs leading-relaxed text-warn"
				>
					<Icon name="info" size={15} class="mt-0.5 shrink-0" />
					<span
						>{running} transfers are still running. They stop when Soteria quits and can be retried from
						Transfers.</span
					>
				</div>
			{/if}
		{:else if mode === 'error'}
			<div
				class="flex gap-2.5 rounded-md bg-danger-soft px-3 py-2.5 text-xs leading-relaxed text-danger"
			>
				<Icon name="info" size={15} class="mt-0.5 shrink-0" />
				<span>{update.error} Nothing on this {System.IsWindows() ? 'PC' : 'Mac'} was changed.</span>
			</div>
		{:else if mode === 'blocked'}
			<p class="text-fg-2">
				{System.IsWindows()
					? 'Soteria is installed in Program Files, which needs administrator rights to change. Download the new installer instead; it installs to your user folder and updates in place from then on.'
					: 'Soteria is running from the disk image, so it can’t replace itself. Drag it to Applications, open it from there and check again.'}
			</p>
		{/if}
		<div class="flex items-center justify-end gap-2">
			{#if mode === 'available'}
				<button class="btn btn-ghost mr-auto" onclick={skip}>Skip this version</button>
				<button class="btn" onclick={close}>Later</button>
				<button class="btn btn-primary" onclick={InstallUpdate}>Update now</button>
			{:else if busy}
				<button class="btn" onclick={close}>Hide</button>
			{:else if mode === 'ready'}
				<button class="btn" onclick={close}>Later</button>
				<button class="btn btn-primary" onclick={RestartToUpdate}>Restart &amp; update</button>
			{:else if mode === 'error'}
				<button class="btn" onclick={site}
					><Icon name="open" size={15} />Download from GitHub</button
				>
				<button class="btn btn-primary" onclick={s.version ? InstallUpdate : check}
					>Try again</button
				>
			{:else}
				<button class="btn" onclick={close}>Later</button>
				<button class="btn btn-primary" onclick={site}
					><Icon name="open" size={15} />Download {s.version}</button
				>
			{/if}
		</div>
	</div>
</Dialog>
