<script lang="ts">
	import { page } from '$app/state';
	import FilesPage from '@/pages/files/files-page.svelte';
</script>

<FilesPage path={page.params.path ?? ''} />
