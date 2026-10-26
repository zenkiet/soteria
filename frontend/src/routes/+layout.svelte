<script lang="ts">
	import '#/app/app.css';
	import { goto } from '$app/navigation';
	import { Events, Window } from '@wailsio/runtime';
	import { on } from 'svelte/events';
	import { Toasts, UpdateDialog } from '#/app/ui/index.ts';
	import { SetAutoUpdate, SetBackground } from '#/shared/api/index.ts';
	import { desktop, mac, prefs, theme } from '#/shared/lib/index.ts';

	let { children } = $props();
	$effect(() => {
		if (desktop) SetBackground(prefs.background, prefs.notify);
	});
	$effect(() => Events.On('nav', (e) => goto(e.data)));
	// The backend owns the update-check schedule; it only needs the preference.
	$effect(() => {
		if (desktop) SetAutoUpdate(prefs.autoUpdate);
	});

	// The native window shows through while WebView2 catches up with a resize.
	function paint() {
		const [r, g, b] = getComputedStyle(document.body).backgroundColor.match(/\d+/g)!.map(Number);
		Window.SetBackgroundColour(r, g, b, 255);
	}
	$effect(() => {
		void theme.value;
		paint();
	});
	$effect(() => on(matchMedia('(prefers-color-scheme: dark)'), 'change', paint));

	const BUTTONS: Record<number, number> = { 3: -1, 4: 1 };
	const KEYS: Record<string, number> = mac ? { '[': -1, ']': 1 } : { ArrowLeft: -1, ArrowRight: 1 };

	function navigate(e: MouseEvent | KeyboardEvent) {
		const step =
			e instanceof MouseEvent ? BUTTONS[e.button] : (mac ? e.metaKey : e.altKey) ? KEYS[e.key] : 0;
		if (!step) return;
		// WebView2 also follows the mouse's side buttons unless mouseup is cancelled.
		e.preventDefault();
		history.go(step);
	}
</script>

<svelte:window onmouseup={navigate} onkeydown={navigate} />

<Toasts />
{#if desktop}<UpdateDialog />{/if}
{@render children()}
