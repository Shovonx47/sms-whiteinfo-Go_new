<script lang="ts">
	import { onMount } from 'svelte';
	import { supermanApi } from '$lib/superman-api.js';
	import ExportDropdown from '$lib/components/common/ExportDropdown.svelte';
	import Pagination from '$lib/components/common/Pagination.svelte';
	import type { ExportColumn } from '$lib/utils/export.js';

	interface TransactionItem {
		id: number;
		user_id: number;
		customer_name: string;
		customer_phone: string;
		reference: string;
		direction: string;
		amount: number;
		balance_before: number;
		balance_after: number;
		description: string | null;
		created_at: string;
	}

	interface Summary {
		total_credits: number;
		total_debits: number;
		net_flow: number;
		total_tx_count: number;
	}

	let transactions = $state<TransactionItem[]>([]);
	let summary = $state<Summary>({
		total_credits: 0,
		total_debits: 0,
		net_flow: 0,
		total_tx_count: 0
	});
	let loading = $state(true);
	let error = $state<string | null>(null);
	let search = $state('');
	let directionFilter = $state('all');
	let dateFrom = $state('');
	let dateTo = $state('');
	let currentPage = $state(1);
	let perPage = $state(10);
	let total = $state(0);

	async function loadFinancialReport(targetPage = currentPage) {
		currentPage = targetPage;
		loading = true;
		error = null;
		try {
			const params = new URLSearchParams({
				page: String(currentPage),
				per_page: String(perPage)
			});
			if (search.trim()) params.set('search', search.trim());
			if (directionFilter !== 'all') params.set('direction', directionFilter);
			if (dateFrom) params.set('from', dateFrom);
			if (dateTo) params.set('until', dateTo);

			const res = await supermanApi.get<{
				data: TransactionItem[];
				meta?: { total: number };
				summary?: Summary;
			}>(`/api/v1/superman/billing/reports?${params.toString()}`);

			transactions = res.data ?? [];
			total = res.meta?.total ?? transactions.length;
			if (res.summary) summary = res.summary;
		} catch (err: unknown) {
			error = err instanceof Error ? err.message : 'Failed to load financial report.';
		} finally {
			loading = false;
		}
	}

	async function getAllForExport(): Promise<TransactionItem[]> {
		const params = new URLSearchParams({ export: '1' });
		if (search.trim()) params.set('search', search.trim());
		if (directionFilter !== 'all') params.set('direction', directionFilter);
		if (dateFrom) params.set('from', dateFrom);
		if (dateTo) params.set('until', dateTo);
		const res = await supermanApi.get<{ data: TransactionItem[] }>(
			`/api/v1/superman/billing/reports?${params.toString()}`
		);
		return res.data ?? [];
	}

	onMount(() => {
		void loadFinancialReport(1);
	});

	const exportColumns: ExportColumn<TransactionItem>[] = [
		{ header: 'TX ID', getValue: (t) => `#${t.id}` },
		{ header: 'Customer', getValue: (t) => t.customer_name },
		{ header: 'Direction', getValue: (t) => t.direction.toUpperCase() },
		{ header: 'Amount', getValue: (t) => `BDT ${Number(t.amount).toFixed(2)}` },
		{ header: 'Balance Before', getValue: (t) => `BDT ${Number(t.balance_before).toFixed(2)}` },
		{ header: 'Balance After', getValue: (t) => `BDT ${Number(t.balance_after).toFixed(2)}` },
		{ header: 'Description', getValue: (t) => t.description || '—' },
		{ header: 'Date', getValue: (t) => t.created_at || '—' }
	];
</script>

<svelte:head>
	<title>Financial Reports — Superman Administrator</title>
</svelte:head>

<div class="space-y-6">
	<div class="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
		<div>
			<h1 class="text-2xl font-bold tracking-tight text-gray-950 dark:text-white">Financial & Ledger Reports</h1>
			<p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
				Audit trail of customer deposits, wallet top-ups, and per-message debit deductions.
			</p>
		</div>
		<div class="flex items-center gap-2">
			<ExportDropdown
				title="Financial and Ledger Reports"
				columns={exportColumns}
				getData={getAllForExport}
				disabled={total === 0}
			/>
			<button type="button" onclick={() => void loadFinancialReport(currentPage)} class="rounded-lg border border-gray-200 bg-white px-3 py-2 text-xs font-medium text-gray-700 shadow-xs hover:bg-gray-50 dark:border-gray-800 dark:bg-gray-900 dark:text-gray-300">
				Refresh Ledger
			</button>
		</div>
	</div>

	<!-- Stats Bar -->
	<div class="grid grid-cols-2 gap-4 sm:grid-cols-3">
		<div class="rounded-xl border border-gray-200 bg-white p-4 shadow-xs dark:border-gray-800 dark:bg-gray-900">
			<span class="text-[11px] font-semibold text-gray-500 uppercase tracking-wider">Total Top-Ups (Credits)</span>
			<p class="mt-1 text-2xl font-bold text-emerald-600">৳{summary.total_credits.toFixed(2)}</p>
		</div>
		<div class="rounded-xl border border-gray-200 bg-white p-4 shadow-xs dark:border-gray-800 dark:bg-gray-900">
			<span class="text-[11px] font-semibold text-gray-500 uppercase tracking-wider">Total Usage (Debits)</span>
			<p class="mt-1 text-2xl font-bold text-rose-600">৳{summary.total_debits.toFixed(2)}</p>
		</div>
		<div class="col-span-2 rounded-xl border border-gray-200 bg-white p-4 shadow-xs sm:col-span-1 dark:border-gray-800 dark:bg-gray-900">
			<span class="text-[11px] font-semibold text-gray-500 uppercase tracking-wider">Total Audit Records</span>
			<p class="mt-1 text-2xl font-bold text-gray-900 dark:text-white">{summary.total_tx_count.toLocaleString()}</p>
		</div>
	</div>

	<form
		onsubmit={(e) => {
			e.preventDefault();
			void loadFinancialReport(1);
		}}
		class="flex flex-wrap items-center gap-3 rounded-xl border border-gray-200 bg-white p-4 shadow-xs dark:border-gray-800 dark:bg-gray-900"
	>
		<input
			type="search"
			bind:value={search}
			placeholder="Search by customer name, phone, or note..."
			class="w-full max-w-sm rounded-lg border border-gray-200 bg-gray-50/70 px-3 py-1.5 text-xs outline-none focus:border-emerald-500 dark:border-gray-700 dark:bg-gray-800 dark:text-white"
		/>
		<select
			bind:value={directionFilter}
			onchange={() => void loadFinancialReport(1)}
			aria-label="Direction filter"
			class="rounded-lg border border-gray-200 bg-white px-3 py-1.5 text-xs text-gray-700 outline-none focus:border-emerald-500 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-300"
		>
			<option value="all">All Movements</option>
			<option value="credit">Credits Only (+)</option>
			<option value="debit">Debits Only (-)</option>
		</select>
		<input
			type="date"
			bind:value={dateFrom}
			aria-label="From date"
			class="rounded-lg border border-gray-200 bg-white px-2 py-1.5 text-xs text-gray-700 outline-none focus:border-emerald-500 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-300"
		/>
		<input
			type="date"
			bind:value={dateTo}
			aria-label="Until date"
			class="rounded-lg border border-gray-200 bg-white px-2 py-1.5 text-xs text-gray-700 outline-none focus:border-emerald-500 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-300"
		/>
		<button type="submit" class="rounded-lg bg-emerald-600 px-4 py-1.5 text-xs font-semibold text-white shadow-xs hover:bg-emerald-700">
			Filter
		</button>
	</form>

	<div class="overflow-hidden rounded-xl border border-gray-200 bg-white shadow-xs dark:border-gray-800 dark:bg-gray-900">
		{#if loading && transactions.length === 0}
			<div class="py-16 text-center text-xs text-gray-500">Loading financial ledger...</div>
		{:else if error}
			<div class="p-6 text-center text-xs text-rose-600">{error}</div>
		{:else if transactions.length === 0}
			<div class="py-16 text-center text-xs text-gray-500">No financial transactions recorded.</div>
		{:else}
			<div class="overflow-x-auto">
				<table class="w-full text-left text-xs">
					<thead>
						<tr class="border-b border-gray-100 bg-gray-50/50 text-[10px] font-semibold uppercase tracking-wider text-gray-500 dark:border-gray-800 dark:bg-gray-800/50 dark:text-gray-400">
							<th class="px-4 py-3">Customer</th>
							<th class="px-4 py-3">Direction</th>
							<th class="px-4 py-3 text-right">Amount</th>
							<th class="px-4 py-3 text-right">Balance Before</th>
							<th class="px-4 py-3 text-right">Balance After</th>
							<th class="px-4 py-3">Description</th>
							<th class="px-4 py-3 text-right">Date</th>
						</tr>
					</thead>
					<tbody class="divide-y divide-gray-100 dark:divide-gray-800 {loading ? 'opacity-60' : ''}">
						{#each transactions as t (t.id)}
							<tr class="hover:bg-gray-50/50 dark:hover:bg-gray-800/30">
								<td class="px-4 py-3">
									<div class="font-semibold text-gray-950 dark:text-white">{t.customer_name}</div>
								</td>
								<td class="px-4 py-3">
									<span class="inline-flex rounded-full px-2 py-0.5 text-[10px] font-bold uppercase {t.direction === 'credit' ? 'bg-emerald-50 text-emerald-700' : 'bg-rose-50 text-rose-700'}">
										{t.direction}
									</span>
								</td>
								<td class="px-4 py-3 text-right font-bold {t.direction === 'credit' ? 'text-emerald-600' : 'text-rose-600'}">
									৳{Number(t.amount).toFixed(2)}
								</td>
								<td class="px-4 py-3 text-right text-gray-500">৳{Number(t.balance_before || 0).toFixed(2)}</td>
								<td class="px-4 py-3 text-right font-semibold text-gray-900 dark:text-white">৳{Number(t.balance_after || 0).toFixed(2)}</td>
								<td class="max-w-[320px] truncate px-4 py-3 text-gray-600 dark:text-gray-400" title={t.description || ''}>{t.description || '—'}</td>
								<td class="whitespace-nowrap px-4 py-3 text-right text-gray-400">{t.created_at || '—'}</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</div>
		{/if}

		<Pagination
			total={total}
			page={currentPage}
			{perPage}
			{loading}
			onPageChange={(p) => void loadFinancialReport(p)}
			onPerPageChange={(size) => {
				perPage = size;
				void loadFinancialReport(1);
			}}
		/>
	</div>
</div>
