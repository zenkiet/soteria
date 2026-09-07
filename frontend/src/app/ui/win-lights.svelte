<script lang="ts">
	import { Window } from '@wailsio/runtime';

	let dim = $state(false);
	let max = $state(false);
	const sync = () => Window.IsMaximised().then((m) => (max = m));
	sync();

	// Double-click on a drag region maximises, like the native caption did.
	function zoom(e: MouseEvent) {
		const el = e.target as Element;
		if (el.closest('button,input,a')) return;
		if (getComputedStyle(el).getPropertyValue('--wails-draggable').trim() === 'drag')
			Window.ToggleMaximise();
	}
</script>

<svelte:window
	onfocus={() => (dim = false)}
	onblur={() => (dim = true)}
	onresize={sync}
	ondblclick={zoom}
/>

<div
	class="lights fixed top-0 left-4 z-50 flex h-13 items-center gap-2"
	class:dim
	style="--wails-draggable: no-drag"
>
	<button
		title="Close"
		style="--c: #ff5f57; --r: #e0443e; --i: #4d0000"
		onclick={() => Window.Close()}
	>
		<svg viewBox="0 0 12 12"><path d="M3.6 3.6l4.8 4.8M8.4 3.6l-4.8 4.8" /></svg>
	</button>
	<button
		title="Minimize"
		style="--c: #febc2e; --r: #dea123; --i: #995700"
		onclick={() => Window.Minimise()}
	>
		<svg viewBox="0 0 12 12"><path d="M2.8 6h6.4" /></svg>
	</button>
	<button
		title={max ? 'Restore' : 'Maximize'}
		style="--c: #28c840; --r: #1aab29; --i: #006500"
		onclick={() => Window.ToggleMaximise()}
	>
		<svg viewBox="0 0 12 12">
			<path
				d={max
					? 'M2.4 6.2h3.8V2.4zM9.6 5.8H5.8v3.8z'
					: 'M2.8 2.8h3.4L2.8 6.2zM9.2 9.2H5.8L9.2 5.8z'}
				fill="currentColor"
				stroke="none"
			/>
		</svg>
	</button>
</div>
