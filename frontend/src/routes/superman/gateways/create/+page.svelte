<script lang="ts">
	import { goto } from '$app/navigation';
	import GatewayForm from '$lib/components/gateway/GatewayForm.svelte';

	let flash = $state<string | null>(null);
	let flashTimer: ReturnType<typeof setTimeout> | undefined;
	let formKey = $state(0);

	function onSaved(result: { id: number; another: boolean }) {
		if (result.another) {
			flash = 'Gateway created.';
			clearTimeout(flashTimer);
			flashTimer = setTimeout(() => (flash = null), 6000);
			window.scrollTo({ top: 0, behavior: 'smooth' });
			return;
		}
		void goto('/superman/gateways?saved=created');
	}
</script>

<svelte:head>
	<title>Create Gateway — SMS Whiteinfo Administrator</title>
</svelte:head>

<div class="space-y-5">
	<div>
		<nav class="flex items-center gap-2 text-sm text-gray-500 dark:text-gray-400" aria-label="Breadcrumb">
			<a href="/superman/gateways" class="hover:text-gray-800 dark:hover:text-gray-200">Gateways</a>
			<span>&rsaquo;</span>
			<span>Create</span>
		</nav>
		<h1 class="mt-1 text-3xl font-bold tracking-tight text-gray-950 dark:text-white">Create Gateway</h1>
	</div>

	{#if flash}
		<div role="status" data-testid="form-success" class="rounded-lg border border-emerald-200 bg-emerald-50 px-4 py-3 text-sm text-emerald-800 dark:border-emerald-900 dark:bg-emerald-950/30 dark:text-emerald-300">{flash}</div>
	{/if}

	{#key formKey}
		<GatewayForm mode="create" {onSaved} onCancel={() => void goto('/superman/gateways')} />
	{/key}
</div>
