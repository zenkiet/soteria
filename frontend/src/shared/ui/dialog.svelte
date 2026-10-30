<script lang="ts">
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

	const id = $props.id();
	// The backdrop is part of the dialog element, so a press there is one that misses the dialog's box.
	const outside = (e: PointerEvent & { currentTarget: HTMLDialogElement }) => {
		const r = e.currentTarget.getBoundingClientRect();
		const { clientX: x, clientY: y } = e;
		if (e.target === e.currentTarget && (x < r.left || x > r.right || y < r.top || y > r.bottom))
			e.currentTarget.close();
	};
</script>

<dialog
	{@attach (d) => {
		if (open && !d.open) d.showModal();
		else if (!open && d.open) d.close();
	}}
	{onclose}
	onpointerdown={outside}
	aria-labelledby={id}
	class="m-auto w-100 rounded-[10px] border border-line bg-surface p-5 text-fg shadow-[0_8px_24px_rgba(0,0,0,0.12)] backdrop:bg-black/35"
>
	<div class="mb-4 flex items-center gap-3.5">
		{#if lead}{@render lead()}{/if}
		<div class="min-w-0">
			<h2 {id} class="text-[15px] font-semibold tracking-tight">{title}</h2>
			{#if sub}<p class="text-xs text-fg-2">{sub}</p>{/if}
		</div>
	</div>
	{@render children()}
</dialog>
