<script lang="ts">
	import { dismiss, pop, toasts } from '@/shared/lib';
	import { Icon } from '@/shared/ui';
</script>

<div class="pointer-events-none fixed right-5 bottom-5 z-20 flex flex-col gap-2">
	{#each toasts.list as t (t.id)}
		<div
			use:pop
			class="pointer-events-auto flex w-95 items-center gap-2.5 rounded-lg border border-line bg-surface px-3 py-2.5 shadow-[0_8px_24px_rgba(0,0,0,0.12)]"
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
