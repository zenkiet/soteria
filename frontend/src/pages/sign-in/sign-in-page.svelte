<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { Connect, Servers, type Server } from '@/shared/api';
	import { msg } from '@/shared/lib';
	import { Icon } from '@/shared/ui';
	import { System } from '@wailsio/runtime';

	let s = $state<Server>({
		id: '',
		name: '',
		url: '',
		username: '',
		insecure: false,
		remember: true,
		lastUsed: 0
	});
	let password = $state('');
	let show = $state(false);
	let error = $state('');
	let busy = $state(false);

	const id = page.url.searchParams.get('id');
	if (id)
		Servers().then((l) => {
			const saved = l?.find((x) => x.id === id);
			if (saved) s = { ...saved };
		});

	async function submit(e: SubmitEvent) {
		e.preventDefault();
		busy = true;
		error = '';
		try {
			await Connect(s, password);
			await goto('/files');
		} catch (err) {
			error = msg(err);
		}
		busy = false;
	}
</script>

<div class="flex h-screen flex-col">
	<div class="h-13 shrink-0" style="--wails-draggable: drag"></div>
	<div class="flex flex-1 items-center justify-center pb-13">
		<form class="flex w-100 flex-col gap-6" onsubmit={submit}>
			<a href="/" class="inline-flex items-center gap-1 text-xs text-fg-2 hover:text-fg">
				<Icon name="chevronLeft" size={14} />Servers
			</a>
			<div>
				<h1 class="text-[22px] font-semibold tracking-tight">
					{s.id ? `Sign in to ${s.name}` : 'Add a server'}
				</h1>
				<p class="mt-1.5 text-fg-2">Any WebDAV server, such as SFTPGo or Nextcloud.</p>
			</div>
			{#if error}
				<div
					class="flex gap-2.5 rounded-md bg-danger-soft px-3 py-2.5 text-xs leading-relaxed text-danger"
				>
					<Icon name="info" size={15} class="mt-px" />{error}
				</div>
			{/if}
			<div class="flex flex-col gap-4">
				<label class="field">
					<span class="label">Server address</span>
					<input
						class="input font-mono"
						bind:value={s.url}
						placeholder="http://192.168.1.10:8081"
						required
					/>
					<span class="hint">Include the port. Use https:// when the server has TLS.</span>
				</label>
				<label class="field">
					<span class="label">Username</span>
					<input class="input" bind:value={s.username} autocomplete="username" required />
				</label>
				<label class="field">
					<span class="label">Password</span>
					<span class="relative">
						<input
							class="input pr-10"
							type={show ? 'text' : 'password'}
							bind:value={password}
							autocomplete="current-password"
						/>
						<button
							type="button"
							class="absolute top-1/2 right-2.5 -translate-y-1/2 text-fg-3 hover:text-fg"
							onclick={() => (show = !show)}
							aria-label="Show password"><Icon name="eye" size={15} /></button
						>
					</span>
				</label>
				<label class="flex items-start gap-2.5">
					<input type="checkbox" bind:checked={s.remember} class="mt-0.5 accent-primary" />
					<span
						><span class="block">Remember me</span><span class="hint"
							>{System.IsMac()
								? 'Saved to the macOS Keychain.'
								: System.IsWindows()
									? 'Saved encrypted for your Windows account.'
									: 'Not available on this platform yet.'}</span
						></span
					>
				</label>
				<label class="flex items-start gap-2.5">
					<input type="checkbox" bind:checked={s.insecure} class="mt-0.5 accent-primary" />
					<span
						><span class="block">Trust self-signed certificate</span><span class="hint"
							>Only for servers you run yourself.</span
						></span
					>
				</label>
			</div>
			<button class="btn btn-primary h-9" disabled={busy}>{busy ? 'Connecting…' : 'Sign in'}</button
			>
		</form>
	</div>
</div>
