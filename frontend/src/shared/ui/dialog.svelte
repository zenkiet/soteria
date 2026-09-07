<script lang="ts">
	import { gsap } from 'gsap';
	import type { Snippet } from 'svelte';

	let {
		open,
		title,
		sub = '',
		lead,
		onclose,
		children
	}: {
		open: boolean;
		title: string;
		sub?: string;
		lead?: Snippet;
		onclose: () => void;
		children: Snippet;
	} = $props();

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
	<div class="mb-4 flex items-center gap-3.5">
		{#if lead}{@render lead()}{/if}
		<div class="min-w-0">
			<h2 class="text-[15px] font-semibold tracking-tight">{title}</h2>
			{#if sub}<p class="text-xs text-fg-2">{sub}</p>{/if}
		</div>
	</div>
	{@render children()}
</dialog>
