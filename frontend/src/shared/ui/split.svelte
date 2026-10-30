<script lang="ts">
	import { Pane, PaneGroup, PaneResizer } from 'paneforge';
	import type { Snippet } from 'svelte';

	let {
		hidden = $bindable(),
		width = $bindable(),
		min,
		max,
		end = false,
		class: cls = '',
		pane,
		children
	}: {
		hidden: boolean;
		width: number;
		min: number;
		max: number;
		end?: boolean;
		class?: string;
		pane: Snippet;
		children: Snippet;
	} = $props();

	let w = $state(0);
	// A drag to collapse passes through the minimum on the way; reopening uses the width it started from.
	let start = 0;
	let resizing = $state(false);
	// PaneForge sizes in percent and re-lays out from defaultSize on any constraint change, so all of them track the pixel width.
	const pct = (px: number) => (px / (w || innerWidth)) * 100;
	const resized = (size: number) => {
		const px = Math.round((size / 100) * (w || innerWidth));
		if (size && px !== width) width = px;
	};
</script>

{#snippet side()}
	<!-- flex-grow is rounded to 3 digits, so the pin keeps the exact pixel width; easing the pin slides the pane -->
	<Pane
		collapsible
		defaultSize={hidden ? 0 : pct(width)}
		minSize={pct(min)}
		maxSize={pct(max)}
		onResize={resized}
		onCollapse={() => {
			hidden = true;
			if (start) width = start;
		}}
		onExpand={() => (hidden = false)}
		inert={hidden}
		class={resizing
			? ''
			: 'motion-safe:transition-[min-width,max-width] motion-safe:duration-200 motion-safe:ease-out'}
		style="min-width: {hidden ? 0 : width}px; max-width: {hidden ? 0 : width}px"
	>
		{@render pane()}
	</Pane>
{/snippet}

{#snippet bar()}
	<!-- Enter on a focused handle collapses without onCollapse, and a disabled one strands a drag -->
	<PaneResizer
		{hidden}
		tabindex={-1}
		onDraggingChange={(d) => {
			resizing = d;
			start = d ? width : 0;
		}}
		class="relative z-20 w-px shrink-0 bg-line after:absolute after:-inset-x-1 after:inset-y-0"
	/>
{/snippet}

<PaneGroup direction="horizontal" class={cls}>
	{#snippet child({ props })}
		<div {...props} bind:clientWidth={w}>
			{#if !end}{@render side()}{@render bar()}{/if}
			<Pane>{@render children()}</Pane>
			{#if end}{@render bar()}{@render side()}{/if}
		</div>
	{/snippet}
</PaneGroup>
