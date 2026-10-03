<script lang="ts">
	import { flip } from 'svelte/animate';
	import { fade } from 'svelte/transition';
	import { dismiss, toasts } from '#/shared/lib/index.ts';
	import { Icon } from '#/shared/ui/index.ts';

	// A popover sits in the top layer; re-showing it on every toast keeps it above a modal opened since.
	function raise(el: HTMLElement) {
		void toasts.list.length;
		el.hidePopover();
		el.showPopover();
	}
</script>

<div
	popover="manual"
	role="status"
	class="pointer-events-none fixed inset-auto right-5 bottom-5 m-0 flex flex-col gap-2 overflow-visible border-0 bg-transparent p-0"
	{@attach raise}
>
	{#each toasts.list as t (t.id)}
		<div
			animate:flip={{ duration: 180 }}
			out:fade={{ duration: 150 }}
			class="pop pointer-events-auto flex w-95 items-center gap-2.5 rounded-lg border border-line bg-surface px-3 py-2.5 text-fg shadow-[0_8px_24px_rgba(0,0,0,0.12)]"
		>
			<Icon
				name={t.kind === 'ok' ? 'check' : 'info'}
				class={t.kind === 'ok' ? 'text-ok' : 'text-danger'}
			/>
			<span class="flex-1">{t.text}</span>
			{#if t.action}
				{@const a = t.action}
				<button
					class="text-xs font-medium text-accent-fg hover:underline"
					onclick={() => {
						a.run();
						dismiss(t.id);
					}}>{a.label}</button
				>
			{/if}
		</div>
	{/each}
</div>
