<script lang="ts">
	import { page } from '$app/state';
</script>

<div class="flex h-screen flex-col items-center justify-center gap-4 text-center">
	<p class="text-[15px] font-semibold">{page.error?.message ?? 'Something went wrong'}</p>
	<a class="btn" href="/">Back to servers</a>
</div>
