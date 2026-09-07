<script lang="ts">
	import { connectDrive, Open, type Server } from '@/shared/api';
	import { msg, pop, prefs, setPrefs, toast } from '@/shared/lib';
	import { Icon } from '@/shared/ui';
	import { System } from '@wailsio/runtime';

	let { server, folders }: { server: Server; folders: string[] } = $props();
	let busy = $state(false);
	const win = System.IsWindows();
	const where = win ? 'File Explorer' : 'Finder';
	const pc = win ? 'PC' : 'Mac';

	const done = (auto: boolean) =>
		setPrefs({
			intro: { ...prefs.intro, [server.id]: true },
			drive: { ...prefs.drive, [server.id]: auto }
		});

	async function connect() {
		busy = true;
		try {
			const d = await connectDrive();
			done(true);
			toast(
				win ? 'Soteria is now a drive in File Explorer.' : 'Soteria is now in your Finder sidebar.',
				'ok',
				{
					label: 'Show',
					run: () => Open(d.path)
				}
			);
		} catch (e) {
			toast(msg(e), 'error');
		}
		busy = false;
	}

	const sample = ['Backups', 'Product Photos', 'Reports', 'Receipts', 'Staff Docs'];
	const shown = $derived(folders.length ? folders.slice(0, 5) : sample);
</script>

{#snippet benefit(ic: 'open' | 'folderMove' | 'refresh', title: string, text: string)}
	<div class="flex items-start gap-3">
		<span
			class="flex h-7.5 w-7.5 shrink-0 items-center justify-center rounded-lg bg-surface-2 text-fg-2"
		>
			<Icon name={ic} size={15} />
		</span>
		<div>
			<div class="font-semibold">{title}</div>
			<div class="text-xs text-fg-2">{text}</div>
		</div>
	</div>
{/snippet}

<div class="fixed inset-0 z-30 flex items-center justify-center bg-black/45">
	<div
		use:pop
		class="w-140 overflow-hidden rounded-2xl border border-line bg-surface shadow-[0_30px_80px_rgba(0,0,0,0.35)]"
	>
		<div
			class="relative h-62.5 overflow-hidden bg-[linear-gradient(135deg,var(--accent-soft),var(--bg)_55%,var(--surface-2))]"
		>
			<div
				class="absolute -top-30 left-15 h-90 w-90 rounded-full bg-accent opacity-15 blur-3xl"
			></div>
			<div
				class="absolute -right-10 -bottom-30 h-65 w-65 rounded-full bg-warn opacity-10 blur-3xl"
			></div>
			<div
				class="absolute -bottom-10 -left-7.5 flex h-57.5 w-105 -rotate-[7deg] overflow-hidden rounded-xl border border-line bg-surface shadow-[0_30px_60px_rgba(27,26,23,0.18)]"
			>
				<div class="flex w-37.5 flex-col gap-1.5 border-r border-line bg-bg px-2.5 py-3.5">
					{#if !win}
						<div class="flex gap-1.25 px-1 pb-1.5">
							<span class="h-2.25 w-2.25 rounded-full bg-[#ec6a5e]"></span>
							<span class="h-2.25 w-2.25 rounded-full bg-[#f4bf4f]"></span>
							<span class="h-2.25 w-2.25 rounded-full bg-[#61c554]"></span>
						</div>
					{/if}
					<div class="px-1.5 text-[9px] font-semibold tracking-[0.06em] text-fg-3 uppercase">
						{win ? 'This PC' : 'Favorites'}
					</div>
					{#each ['Desktop', 'Documents', 'Downloads'] as n (n)}
						<div class="flex items-center gap-1.5 px-1.5 py-0.75 text-[11px] text-fg-2">
							<Icon name="folderFill" size={12} />{n}
						</div>
					{/each}
					<div class="px-1.5 pt-2 text-[9px] font-semibold tracking-[0.06em] text-fg-3 uppercase">
						{win ? 'Network locations' : 'Locations'}
					</div>
					<div
						class="flex items-center gap-1.5 rounded-[5px] bg-accent-soft px-1.5 py-1 text-[11px] font-semibold text-accent-fg"
					>
						<Icon name="server" size={12} />Soteria
					</div>
				</div>
				<div class="grid flex-1 grid-cols-3 content-start gap-2.5 px-4 py-3.5">
					{#each shown as n (n)}
						<div class="flex flex-col items-center gap-1 text-center text-[9px] text-fg-2">
							<Icon name="folderFill" size={30} /><span class="truncate">{n}</span>
						</div>
					{/each}
				</div>
			</div>
			<img
				src="/bo.svg"
				alt=""
				class="absolute right-14 bottom-5.5 h-37.5 w-37.5 drop-shadow-[0_18px_30px_rgba(27,26,23,0.25)]"
			/>
		</div>
		<div class="flex flex-col gap-5 px-7 pt-6.5 pb-6">
			<div class="flex flex-col gap-2">
				<div class="text-[11px] font-semibold tracking-[0.08em] text-accent-fg uppercase">
					New · Network drive
				</div>
				<h1 class="text-[22px] font-semibold tracking-tight">
					{win ? 'Map Soteria as a network drive' : 'Open Soteria right in Finder'}
				</h1>
				<p class="text-fg-2">
					Connect {server.name} as a drive named <b class="font-semibold text-fg">Soteria</b>. Your
					files show up next to your local folders in {where}, without copying anything to this {pc}.
				</p>
			</div>
			<div class="flex flex-col gap-3">
				{@render benefit(
					'open',
					'Open and save from any app',
					'Edit a spreadsheet or a photo in the app you already use.'
				)}
				{@render benefit(
					'folderMove',
					`Drag and drop in ${where}`,
					`Move files between Soteria and your ${pc} like any other folder.`
				)}
				{@render benefit(
					'refresh',
					'No sync, no duplicates',
					'Nothing is copied to your disk. You always see the latest version of every file.'
				)}
			</div>
			<div class="flex items-center gap-2.5">
				<button class="btn btn-primary h-9" disabled={busy} onclick={connect}>
					<Icon name="server" size={15} />{busy ? 'Connecting…' : `Connect to ${where}`}
				</button>
				<button class="btn h-9" disabled={busy} onclick={() => done(false)}>Not now</button>
			</div>
		</div>
	</div>
</div>
