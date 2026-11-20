<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';

	onMount(() => {
		void goto('/superman/alerts/configuration', { replaceState: true });
	});
</script>

<svelte:head>
	<title>Alert Rules &amp; Thresholds Configuration — SMS Whiteinfo Administrator</title>
</svelte:head>

<p class="text-xs text-gray-500">Opening alert configuration…</p>
