<script lang="ts">
	import '#/app/app.css';
	import { goto } from '$app/navigation';
	import { Events } from '@wailsio/runtime';
	import { Toasts, UpdateDialog, WinLights } from '#/app/ui/index.ts';
	import { SetAutoUpdate, SetBackground } from '#/shared/api/index.ts';
	import { desktop, prefs, windows } from '#/shared/lib/index.ts';

	let { children } = $props();
	if (windows) document.documentElement.dataset.os = 'windows';
	$effect(() => {
		if (desktop) SetBackground(prefs.background, prefs.notify);
	});
	$effect(() => Events.On('nav', (e) => goto(e.data)));
	// The backend owns the update-check schedule; it only needs the preference.
	$effect(() => {
		if (desktop) SetAutoUpdate(prefs.autoUpdate);
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
{#if windows}<WinLights />{/if}
{#if desktop}<UpdateDialog />{/if}
{@render children()}
