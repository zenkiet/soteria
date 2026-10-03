<script module lang="ts">
	// Module scope: reconnect at launch, not every time this page is shown.
	let tried = false;
</script>

<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { ConnectSaved, DeepLink, Forget, Servers, type Server } from '#/shared/api/index.ts';
	import { mac, msg, prefs, toast, windows } from '#/shared/lib/index.ts';
	import { Icon } from '#/shared/ui/index.ts';

	let servers = $state<Server[]>([]);
	let busy = $state('');
	Servers().then((l) => {
		servers = l ?? [];
		// Most recent first; without a kept password reconnecting fails quietly and this list is the right screen.
		if (prefs.reconnect && !tried && servers.length) {
			tried = true;
			reconnect(servers[0]);
		}
	});

	async function reconnect(s: Server) {
		busy = s.id;
		try {
			await ConnectSaved(s.id);
			if (page.route.id === '/') await goto((await DeepLink()) || '/files');
		} catch (e) {
			if (!msg(e).includes('password required')) toast(msg(e), 'error');
		}
		busy = '';
	}

	async function pick(s: Server) {
		busy = s.id;
		try {
			await ConnectSaved(s.id);
			await goto((await DeepLink()) || '/files');
		} catch (e) {
			if (msg(e).includes('password required')) await goto(`/sign-in?id=${s.id}`);
			else toast(msg(e), 'error');
		}
		busy = '';
	}

	async function forget(s: Server) {
		await Forget(s.id);
		servers = servers.filter((x) => x.id !== s.id);
	}
</script>

<div class="flex h-screen flex-col">
	<div class="h-13 shrink-0" style="--wails-draggable: drag"></div>
	<div class="flex flex-1 items-center justify-center pb-13">
		<div class="flex w-130 flex-col gap-5">
			<div>
				<h1 class="text-[22px] font-semibold tracking-tight">Choose a server</h1>
				<p class="mt-1.5 text-fg-2">
					{mac
						? 'Connections saved on this Mac. Passwords stay in the Keychain.'
						: windows
							? 'Connections saved on this PC. Passwords are stored encrypted for your Windows account.'
							: 'Connections saved on this computer.'}
				</p>
			</div>
			{#if servers.length}
				<div class="reveal divide-y divide-line rounded-lg border border-line bg-surface">
					{#each servers as s (s.id)}
						<div class="flex items-center gap-2 py-3.5 pr-3 pl-4">
							<button
								class="flex min-w-0 flex-1 items-center gap-3.5 text-left disabled:opacity-60"
								onclick={() => pick(s)}
								disabled={!!busy}
							>
								<span
									class="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-surface-2 text-fg-2"
									><Icon name="server" /></span
								>
								<span class="flex min-w-0 flex-1 flex-col gap-0.5">
									<span class="text-sm font-medium">{s.name}</span>
									<span class="truncate font-mono text-xs text-fg-3"
										>{s.url.replace(/^https?:\/\//, '')} · {s.username}</span
									>
								</span>
								<span class="text-xs text-fg-2">
									{busy === s.id
										? 'Connecting…'
										: s.remember
											? 'Password saved'
											: 'Asks for password'}
								</span>
								<Icon name="chevronRight" class="text-fg-3" />
							</button>
							<button
								class="btn btn-ghost h-7 w-7 px-0"
								onclick={() => forget(s)}
								disabled={!!busy}
								aria-label="Forget {s.name}"><Icon name="x" size={14} /></button
							>
						</div>
					{/each}
				</div>
			{/if}
			<a href="/sign-in" class="btn h-9"><Icon name="plus" size={15} />Add server</a>
		</div>
	</div>
</div>
