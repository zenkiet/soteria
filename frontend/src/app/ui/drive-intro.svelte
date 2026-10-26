<script lang="ts">
	import { connectDrive, Open, type Server } from '#/shared/api/index.ts';
	import {
		fileManager as where,
		msg,
		prefs,
		setPrefs,
		toast,
		windows as win
	} from '#/shared/lib/index.ts';
	import { Icon } from '#/shared/ui/index.ts';

	let { server }: { server: Server } = $props();
	let busy = $state(false);
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

<dialog
	class="m-auto w-140 overflow-y-auto rounded-2xl border border-line bg-surface p-0 text-fg shadow-[0_30px_80px_rgba(0,0,0,0.35)] backdrop:bg-black/45"
	aria-label="Network drive"
	onclose={() => done(false)}
	{@attach (d) => d.showModal()}
>
	<div class="flex flex-col gap-5 px-7 pt-6.5 pb-6">
		<div class="flex flex-col gap-2">
			<div class="text-[11px] font-semibold tracking-[0.08em] text-accent-fg uppercase">
				New · Network drive
			</div>
			<h1 class="text-[22px] font-semibold tracking-tight">
				Open Soteria right in {where}
			</h1>
			<p class="text-fg-2">
				Connect {server.name} as a drive named <b class="font-semibold text-fg">Soteria</b>. Your
				files show up next to your local folders in {where}, without copying anything to this {pc}.
				{#if win}The first time, Windows asks to install WinFsp, a free driver.{/if}
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
</dialog>
