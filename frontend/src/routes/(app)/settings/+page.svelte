<script lang="ts">
	import SettingsPage from '@/pages/settings/settings-page.svelte';

	let { data } = $props();
</script>

<SettingsPage server={data.server} />
