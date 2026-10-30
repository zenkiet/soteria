<script lang="ts">
	import type { Snippet } from 'svelte';
	import Icon from './icon.svelte';

	let { count, onclear, children }: { count: number; onclear: () => void; children: Snippet } =
		$props();
</script>

<div class="flex shrink-0 items-center gap-1.5">
	<button
		class="btn btn-ghost btn-icon"
		onclick={onclear}
		aria-label="Clear selection"
		title="Clear selection (Esc)"
	>
		<Icon name="x" size={15} />
	</button>
	<span class="mr-2 font-medium whitespace-nowrap">{count} selected</span>
	{@render children()}
</div>
