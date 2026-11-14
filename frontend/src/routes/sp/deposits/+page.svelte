<script lang="ts">
	import { onMount } from 'svelte';
	import { spApi, SpApiError } from '$lib/sp-api.js';

	interface Deposit {
		id: number; customer: string; email: string; amount: number; currency: string;
		payment_method: string; transaction_id: string; status: string; paid_at: string | null; created_at: string;
	}
	interface Meta { page: number; per_page: number; total: number; }

	let deposits = $state<Deposit[]>([]);
	let meta = $state<Meta>({ page: 1, per_page: 25, total: 0 });
	let loading = $state(true);
	let error = $state('');

	async function load(page = 1) {
		loading = true;
		error = '';
		try {
			const res = await spApi.get<{ data: Deposit[]; meta: Meta }>(`/api/v1/sp/deposits?page=${page}`);
			deposits = res.data ?? [];
			meta = res.meta ?? meta;
		} catch (e) {
			error = e instanceof SpApiError ? e.message : 'Unable to load deposits.';
		} finally {
			loading = false;
		}
	}

	onMount(() => load());
</script>

<svelte:head><title>Deposits — SMS Whiteinfo S P Reseller</title></svelte:head>

<div class="space-y-4">
	<div>
		<h1 class="text-2xl font-bold tracking-tight text-gray-950 dark:text-white">Deposits</h1>
		<p class="text-xs text-gray-500 dark:text-gray-400">Payment history for customers under your reseller account.</p>
	</div>

	{#if error}<div class="rounded-xl border border-rose-200 bg-rose-50 p-3 text-xs text-rose-700 dark:border-rose-900 dark:bg-rose-950/40 dark:text-rose-300">{error}</div>{/if}

	<div class="overflow-x-auto rounded-xl border border-gray-200 bg-white shadow-xs dark:border-gray-800 dark:bg-gray-900">
		<table class="w-full text-left text-xs">
			<thead><tr class="border-b border-gray-100 text-[10px] uppercase text-gray-400 dark:border-gray-800">
				<th class="px-4 py-3">Customer</th><th class="px-4 py-3">Amount</th><th class="px-4 py-3">Method</th><th class="px-4 py-3">Transaction</th><th class="px-4 py-3">Status</th><th class="px-4 py-3">Paid At</th><th class="px-4 py-3">Created</th>
			</tr></thead>
			<tbody class="divide-y divide-gray-100 dark:divide-gray-800">
				{#if loading}
					<tr><td colspan="7" class="px-4 py-10 text-center text-gray-400">Loading deposits…</td></tr>
				{:else if deposits.length === 0}
					<tr><td colspan="7" class="px-4 py-10 text-center text-gray-400">No deposits found.</td></tr>
				{:else}
					{#each deposits as deposit (deposit.id)}
						<tr>
							<td class="px-4 py-3"><div class="font-semibold text-gray-900 dark:text-white">{deposit.customer}</div><div class="text-[11px] text-gray-400">{deposit.email}</div></td>
							<td class="px-4 py-3 font-mono font-bold text-amber-600 dark:text-amber-400">{deposit.currency} {deposit.amount.toFixed(2)}</td>
							<td class="px-4 py-3">{deposit.payment_method || '—'}</td>
							<td class="px-4 py-3 font-mono">{deposit.transaction_id || '—'}</td>
							<td class="px-4 py-3">{deposit.status}</td>
							<td class="px-4 py-3">{deposit.paid_at ? new Date(deposit.paid_at).toLocaleString() : '—'}</td>
							<td class="px-4 py-3 text-gray-500">{deposit.created_at}</td>
						</tr>
					{/each}
				{/if}
			</tbody>
		</table>
	</div>

	<div class="flex items-center justify-between text-xs text-gray-500">
		<span>{meta.total} deposits</span>
		<div class="flex gap-2">
			<button type="button" disabled={meta.page <= 1} onclick={() => load(meta.page - 1)} class="rounded-lg border border-gray-200 px-2 py-1 disabled:opacity-40 dark:border-gray-700">Previous</button>
			<button type="button" disabled={meta.page * meta.per_page >= meta.total} onclick={() => load(meta.page + 1)} class="rounded-lg border border-gray-200 px-2 py-1 disabled:opacity-40 dark:border-gray-700">Next</button>
		</div>
	</div>
</div>
