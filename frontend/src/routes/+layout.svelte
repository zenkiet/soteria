<script lang="ts">
	import '@/app/app.css';
	import { goto } from '$app/navigation';
	import { Toasts, WinLights } from '@/app/ui';
	import { SetBackground } from '@/shared/api';
	import { initTheme, prefs } from '@/shared/lib';
	import { Events, System } from '@wailsio/runtime';

	let { children } = $props();
	initTheme();
	const desktop = System.IsMac() || System.IsWindows();
	$effect(() => {
		if (desktop) SetBackground(prefs.background, prefs.notify);
	});
	$effect(() => Events.On('nav', (e) => goto(e.data)));

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
{@render children()}
