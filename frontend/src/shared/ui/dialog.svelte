<script lang="ts">
	import { gsap } from 'gsap';
	import type { Snippet } from 'svelte';

	let {
		open,
		title,
		onclose,
		children
	}: { open: boolean; title: string; onclose: () => void; children: Snippet } = $props();

	let el: HTMLDialogElement;
	const reduce = matchMedia('(prefers-reduced-motion: reduce)').matches;

	$effect(() => {
		if (open && !el.open) {
			el.showModal();
			if (!reduce)
				gsap.fromTo(
					el,
					{ autoAlpha: 0, scale: 0.97, y: 6 },
					{ autoAlpha: 1, scale: 1, y: 0, duration: 0.2, ease: 'power2.out' }
				);
		} else if (!open && el.open) el.close();
	});
</script>

<dialog
	bind:this={el}
	{onclose}
	class="m-auto w-100 rounded-[10px] border border-line bg-surface p-5 text-fg shadow-[0_8px_24px_rgba(0,0,0,0.12)] backdrop:bg-black/20"
>
	<h2 class="mb-4 text-[15px] font-semibold tracking-tight">{title}</h2>
	{@render children()}
</dialog>
