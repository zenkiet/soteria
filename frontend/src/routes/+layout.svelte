<script lang="ts">
	import '@/app/app.css';
	import { goto } from '$app/navigation';
	import { Toasts, UpdateDialog, WinLights } from '@/app/ui';
	import { SetBackground, check, update } from '@/shared/api';
	import { initTheme, prefs } from '@/shared/lib';
	import { Events, System } from '@wailsio/runtime';

	let { children } = $props();
	initTheme();
	const desktop = System.IsMac() || System.IsWindows();
	$effect(() => {
		if (desktop) SetBackground(prefs.background, prefs.notify);
	});
	$effect(() => Events.On('nav', (e) => goto(e.data)));
	// Update checks: 10 s after launch, then every 6 h; the dialog decides what to show.
	$effect(() => {
		if (!desktop || !prefs.autoUpdate) return;
		const run = () => update.s.state !== 'unconfigured' && check();
		const t = setTimeout(run, 10_000);
		const i = setInterval(run, 6 * 3_600_000);
		return () => {
			clearTimeout(t);
			clearInterval(i);
		};
	});

	const BUTTONS: Record<number, number> = { 3: -1, 4: 1 };
	const KEYS: Record<string, number> = { '[': -1, ']': 1 };

	function navigate(e: MouseEvent | KeyboardEvent) {
		const step = e instanceof MouseEvent ? BUTTONS[e.button] : e.metaKey ? KEYS[e.key] : 0;
		if (step) history.go(step);
	}
</script>

<svelte:window onmouseup={navigate} onkeydown={navigate} />

<Toasts />
{#if System.IsWindows()}<WinLights />{/if}
{#if desktop}<UpdateDialog />{/if}
{@render children()}
