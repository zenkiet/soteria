<script lang="ts">
	import { Toaster, toast } from 'svelte-sonner';
	import { Icon } from '#/shared/ui/index.ts';

	// A popover sits in the top layer; re-showing it on every toast keeps it above a modal opened since.
	function raise(el: HTMLElement) {
		void toast.getActiveToasts().length;
		el.hidePopover();
		el.showPopover();
	}
</script>

<!-- block: a closed popover is display:none, and sonner measures each toast as it mounts -->
<Toaster
	position="bottom-center"
	popover="manual"
	class="inset-auto block overflow-visible border-0 bg-transparent"
	style="font-family: inherit; --normal-bg: var(--surface); --normal-border: var(--line); --normal-text: var(--fg)"
	{@attach raise}
>
	{#snippet successIcon()}<Icon name="check" class="text-ok" />{/snippet}
	{#snippet errorIcon()}<Icon name="info" class="text-danger" />{/snippet}
</Toaster>
