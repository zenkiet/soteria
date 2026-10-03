<script lang="ts">
	import { Shell } from '#/app/ui/index.ts';

	let { data, children } = $props();
</script>

<Shell server={data.server}>{@render children()}</Shell>
