<script lang="ts">
	import { onMount } from 'svelte';
	import { page as pageStore } from '$app/stores';
	import { goto } from '$app/navigation';
	import { supermanApi } from '$lib/superman-api.js';
	import GatewayForm from '$lib/components/gateway/GatewayForm.svelte';
	import OperatorRatesPanel from '$lib/components/gateway/OperatorRatesPanel.svelte';
	import type { GatewayDetail } from '$lib/gateway-admin.js';

	let gatewayId = $derived(Number($pageStore.params.id));
	let gateway = $state<GatewayDetail | null>(null);
	let loading = $state(true);
	let loadError = $state<string | null>(null);
	let apiType = $state('iptsp');
	let flash = $state<{ type: 'success' | 'error'; text: string } | null>(null);
	let flashTimer: ReturnType<typeof setTimeout> | undefined;
	let deleting = $state(false);

	function notify(type: 'success' | 'error', text: string) {
		flash = { type, text };
		clearTimeout(flashTimer);
		flashTimer = setTimeout(() => (flash = null), 6000);
	}

	async function load() {
		loading = true;
		loadError = null;
		try {
			const res = await supermanApi.get<{ data: GatewayDetail }>(`/api/v1/superman/gateways/${gatewayId}?edit=1`);
			gateway = res.data;
			apiType = String(res.data.credentials?.api_type ?? 'iptsp').toLowerCase() === 'mno' ? 'mno' : 'iptsp';
		} catch (err: unknown) {
			loadError = err instanceof Error ? err.message : 'Unable to load the gateway.';
		} finally {
			loading = false;
		}
	}

	async function onSaved() {
		notify('success', 'Saved');
		window.scrollTo({ top: 0, behavior: 'smooth' });
		await load();
	}

	async function remove() {
		if (!gateway || !window.confirm(`Delete gateway "${gateway.name}"?`)) return;
		deleting = true;
		try {
			await supermanApi.delete(`/api/v1/superman/gateways/${gateway.id}`);
			void goto('/superman/gateways?saved=deleted');
		} catch (err: unknown) {
			notify('error', err instanceof Error ? err.message : 'Unable to delete the gateway.');
			deleting = false;
		}
	}

	onMount(() => void load());
</script>

<svelte:head>
	<title>{gateway ? `Edit ${gateway.name}` : 'Edit Gateway'} — SMS Whiteinfo Administrator</title>
</svelte:head>

<div class="space-y-6">
	<div class="flex flex-wrap items-end justify-between gap-3">
		<div>
			<nav class="flex items-center gap-2 text-sm text-gray-500 dark:text-gray-400" aria-label="Breadcrumb">
				<a href="/superman/gateways" class="hover:text-gray-800 dark:hover:text-gray-200">Gateways</a>
				<span>&rsaquo;</span>
				<a href="/superman/gateways/{gatewayId}" class="hover:text-gray-800 dark:hover:text-gray-200">{gateway?.name ?? '…'}</a>
				<span>&rsaquo;</span>
				<span>Edit</span>
			</nav>
			<h1 class="mt-1 text-3xl font-bold tracking-tight text-gray-950 dark:text-white">Edit {gateway?.name ?? ''}</h1>
		</div>
		{#if gateway}
			<button type="button" onclick={() => void remove()} disabled={deleting} data-testid="delete-gateway" class="inline-flex items-center gap-2 rounded-lg border border-rose-500 bg-white px-3.5 py-2 text-sm font-medium text-rose-600 shadow-xs hover:bg-rose-50 disabled:opacity-60 dark:bg-gray-900 dark:hover:bg-rose-950/30">
				<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke-width="1.75" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="m14.74 9-.346 9m-4.788 0L9.26 9m9.968-3.21c.342.052.682.107 1.022.166m-1.022-.165L18.16 19.673a2.25 2.25 0 0 1-2.244 2.077H8.084a2.25 2.25 0 0 1-2.244-2.077L4.772 5.79m14.456 0a48.108 48.108 0 0 0-3.478-.397m-12 .562c.34-.059.68-.114 1.022-.165m0 0a48.11 48.11 0 0 1 3.478-.397m7.5 0v-.916c0-1.18-.91-2.164-2.09-2.201a51.964 51.964 0 0 0-3.32 0c-1.18.037-2.09 1.022-2.09 2.201v.916m7.5 0a48.667 48.667 0 0 0-7.5 0" /></svg>
				Delete
			</button>
		{/if}
	</div>

	{#if flash}
		<div role="status" data-testid="flash" class="flex items-center justify-between rounded-lg border px-4 py-3 text-sm {flash.type === 'success' ? 'border-emerald-200 bg-emerald-50 text-emerald-800 dark:border-emerald-900 dark:bg-emerald-950/30 dark:text-emerald-300' : 'border-rose-200 bg-rose-50 text-rose-800 dark:border-rose-900 dark:bg-rose-950/30 dark:text-rose-300'}">
			<span>{flash.text}</span>
			<button type="button" onclick={() => (flash = null)} aria-label="Dismiss" class="ml-4 text-lg leading-none opacity-60 hover:opacity-100">×</button>
		</div>
	{/if}

	{#if loading && !gateway}
		<p class="py-16 text-center text-sm text-gray-400">Loading gateway…</p>
	{:else if loadError}
		<div role="alert" class="rounded-lg border border-rose-200 bg-rose-50 p-4 text-sm text-rose-700">{loadError}</div>
	{:else if gateway}
		<GatewayForm
			mode="edit"
			{gateway}
			{onSaved}
			onCancel={() => void goto('/superman/gateways')}
			onApiTypeChange={(value) => (apiType = value)}
		/>
		<OperatorRatesPanel gatewayId={gateway.id} {apiType} />
	{/if}
</div>
