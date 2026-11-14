<script lang="ts">
	import { onMount } from 'svelte';
	import { spApi, SpApiError } from '$lib/sp-api.js';

	interface ReportData {
		from: string; to: string;
		stats: { balance: number; total_sent: number; total_delivered: number; total_failed: number; customer_spend: number; distributed: number; customers: number };
		daily: { date: string; total: number; delivered: number; cost: number }[];
		top_customers: { id: number; name: string; total: number; cost: number }[];
	}

	const today = new Date().toISOString().slice(0, 10);
	const monthAgo = new Date(Date.now() - 30 * 86400000).toISOString().slice(0, 10);

	let from = $state(monthAgo);
	let to = $state(today);
	let data = $state<ReportData | null>(null);
	let loading = $state(true);
	let error = $state('');

	async function load() {
		loading = true;
		error = '';
		try {
			const res = await spApi.get<{ data: ReportData }>(`/api/v1/sp/reports?from=${from}&to=${to}`);
			data = res.data;
		} catch (e) {
			error = e instanceof SpApiError ? e.message : 'Unable to load reports.';
		} finally {
			loading = false;
		}
	}

	onMount(load);
</script>

<svelte:head><title>Reports — SMS Whiteinfo S P Reseller</title></svelte:head>

<div class="space-y-4">
	<div class="flex flex-wrap items-end justify-between gap-3">
		<div>
			<h1 class="text-2xl font-bold tracking-tight text-gray-950 dark:text-white">Reports</h1>
			<p class="text-xs text-gray-500 dark:text-gray-400">Delivery, cost and distribution performance for your customers.</p>
		</div>
		<div class="flex items-end gap-2 text-xs">
			<div><label for="from" class="block text-gray-400">From</label><input id="from" type="date" bind:value={from} class="rounded-lg border border-gray-300 px-2 py-1 dark:border-gray-700 dark:bg-gray-900 dark:text-white" /></div>
			<div><label for="to" class="block text-gray-400">To</label><input id="to" type="date" bind:value={to} class="rounded-lg border border-gray-300 px-2 py-1 dark:border-gray-700 dark:bg-gray-900 dark:text-white" /></div>
			<button type="button" onclick={load} class="rounded-lg bg-amber-500 px-3 py-1.5 font-semibold text-white">Apply</button>
		</div>
	</div>

	{#if error}<div class="rounded-xl border border-rose-200 bg-rose-50 p-3 text-xs text-rose-700 dark:border-rose-900 dark:bg-rose-950/40 dark:text-rose-300">{error}</div>{/if}

	{#if loading}
		<p class="text-xs text-gray-400">Loading report…</p>
	{:else if data}
		<div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
			{#each [{ label: 'Messages', value: data.stats.total_sent }, { label: 'Delivered', value: data.stats.total_delivered }, { label: 'Failed', value: data.stats.total_failed }, { label: 'Customers', value: data.stats.customers }] as card}
				<div class="rounded-xl border border-gray-200 bg-white p-4 shadow-xs dark:border-gray-800 dark:bg-gray-900"><p class="text-[11px] uppercase text-gray-400">{card.label}</p><p class="mt-1 text-xl font-bold text-gray-900 dark:text-white">{card.value}</p></div>
			{/each}
			{#each [{ label: 'Customer Spend (BDT)', value: data.stats.customer_spend }, { label: 'Balance Distributed (BDT)', value: data.stats.distributed }, { label: 'Wallet Balance (BDT)', value: data.stats.balance }] as card}
				<div class="rounded-xl border border-gray-200 bg-white p-4 shadow-xs dark:border-gray-800 dark:bg-gray-900"><p class="text-[11px] uppercase text-gray-400">{card.label}</p><p class="mt-1 text-xl font-bold text-amber-600 dark:text-amber-400">{card.value.toFixed(2)}</p></div>
			{/each}
		</div>

		<div class="grid gap-4 lg:grid-cols-2">
			<div class="rounded-xl border border-gray-200 bg-white p-4 shadow-xs dark:border-gray-800 dark:bg-gray-900">
				<h3 class="mb-3 text-sm font-bold text-gray-900 dark:text-white">Daily traffic</h3>
				<div class="overflow-x-auto">
					<table class="w-full text-left text-xs">
						<thead><tr class="text-[10px] uppercase text-gray-400"><th class="py-2">Date</th><th class="py-2">Messages</th><th class="py-2">Delivered</th><th class="py-2 text-right">Cost</th></tr></thead>
						<tbody class="divide-y divide-gray-100 dark:divide-gray-800">
							{#each data.daily as day}
								<tr><td class="py-2">{day.date}</td><td class="py-2">{day.total}</td><td class="py-2">{day.delivered}</td><td class="py-2 text-right font-mono">{day.cost.toFixed(2)}</td></tr>
							{:else}
								<tr><td colspan="4" class="py-6 text-center text-gray-400">No traffic in this period.</td></tr>
							{/each}
						</tbody>
					</table>
				</div>
			</div>

			<div class="rounded-xl border border-gray-200 bg-white p-4 shadow-xs dark:border-gray-800 dark:bg-gray-900">
				<h3 class="mb-3 text-sm font-bold text-gray-900 dark:text-white">Top customers</h3>
				<div class="space-y-2">
					{#each data.top_customers as customer}
						<div class="flex items-center justify-between rounded-lg border border-gray-100 px-3 py-2 text-xs dark:border-gray-800">
							<span class="font-semibold text-gray-800 dark:text-gray-200">{customer.name}</span>
							<span class="text-gray-500">{customer.total} SMS · BDT {customer.cost.toFixed(2)}</span>
						</div>
					{:else}
						<p class="py-4 text-center text-xs text-gray-400">No customer activity.</p>
					{/each}
				</div>
			</div>
		</div>
	{/if}
</div>
