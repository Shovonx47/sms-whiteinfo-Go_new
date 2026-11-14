<script lang="ts">
	import { api } from '$lib/api.js';
	import { onMount } from 'svelte';

	interface Transaction {
		id: number;
		transactionable_type: string;
		direction: 'credit' | 'debit';
		amount: string;
		balance_before: string;
		balance_after: string;
		description: string;
		created_at: string;
	}

	interface Meta {
		total: number;
		per_page: number;
		current_page: number;
	}

	let transactions = $state<Transaction[]>([]);
	let meta = $state<Meta>({ total: 0, per_page: 10, current_page: 1 });
	let loading = $state(true);
	let search = $state('');
	let perPage = $state(10);
	let currentPage = $state(1);

	let selectedTx = $state<Transaction | null>(null);

	async function load(p = 1) {
		loading = true;
		currentPage = p;
		try {
			const params = new URLSearchParams({ page: String(p), per_page: String(perPage) });
			if (search) params.set('search', search);
			const data = await api.get<{ data: Transaction[]; meta: Meta }>(`/api/v1/customer/transactions?${params}`);
			transactions = data.data || [];
			meta = data.meta || { total: 0, per_page: perPage, current_page: p };
		} catch {
			transactions = [];
			meta = { total: 0, per_page: perPage, current_page: p };
		} finally {
			loading = false;
		}
	}

	onMount(() => load(1));

	function formatRef(type: string): string {
		if (!type) return 'System';
		if (type.includes('Deposit')) return 'Deposit';
		if (type.includes('Batch')) return 'Batch';
		if (type.includes('Message')) return 'Message';
		return type;
	}

	let filteredTransactions = $derived.by(() => {
		if (!search) return transactions;
		const q = search.toLowerCase();
		return transactions.filter(
			(t) =>
				t.description?.toLowerCase().includes(q) ||
				t.transactionable_type?.toLowerCase().includes(q) ||
				t.direction?.toLowerCase().includes(q) ||
				t.amount?.includes(q)
		);
	});
</script>

<svelte:head>
	<title>Transactions - White INFO</title>
</svelte:head>

<div class="space-y-6">
	<!-- Breadcrumb -->
	<nav class="flex items-center gap-2 text-xs font-medium text-gray-500">
		<a href="/billing/transactions" class="hover:text-gray-700">Transactions</a>
		<svg class="h-3 w-3 text-gray-400" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
			<path stroke-linecap="round" stroke-linejoin="round" d="m8.25 4.5 7.5 7.5-7.5 7.5" />
		</svg>
		<span class="text-gray-900 font-semibold">List</span>
	</nav>

	<!-- Page Heading -->
	<div class="flex items-center justify-between">
		<h1 class="text-2xl font-bold tracking-tight text-gray-950">Transactions</h1>
	</div>

	<!-- Main Card -->
	<div class="rounded-xl border border-gray-200 bg-white shadow-sm overflow-hidden">
		<!-- Toolbar: Search -->
		<div class="flex items-center justify-end gap-2 border-b border-gray-100 p-3 bg-white">
			<div class="relative w-64">
				<div class="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-2.5">
					<svg class="h-4 w-4 text-gray-400" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
						<path stroke-linecap="round" stroke-linejoin="round" d="m21 21-5.197-5.197m0 0A7.5 7.5 0 1 0 5.196 5.196a7.5 7.5 0 0 0 10.607 10.607Z" />
					</svg>
				</div>
				<input
					type="text"
					bind:value={search}
					placeholder="Search"
					class="w-full rounded-lg border border-gray-300 bg-white py-1.5 pr-3 pl-8 text-xs text-gray-900 placeholder-gray-400 focus:border-teal-600 focus:outline-none focus:ring-1 focus:ring-teal-600"
				/>
			</div>
		</div>

		<!-- Table -->
		<div class="overflow-x-auto">
			<table class="w-full text-left border-collapse text-xs">
				<thead>
					<tr class="border-b border-gray-200 bg-gray-50/70 text-gray-600 font-semibold tracking-wider uppercase text-[11px]">
						<th class="py-3 px-4">REFERENCE</th>
						<th class="py-3 px-4">DIRECTION</th>
						<th class="py-3 px-4 text-right">AMOUNT</th>
						<th class="py-3 px-4 text-right">BALANCE</th>
						<th class="py-3 px-4">DESCRIPTION</th>
						<th class="py-3 px-4">DATE</th>
						<th class="py-3 px-4 text-right"></th>
					</tr>
				</thead>
				<tbody class="divide-y divide-gray-100 bg-white">
					{#if loading}
						<tr>
							<td colspan="7" class="py-12 text-center text-gray-500">
								<div class="inline-block h-6 w-6 animate-spin rounded-full border-2 border-gray-300 border-t-teal-600"></div>
								<p class="mt-2 text-xs">Loading transactions...</p>
							</td>
						</tr>
					{:else if filteredTransactions.length === 0}
						<tr>
							<td colspan="7" class="py-12 text-center text-gray-500">
								<p class="text-sm font-medium text-gray-700">No transactions recorded</p>
								<p class="mt-1 text-xs text-gray-400">Account credits and debits will appear here.</p>
							</td>
						</tr>
					{:else}
						{#each filteredTransactions as tx (tx.id)}
							<tr class="hover:bg-gray-50/70 transition">
								<td class="py-3 px-4 whitespace-nowrap">
									<span class="inline-flex items-center rounded-md bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-700">
										{formatRef(tx.transactionable_type)}
									</span>
								</td>
								<td class="py-3 px-4 whitespace-nowrap">
									{#if tx.direction === 'credit'}
										<span class="inline-flex items-center gap-1 rounded-md bg-emerald-50 px-2 py-0.5 text-xs font-semibold text-emerald-700 border border-emerald-200">
											<svg class="h-3 w-3" fill="none" viewBox="0 0 24 24" stroke-width="2.5" stroke="currentColor">
												<path stroke-linecap="round" stroke-linejoin="round" d="m19.5 8.25-7.5 7.5-7.5-7.5" />
											</svg>
											Credit
										</span>
									{:else}
										<span class="inline-flex items-center gap-1 rounded-md bg-red-50 px-2 py-0.5 text-xs font-semibold text-red-700 border border-red-200">
											<svg class="h-3 w-3" fill="none" viewBox="0 0 24 24" stroke-width="2.5" stroke="currentColor">
												<path stroke-linecap="round" stroke-linejoin="round" d="m4.5 15.75 7.5-7.5 7.5 7.5" />
											</svg>
											Debit
										</span>
									{/if}
								</td>
								<td class="py-3 px-4 font-mono font-semibold text-right whitespace-nowrap {tx.direction === 'credit' ? 'text-emerald-600' : 'text-gray-900'}">
									BDT {Number(tx.amount).toFixed(2)}
								</td>
								<td class="py-3 px-4 font-mono text-gray-700 font-medium text-right whitespace-nowrap">
									BDT {Number(tx.balance_after).toFixed(2)}
								</td>
								<td class="py-3 px-4 text-gray-700 max-w-xs truncate" title={tx.description}>
									{tx.description || '—'}
								</td>
								<td class="py-3 px-4 text-gray-600 whitespace-nowrap">
									{new Date(tx.created_at).toLocaleString('en-GB', { day: '2-digit', month: 'short', year: 'numeric', hour: '2-digit', minute: '2-digit' })}
								</td>
								<td class="py-3 px-4 text-right whitespace-nowrap">
									<button
										type="button"
										onclick={() => (selectedTx = tx)}
										class="rounded p-1 text-gray-400 hover:bg-gray-100 hover:text-gray-700 cursor-pointer"
										title="View transaction"
									>
										<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
											<path stroke-linecap="round" stroke-linejoin="round" d="M12 6.75a.75.75 0 1 1 0-1.5.75.75 0 0 1 0 1.5ZM12 12.75a.75.75 0 1 1 0-1.5.75.75 0 0 1 0 1.5ZM12 18.75a.75.75 0 1 1 0-1.5.75.75 0 0 1 0 1.5Z" />
										</svg>
									</button>
								</td>
							</tr>
						{/each}
					{/if}
				</tbody>
			</table>
		</div>

		<!-- Pagination Footer -->
		<div class="flex items-center justify-between border-t border-gray-200 px-4 py-3 bg-white text-xs text-gray-600">
			<div>
				Showing {filteredTransactions.length > 0 ? (currentPage - 1) * perPage + 1 : 0} to {Math.min(currentPage * perPage, meta.total)} of {meta.total} results
			</div>
			<div class="flex items-center gap-2">
				<span class="text-xs text-gray-500">Per page</span>
				<select
					bind:value={perPage}
					onchange={() => load(1)}
					class="rounded-lg border border-gray-300 bg-white py-1 px-2 text-xs text-gray-700 focus:border-teal-600 focus:outline-none focus:ring-1 focus:ring-teal-600"
				>
					<option value={10}>10</option>
					<option value={25}>25</option>
					<option value={50}>50</option>
				</select>
			</div>
		</div>
	</div>
</div>

<!-- View Transaction Modal -->
{#if selectedTx}
	<div class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4 animate-fade-in backdrop-blur-xs">
		<div class="relative w-full max-w-md rounded-2xl bg-white p-6 shadow-xl border border-gray-100">
			<div class="flex items-center justify-between pb-3 border-b border-gray-100">
				<h3 class="text-base font-bold text-gray-900">Transaction Details</h3>
				<button
					type="button"
					onclick={() => (selectedTx = null)}
					class="text-gray-400 hover:text-gray-600 cursor-pointer"
				>
					<svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
						<path stroke-linecap="round" stroke-linejoin="round" d="M6 18 18 6M6 6l12 12" />
					</svg>
				</button>
			</div>

			<div class="mt-4 space-y-3 text-xs">
				<div class="flex justify-between py-1 border-b border-gray-50">
					<span class="text-gray-500 font-medium">Reference</span>
					<span class="font-semibold text-gray-900">{formatRef(selectedTx.transactionable_type)}</span>
				</div>
				<div class="flex justify-between py-1 border-b border-gray-50">
					<span class="text-gray-500 font-medium">Direction</span>
					<span class="font-semibold capitalize {selectedTx.direction === 'credit' ? 'text-emerald-700' : 'text-red-700'}">
						{selectedTx.direction}
					</span>
				</div>
				<div class="flex justify-between py-1 border-b border-gray-50">
					<span class="text-gray-500 font-medium">Amount</span>
					<span class="font-semibold font-mono text-gray-900">BDT {Number(selectedTx.amount).toFixed(2)}</span>
				</div>
				<div class="flex justify-between py-1 border-b border-gray-50">
					<span class="text-gray-500 font-medium">Balance Before</span>
					<span class="font-mono text-gray-700">BDT {Number(selectedTx.balance_before).toFixed(2)}</span>
				</div>
				<div class="flex justify-between py-1 border-b border-gray-50">
					<span class="text-gray-500 font-medium">Balance After</span>
					<span class="font-mono font-semibold text-gray-900">BDT {Number(selectedTx.balance_after).toFixed(2)}</span>
				</div>
				<div class="flex justify-between py-1 border-b border-gray-50">
					<span class="text-gray-500 font-medium">Date</span>
					<span class="text-gray-700">{new Date(selectedTx.created_at).toLocaleString()}</span>
				</div>
				<div class="py-2">
					<span class="block text-gray-500 font-medium mb-1">Description</span>
					<div class="rounded-lg bg-gray-50 p-2.5 text-xs text-gray-800 border border-gray-200">
						{selectedTx.description || '—'}
					</div>
				</div>
			</div>

			<div class="mt-6 flex justify-end">
				<button
					type="button"
					onclick={() => (selectedTx = null)}
					class="rounded-lg bg-gray-100 px-4 py-2 text-xs font-semibold text-gray-700 hover:bg-gray-200 transition cursor-pointer"
				>
					Close
				</button>
			</div>
		</div>
	</div>
{/if}
