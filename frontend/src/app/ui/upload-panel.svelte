<script lang="ts">
	import { goto } from '$app/navigation';
	import { SvelteSet } from 'svelte/reactivity';
	import { Cancel, CancelGroup, Retry, transfers, type Transfer } from '@/shared/api';
	import { bytes, filesHref, iconFor, parent, pop } from '@/shared/lib';
	import { Dialog, Icon } from '@/shared/ui';

	type Row = { key: string; name: string; dir: string; group: boolean; ts: Transfer[] };

	const hidden = new SvelteSet<string>();
	let collapsed = $state(false);
	let confirm = $state(false);
	let start = { at: 0, done: 0 };

	const live = (t: Transfer) => t.status === 'running' || t.status === 'queued';
	const sum = (ts: Transfer[], f: (t: Transfer) => number) => ts.reduce((s, t) => s + f(t), 0);
	const status = (ts: Transfer[]) =>
		['running', 'error', 'queued'].find((s) => ts.some((t) => t.status === s)) ?? 'done';
	const rank: Record<string, number> = { error: 0, running: 1, queued: 2, done: 3 };

	const items = $derived(transfers.list.filter((t) => t.kind === 'upload' && !hidden.has(t.id)));
	const rows = $derived.by(() => {
		const out: Row[] = [];
		for (const t of items) {
			const key = t.group || t.id;
			const row = out.find((r) => r.key === key);
			if (row) row.ts.push(t);
			else if (t.group)
				out.push({
					key,
					name: t.group.slice(t.group.lastIndexOf('/') + 1),
					dir: t.group,
					group: true,
					ts: [t]
				});
			else out.push({ key, name: t.name, dir: parent(t.remote), group: false, ts: [t] });
		}
		return out.sort((a, b) => rank[status(a.ts)] - rank[status(b.ts)]);
	});
	const pending = $derived(items.filter(live).length);
	const failed = $derived(items.filter((t) => t.status === 'error').length);
	const done = $derived(sum(items, (t) => t.done));
	const total = $derived(sum(items, (t) => t.total));
	const eta = $derived.by(() => {
		if (!pending) return ((start = { at: 0, done: 0 }), '');
		if (!start.at) start = { at: Date.now(), done };
		const s = (Date.now() - start.at) / 1000;
		const rate = (done - start.done) / s;
		if (s < 3 || rate <= 0) return '';
		const left = (total - done) / rate;
		return left < 60 ? ' · less than a minute left' : ` · about ${Math.round(left / 60)} min left`;
	});

	const dismiss = () => items.forEach((t) => hidden.add(t.id));
	const cancelAll = () => {
		items.filter(live).forEach((t) => Cancel(t.id));
		dismiss();
		confirm = false;
	};
	const show = (r: Row) =>
		goto(
			r.group ? filesHref(r.dir) : filesHref(r.dir) + '?focus=' + encodeURIComponent(r.ts[0].remote)
		);
	const sub = (r: Row) => {
		const s = status(r.ts);
		if (s === 'queued') return 'Waiting';
		if (s === 'error')
			return r.group ? `${r.ts.filter((t) => t.status === 'error').length} failed` : r.ts[0].error;
		if (s === 'done') return `${bytes(sum(r.ts, (t) => t.total))} · to ${r.dir}`;
		return `${bytes(sum(r.ts, (t) => t.done))} of ${bytes(sum(r.ts, (t) => t.total))}`;
	};
</script>

{#if items.length}
	<div
		use:pop
		class="absolute right-5 bottom-5 z-10 flex w-95 flex-col overflow-hidden rounded-[10px] border border-line bg-surface shadow-[0_8px_24px_rgba(0,0,0,0.12)]"
	>
		<div class="flex flex-col gap-2 py-2.5 pr-2 pl-3.5">
			<div class="flex items-center gap-2.5">
				{#if !pending}<Icon name="check" size={15} class="text-ok" />{/if}
				<div class="flex min-w-0 flex-1 flex-col">
					<div class="truncate font-semibold">
						{pending
							? `Uploading ${pending} of ${items.length} items`
							: `${items.length} items uploaded`}
						{#if failed}<span class="font-normal text-danger"> · {failed} failed</span>{/if}
					</div>
					{#if !collapsed}
						<div class="truncate text-[11px] text-fg-3">
							{pending ? `${bytes(done)} of ${bytes(total)}${eta}` : bytes(total)}
						</div>
					{/if}
				</div>
				<button
					class="btn btn-ghost h-6.5 w-6.5 px-0 {collapsed ? 'rotate-180' : ''}"
					onclick={() => (collapsed = !collapsed)}
					aria-label={collapsed ? 'Expand' : 'Collapse'}
					><Icon name="chevronDown" size={14} /></button
				>
				<button
					class="btn btn-ghost h-6.5 w-6.5 px-0"
					onclick={() => (pending ? (confirm = true) : dismiss())}
					aria-label="Close"><Icon name="x" size={14} /></button
				>
			</div>
			{#if pending}
				<div class="h-0.75 overflow-hidden rounded-full bg-surface-2">
					<div class="h-full bg-accent" style="width:{total ? (done / total) * 100 : 0}%"></div>
				</div>
			{/if}
		</div>
		{#if !collapsed}
			<div class="max-h-80 overflow-y-auto">
				{#each rows as r (r.key)}
					{@const s = status(r.ts)}
					{@const t = r.ts[0]}
					<div
						class="group flex items-center gap-2.5 border-t border-line py-2 pr-2.5 pl-3.5 hover:bg-surface-2"
					>
						<Icon
							name={r.group ? 'folderFill' : iconFor({ name: r.name, dir: false })}
							size={16}
							class={r.group ? '' : 'text-fg-2'}
						/>
						<div class="flex min-w-0 flex-1 flex-col gap-1">
							<div class="flex items-baseline justify-between gap-2.5">
								<span class="truncate">{r.name}</span>
								<span class="font-mono text-[11px] text-fg-3">
									{#if r.group}{r.ts.filter((x) => x.status === 'done').length} / {r.ts.length}
									{:else if s === 'running'}{Math.round((t.done / t.total) * 100) || 0}%{/if}
								</span>
							</div>
							{#if s === 'running'}
								<div class="h-0.75 overflow-hidden rounded-full bg-surface-2">
									<div
										class="h-full bg-accent"
										style="width:{(sum(r.ts, (x) => x.done) / sum(r.ts, (x) => x.total)) * 100}%"
									></div>
								</div>
							{/if}
							<span class="truncate text-[11px] {s === 'error' ? 'text-danger' : 'text-fg-3'}"
								>{sub(r)}</span
							>
						</div>
						{#if s === 'running' || s === 'queued'}
							<button
								class="btn btn-ghost h-6 w-6 px-0"
								onclick={() => (r.group ? CancelGroup(r.dir) : Cancel(t.id))}
								aria-label="Cancel"
							>
								<Icon name="x" size={13} />
							</button>
						{:else if s === 'error'}
							<button
								class="text-xs font-medium text-accent-fg hover:underline"
								onclick={() => r.ts.filter((x) => x.status === 'error').forEach((x) => Retry(x.id))}
								>Retry</button
							>
						{:else}
							<Icon name="check" size={14} class="text-ok group-hover:hidden" />
							<button
								class="hidden text-xs font-medium text-accent-fg group-hover:inline hover:underline"
								onclick={() => show(r)}>Show in folder</button
							>
						{/if}
					</div>
				{/each}
			</div>
			<div class="flex items-center justify-between border-t border-line py-2 pr-2.5 pl-3.5">
				<a href="/transfers" class="text-xs font-medium text-fg-2 hover:text-fg">Open Transfers</a>
				{#if pending}
					<button class="btn h-6.5 text-xs" onclick={() => (confirm = true)}>Cancel all</button>
				{:else if failed}
					<button
						class="btn h-6.5 text-xs"
						onclick={() => items.filter((t) => t.status === 'error').forEach((t) => Retry(t.id))}
						>Retry all</button
					>
				{/if}
			</div>
		{/if}
	</div>
{/if}

<Dialog
	open={confirm}
	title="Cancel {pending} remaining uploads?"
	onclose={() => (confirm = false)}
>
	<p class="text-fg-2">
		Files already on the server stay there. Files half-way through are removed.
	</p>
	<div class="mt-5 flex justify-end gap-2">
		<button class="btn" onclick={() => (confirm = false)}>Keep uploading</button>
		<button class="btn btn-danger" onclick={cancelAll}>Cancel uploads</button>
	</div>
</Dialog>
