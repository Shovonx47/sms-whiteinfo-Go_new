<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';

	onMount(() => {
		void goto('/superman/templates', { replaceState: true });
	});
</script>

<div class="p-8 text-center text-xs text-gray-400">
	Redirecting to Bulk SMS Templates...
</div>
