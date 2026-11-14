<script lang="ts">
	import { onMount } from 'svelte';
	import { spApi, SpApiError } from '$lib/sp-api.js';

	interface QueueItem { id: number; customer: string; recipient: string; status: string; created_at: string; }
	interface Counts { queued: number; processing: number; sent: number; failed: number; }

	let counts = $state<Counts>({ queued: 0, processing: 0, sent: 0, failed: 0 });
	let queue = $state<QueueItem[]>([]);
	let loading = $state(true);
	let error = $state('');
	let timer: ReturnType<typeof setInterval>;

	async function load() {
		try {
			const res = await spApi.get<{ data: { counts: Counts; queue: QueueItem[] } }>('/api/v1/sp/queue');
			counts = res.data.counts;
			queue = res.data.queue ?? [];
			error = '';
		} catch (e) {
			error = e instanceof SpApiError ? e.message : 'Unable to load live queue.';
		} finally {
			loading = false;
		}
	}

	onMount(() => {
		load();
		timer = setInterval(load, 10000);
		return () => clearInterval(timer);
	});
</script>

<svelte:head><title>Live Queue — SMS Whiteinfo S P Reseller</title></svelte:head>

<div class="space-y-4">
	<div>
		<h1 class="text-2xl font-bold tracking-tight text-gray-950 dark:text-white">Live Queue</h1>
		<p class="text-xs text-gray-500 dark:text-gray-400">Messages from your customers currently waiting or processing. Refreshes every 10 seconds.</p>
	</div>

	{#if error}<div class="rounded-xl border border-rose-200 bg-rose-50 p-3 text-xs text-rose-700 dark:border-rose-900 dark:bg-rose-950/40 dark:text-rose-300">{error}</div>{/if}

	<div class="grid gap-3 sm:grid-cols-4">
		{#each [{ label: 'Queued', value: counts.queued }, { label: 'Processing', value: counts.processing }, { label: 'Sent', value: counts.sent }, { label: 'Failed', value: counts.failed }] as card}
			<div class="rounded-xl border border-gray-200 bg-white p-4 shadow-xs dark:border-gray-800 dark:bg-gray-900">
				<p class="text-[11px] uppercase text-gray-400">{card.label}</p>
				<p class="mt-1 text-xl font-bold text-gray-900 dark:text-white">{card.value}</p>
			</div>
		{/each}
	</div>

	<div class="overflow-x-auto rounded-xl border border-gray-200 bg-white shadow-xs dark:border-gray-800 dark:bg-gray-900">
		<table class="w-full text-left text-xs">
			<thead><tr class="border-b border-gray-100 text-[10px] uppercase text-gray-400 dark:border-gray-800">
				<th class="px-4 py-3">Message</th><th class="px-4 py-3">Customer</th><th class="px-4 py-3">Recipient</th><th class="px-4 py-3">Status</th><th class="px-4 py-3">Queued At</th>
			</tr></thead>
			<tbody class="divide-y divide-gray-100 dark:divide-gray-800">
				{#if loading}
					<tr><td colspan="5" class="px-4 py-10 text-center text-gray-400">Loading queue…</td></tr>
				{:else if queue.length === 0}
					<tr><td colspan="5" class="px-4 py-10 text-center text-gray-400">The queue is empty.</td></tr>
				{:else}
					{#each queue as item (item.id)}
						<tr>
							<td class="px-4 py-3 font-mono">#{item.id}</td>
							<td class="px-4 py-3 font-semibold text-gray-800 dark:text-gray-200">{item.customer}</td>
							<td class="px-4 py-3 font-mono">{item.recipient}</td>
							<td class="px-4 py-3">{item.status}</td>
							<td class="px-4 py-3 text-gray-500">{item.created_at}</td>
						</tr>
					{/each}
				{/if}
			</tbody>
		</table>
	</div>
</div>
