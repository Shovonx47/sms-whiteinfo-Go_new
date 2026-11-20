<script lang="ts">
	import { onMount } from 'svelte';
	import { supermanApi } from '$lib/superman-api.js';
	import ExportDropdown from '$lib/components/common/ExportDropdown.svelte';
	import Pagination from '$lib/components/common/Pagination.svelte';
	import type { ExportColumn } from '$lib/utils/export.js';

	interface Transaction {
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

	interface ReportSummary {
		total_credits: number;
		total_debits: number;
		net_flow: number;
		total_tx_count: number;
	}

	let transactions = $state<Transaction[]>([]);
	let summary = $state<ReportSummary>({
		total_credits: 0,
		total_debits: 0,
		net_flow: 0,
		total_tx_count: 0
	});

	let loading = $state(true);
	let search = $state('');
	let directionFilter = $state('all');
	let fromDate = $state('');
	let untilDate = $state('');

	// Pagination
	let currentPage = $state(1);
	let perPage = $state(10);
	let totalRecords = $state(0);
	let loadError = $state<string | null>(null);

	// Selected transaction for details modal
	let selectedTx = $state<Transaction | null>(null);
	let menuTxId = $state<number | null>(null);

	let requestSeq = 0;

	async function loadTransactions() {
		if (fromDate && untilDate && fromDate > untilDate) {
			loadError = 'The "from" date must be on or before the "to" date.';
			return;
		}
		const seq = ++requestSeq;
		loading = true;
		loadError = null;
		try {
			const query = new URLSearchParams({
				page: String(currentPage),
				per_page: String(perPage)
			});
			if (search) query.set('search', search);
			if (directionFilter !== 'all') query.set('direction', directionFilter);
			if (fromDate) query.set('from', fromDate);
			if (untilDate) query.set('until', untilDate);

			const res = await supermanApi.get<{
				data: Transaction[];
				meta: { page: number; per_page: number; total: number };
				summary: ReportSummary;
			}>(`/api/v1/superman/billing/reports?${query.toString()}`);

			if (seq !== requestSeq) return; // a newer request superseded this one

			transactions = res.data || [];
			totalRecords = res.meta?.total || 0;
			if (res.summary) {
				summary = res.summary;
			}
		} catch (err) {
			if (seq !== requestSeq) return;
			loadError = err instanceof Error ? err.message : 'Failed to load transactions.';
		} finally {
			if (seq === requestSeq) loading = false;
		}
	}

	function applyFilters() {
		currentPage = 1;
		void loadTransactions();
	}

	function goToPage(page: number) {
		currentPage = page;
		void loadTransactions();
	}

	function changePerPage(size: number) {
		perPage = size;
		currentPage = 1;
		void loadTransactions();
	}

	onMount(() => {
		void loadTransactions();
	});

	function formatCurrency(val: number): string {
		return '৳' + (val || 0).toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 });
	}

	function formatBdt(val: number): string {
		return 'BDT ' + (val || 0).toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 });
	}

	// "App\\Models\\Batch" / "message_batch" -> "Message Batch"
	function referenceLabel(ref: string | null | undefined): string {
		if (!ref) return 'General';
		const last = ref.split('\\').pop() ?? ref;
		const words = last
			.replace(/([a-z])([A-Z])/g, '$1 $2')
			.replace(/[_-]+/g, ' ')
			.trim()
			.toLowerCase();
		if (words === 'batch') return 'Message Batch';
		return words.replace(/\b\w/g, (c) => c.toUpperCase());
	}

	async function fetchAllForExport(): Promise<Transaction[]> {
		const query = new URLSearchParams({ export: '1' });
		if (search) query.set('search', search);
		if (directionFilter !== 'all') query.set('direction', directionFilter);
		if (fromDate) query.set('from', fromDate);
		if (untilDate) query.set('until', untilDate);
		const res = await supermanApi.get<{ data: Transaction[] }>(`/api/v1/superman/billing/reports?${query.toString()}`);
		return res.data || [];
	}

	const exportColumns: ExportColumn<Transaction>[] = [
		{ header: 'ID', getValue: (t) => `#${t.id}` },
		{ header: 'Date & Time', getValue: (t) => t.created_at },
		{ header: 'Customer', getValue: (t) => t.customer_name },
		{ header: 'Phone', getValue: (t) => t.customer_phone || '—' },
		{ header: 'Type', getValue: (t) => referenceLabel(t.reference) },
		{ header: 'Direction', getValue: (t) => (t.direction === 'credit' ? 'Credit' : 'Debit') },
		{ header: 'Amount (BDT)', getValue: (t) => `BDT ${t.amount.toFixed(2)}` },
		{ header: 'Balance Before', getValue: (t) => `BDT ${t.balance_before.toFixed(2)}` },
		{ header: 'Balance After', getValue: (t) => `BDT ${t.balance_after.toFixed(2)}` },
		{ header: 'Description', getValue: (t) => t.description || '—' }
	];
</script>

<svelte:window onclick={() => (menuTxId = null)} />


<svelte:head>
	<title>Transactions Ledger — SMS Whiteinfo Administrator</title>
</svelte:head>

<div class="space-y-6">
	<!-- Page Header -->
	<div class="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
		<div>
			<div class="flex items-center gap-2">
				<a href="/superman/billing" class="text-xs font-semibold text-gray-500 hover:text-emerald-600 dark:hover:text-emerald-400">
					Billing
				</a>
				<span class="text-xs text-gray-400">/</span>
				<span class="text-xs font-semibold text-gray-900 dark:text-white">Transactions</span>
			</div>
			<h1 class="mt-1 text-xl font-bold tracking-tight text-gray-900 dark:text-white sm:text-2xl">
				Financial Transactions Ledger
			</h1>
			<p class="text-xs text-gray-500 dark:text-gray-400">
				Double-entry balance audit ledger tracking credits, message deductions, top-ups, and refunds.
			</p>
		</div>

		<div class="flex flex-wrap items-center gap-2">
			<button
				type="button"
				onclick={loadTransactions}
				class="inline-flex items-center gap-1.5 whitespace-nowrap rounded-lg border border-gray-200 bg-white px-3 py-2 text-xs font-semibold text-gray-700 transition hover:bg-gray-50 dark:border-gray-800 dark:bg-gray-900 dark:text-gray-300 dark:hover:bg-gray-800"
			>
				<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
					<path stroke-linecap="round" stroke-linejoin="round" d="M16.023 9.348h4.992v-.001M2.985 19.644v-4.992m0 0h4.992m-4.993 0 3.181 3.183a8.25 8.25 0 0 0 13.803-3.7M4.031 9.865a8.25 8.25 0 0 1 13.803-3.7l3.181 3.182m0-4.991v4.99" />
				</svg>
				Refresh
			</button>
			<ExportDropdown
				title="Transactions Ledger"
				columns={exportColumns}
				getData={fetchAllForExport}
				disabled={totalRecords === 0}
			/>
			<a
				href="/superman/billing"
				class="inline-flex items-center gap-1.5 whitespace-nowrap rounded-lg border border-gray-200 bg-white px-3 py-2 text-xs font-semibold text-gray-700 transition hover:bg-gray-50 dark:border-gray-800 dark:bg-gray-900 dark:text-gray-300 dark:hover:bg-gray-800"
			>
				<svg class="h-4 w-4 text-gray-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M11 17l-5-5m0 0l5-5m-5 5h12" />
				</svg>
				Billing Control Center
			</a>
			<a
				href="/superman/billing/deposits"
				class="inline-flex items-center gap-1.5 whitespace-nowrap rounded-lg border border-gray-200 bg-white px-3 py-2 text-xs font-semibold text-gray-700 transition hover:bg-gray-50 dark:border-gray-800 dark:bg-gray-900 dark:text-gray-300 dark:hover:bg-gray-800"
			>
				<svg class="h-4 w-4 text-gray-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
				</svg>
				Deposits
			</a>
		</div>
	</div>

	<!-- Summary Cards -->
	<div class="grid grid-cols-2 gap-3 sm:grid-cols-4">
		<div class="rounded-xl border border-gray-200 bg-white p-4 shadow-xs dark:border-gray-800 dark:bg-gray-900">
			<span class="text-[11px] font-medium text-gray-400 uppercase tracking-wider">Total Credits</span>
			<p class="mt-1 text-lg font-bold text-emerald-600 dark:text-emerald-400">+{formatCurrency(summary.total_credits)}</p>
			<p class="text-[10px] text-gray-400">Total top-ups & refunds</p>
		</div>

		<div class="rounded-xl border border-gray-200 bg-white p-4 shadow-xs dark:border-gray-800 dark:bg-gray-900">
			<span class="text-[11px] font-medium text-gray-400 uppercase tracking-wider">Total Debits</span>
			<p class="mt-1 text-lg font-bold text-rose-600 dark:text-rose-400">-{formatCurrency(summary.total_debits)}</p>
			<p class="text-[10px] text-gray-400">Total SMS usages deducted</p>
		</div>

		<div class="rounded-xl border border-gray-200 bg-white p-4 shadow-xs dark:border-gray-800 dark:bg-gray-900">
			<span class="text-[11px] font-medium text-gray-400 uppercase tracking-wider">Net Cash Flow</span>
			<p class="mt-1 text-lg font-bold {summary.net_flow >= 0 ? 'text-emerald-600 dark:text-emerald-400' : 'text-rose-600'}">
				{formatCurrency(summary.net_flow)}
			</p>
			<p class="text-[10px] text-gray-400">Credit minus debit balance</p>
		</div>

		<div class="rounded-xl border border-gray-200 bg-white p-4 shadow-xs dark:border-gray-800 dark:bg-gray-900">
			<span class="text-[11px] font-medium text-gray-400 uppercase tracking-wider">Transactions Count</span>
			<p class="mt-1 text-lg font-bold text-gray-900 dark:text-white">{summary.total_tx_count.toLocaleString()}</p>
			<p class="text-[10px] text-gray-400">Matching filtered subset</p>
		</div>
	</div>

	<!-- Toolbar & Filters -->
	<div class="flex flex-col gap-3 rounded-xl border border-gray-200 bg-white p-3 shadow-xs dark:border-gray-800 dark:bg-gray-900 sm:flex-row sm:items-center sm:justify-between">
		<div class="relative flex-1 max-w-sm">
			<svg class="pointer-events-none absolute left-3 top-2.5 h-4 w-4 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
				<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
			</svg>
			<input
				type="text"
				bind:value={search}
				onkeydown={(e) => {
					if (e.key === 'Enter') applyFilters();
				}}
				placeholder="Search customer, note… (Enter)"
				class="w-full rounded-lg border border-gray-200 bg-gray-50 py-1.5 pl-9 pr-3 text-xs text-gray-900 focus:border-emerald-500 focus:bg-white focus:outline-hidden dark:border-gray-800 dark:bg-gray-800 dark:text-white"
			/>
		</div>

		<div class="flex flex-wrap items-center gap-2">
			<!-- Direction Filter -->
			<select
				bind:value={directionFilter}
				onchange={applyFilters}
				aria-label="Direction filter"
				class="rounded-lg border border-gray-200 bg-white px-2.5 py-1.5 text-xs text-gray-700 focus:border-emerald-500 focus:outline-hidden dark:border-gray-800 dark:bg-gray-800 dark:text-gray-300"
			>
				<option value="all">All Directions</option>
				<option value="credit">Credits Only (+)</option>
				<option value="debit">Debits Only (-)</option>
			</select>

			<!-- Date range -->
			<input
				type="date"
				bind:value={fromDate}
				onchange={applyFilters}
				aria-label="From date"
				class="rounded-lg border border-gray-200 bg-white px-2 py-1 text-xs font-mono text-gray-700 focus:border-emerald-500 focus:outline-hidden dark:border-gray-800 dark:bg-gray-800 dark:text-gray-300"
			/>
			<span class="text-xs text-gray-400">to</span>
			<input
				type="date"
				bind:value={untilDate}
				onchange={applyFilters}
				aria-label="To date"
				class="rounded-lg border border-gray-200 bg-white px-2 py-1 text-xs font-mono text-gray-700 focus:border-emerald-500 focus:outline-hidden dark:border-gray-800 dark:bg-gray-800 dark:text-gray-300"
			/>

			<button
				type="button"
				onclick={applyFilters}
				class="rounded-lg bg-gray-900 px-3 py-1.5 text-xs font-semibold text-white hover:bg-gray-800 dark:bg-white dark:text-gray-900 dark:hover:bg-gray-100"
			>
				Filter
			</button>
		</div>
	</div>

	<!-- Transactions Table -->
	<div class="overflow-hidden rounded-xl border border-gray-200 bg-white shadow-xs dark:border-gray-800 dark:bg-gray-900">
		{#if loadError}
			<div class="m-4 rounded-lg border border-rose-200 bg-rose-50 p-3 text-xs text-rose-700 dark:border-rose-900 dark:bg-rose-950/30 dark:text-rose-300">
				{loadError}
			</div>
		{/if}
		<div class="overflow-x-auto">
			<table class="w-full min-w-[960px] text-left text-sm" data-testid="transactions-table">
				<thead>
					<tr class="border-b border-gray-100 bg-gray-50/60 text-xs font-semibold text-gray-600 dark:border-gray-800 dark:bg-gray-800/40 dark:text-gray-300">
						<th class="px-5 py-3.5">ID</th>
						<th class="px-4 py-3.5">Date</th>
						<th class="px-4 py-3.5">Customer</th>
						<th class="px-4 py-3.5">Type</th>
						<th class="px-4 py-3.5">Direction</th>
						<th class="px-4 py-3.5 text-right">Amount</th>
						<th class="px-4 py-3.5 text-right">Balance Before</th>
						<th class="px-4 py-3.5 text-right">Balance After</th>
						<th class="w-14 px-4 py-3.5"><span class="sr-only">Actions</span></th>
					</tr>
				</thead>
				<tbody class="divide-y divide-gray-100 dark:divide-gray-800 {loading && transactions.length > 0 ? 'opacity-60' : ''}">
					{#each transactions as t (t.id)}
						<tr class="transition-colors hover:bg-gray-50/70 dark:hover:bg-gray-800/30">
							<td class="px-5 py-3.5 font-mono text-gray-600 dark:text-gray-300">{t.id}</td>
							<td class="whitespace-nowrap px-4 py-3.5 text-xs text-gray-500 dark:text-gray-400">{t.created_at}</td>
							<td class="px-4 py-3.5">
								<p class="font-medium text-gray-900 dark:text-white">{t.customer_name}</p>
								{#if t.customer_phone}
									<p class="font-mono text-[11px] text-gray-400">{t.customer_phone}</p>
								{/if}
							</td>
							<td class="whitespace-nowrap px-4 py-3.5">
								<span class="inline-flex items-center gap-1.5 whitespace-nowrap rounded-md border border-blue-200 bg-blue-50 px-2 py-0.5 text-xs font-medium text-blue-700 dark:border-blue-900 dark:bg-blue-950/40 dark:text-blue-300">
									<svg class="h-3.5 w-3.5 shrink-0" fill="none" viewBox="0 0 24 24" stroke-width="1.75" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="M21.75 6.75v10.5a2.25 2.25 0 0 1-2.25 2.25h-15a2.25 2.25 0 0 1-2.25-2.25V6.75m19.5 0A2.25 2.25 0 0 0 19.5 4.5h-15a2.25 2.25 0 0 0-2.25 2.25m19.5 0v.243a2.25 2.25 0 0 1-1.07 1.916l-7.5 4.615a2.25 2.25 0 0 1-2.36 0L3.32 8.91a2.25 2.25 0 0 1-1.07-1.916V6.75" /></svg>
									{referenceLabel(t.reference)}
								</span>
							</td>
							<td class="whitespace-nowrap px-4 py-3.5">
								<span
									class="inline-flex items-center gap-1 whitespace-nowrap rounded-md border px-2 py-0.5 text-xs font-semibold {t.direction === 'credit'
										? 'border-emerald-200 bg-emerald-50 text-emerald-700 dark:border-emerald-900 dark:bg-emerald-950/40 dark:text-emerald-300'
										: 'border-rose-200 bg-rose-50 text-rose-700 dark:border-rose-900 dark:bg-rose-950/40 dark:text-rose-300'}"
								>
									<svg class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
										{#if t.direction === 'credit'}
											<path stroke-linecap="round" stroke-linejoin="round" d="M19.5 13.5 12 21m0 0-7.5-7.5M12 21V3" />
										{:else}
											<path stroke-linecap="round" stroke-linejoin="round" d="M4.5 10.5 12 3m0 0 7.5 7.5M12 3v18" />
										{/if}
									</svg>
									{t.direction === 'credit' ? 'Credit' : 'Debit'}
								</span>
							</td>
							<td class="whitespace-nowrap px-4 py-3.5 text-right text-gray-900 dark:text-white">{formatBdt(t.amount)}</td>
							<td class="whitespace-nowrap px-4 py-3.5 text-right text-gray-600 dark:text-gray-300">{formatBdt(t.balance_before)}</td>
							<td class="whitespace-nowrap px-4 py-3.5 text-right text-gray-900 dark:text-white">{formatBdt(t.balance_after)}</td>
							<td class="relative px-4 py-3.5 text-right">
								<button
									type="button"
									aria-label="Row actions for transaction {t.id}"
									onclick={(e) => {
										e.stopPropagation();
										menuTxId = menuTxId === t.id ? null : t.id;
									}}
									class="inline-flex h-8 w-8 items-center justify-center rounded-lg border border-emerald-600/60 text-emerald-700 transition hover:bg-emerald-50 dark:border-emerald-700 dark:text-emerald-400 dark:hover:bg-emerald-950/30"
								>
									<svg class="h-4 w-4" fill="currentColor" viewBox="0 0 24 24"><circle cx="12" cy="5" r="1.6" /><circle cx="12" cy="12" r="1.6" /><circle cx="12" cy="19" r="1.6" /></svg>
								</button>
								{#if menuTxId === t.id}
									<div class="absolute right-4 top-full z-20 w-40 rounded-lg border border-gray-200 bg-white p-1 text-left shadow-lg dark:border-gray-800 dark:bg-gray-900">
										<button
											type="button"
											onclick={() => {
												selectedTx = t;
												menuTxId = null;
											}}
											class="flex w-full items-center rounded px-3 py-1.5 text-xs text-gray-700 hover:bg-gray-50 dark:text-gray-200 dark:hover:bg-gray-800"
										>
											View details
										</button>
										<a
											href="/superman/users/customers/{t.user_id}"
											class="flex w-full items-center rounded px-3 py-1.5 text-xs text-gray-700 hover:bg-gray-50 dark:text-gray-200 dark:hover:bg-gray-800"
										>
											View customer
										</a>
									</div>
								{/if}
							</td>
						</tr>
					{:else}
						<tr>
							<td colspan="9" class="py-12 text-center text-xs text-gray-400">
								{loading ? 'Loading ledger transactions…' : 'No transactions matching filters.'}
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>

		<Pagination
			total={totalRecords}
			page={currentPage}
			{perPage}
			{loading}
			onPageChange={goToPage}
			onPerPageChange={changePerPage}
		/>
	</div>
</div>

<!-- Modal: Transaction Details -->
{#if selectedTx}
	<div class="fixed inset-0 z-50 flex items-center justify-center bg-gray-900/60 backdrop-blur-xs p-4">
		<div class="w-full max-w-md rounded-2xl border border-gray-200 bg-white p-6 shadow-xl dark:border-gray-800 dark:bg-gray-900">
			<div class="flex items-center justify-between border-b border-gray-100 pb-3 dark:border-gray-800">
				<div>
					<h3 class="text-base font-bold text-gray-900 dark:text-white">Transaction #{selectedTx.id}</h3>
					<p class="text-xs text-gray-500">Ledger entry details</p>
				</div>
				<button
					type="button"
					onclick={() => (selectedTx = null)}
					class="rounded-lg p-1 text-gray-400 hover:bg-gray-100 hover:text-gray-600 dark:hover:bg-gray-800"
				>
					<svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
					</svg>
				</button>
			</div>

			<div class="mt-4 space-y-3 text-xs">
				<div class="flex items-center justify-between border-b border-gray-100 pb-2 dark:border-gray-800">
					<span class="text-gray-400">Customer</span>
					<span class="font-bold text-gray-900 dark:text-white">{selectedTx.customer_name}</span>
				</div>
				<div class="flex items-center justify-between border-b border-gray-100 pb-2 dark:border-gray-800">
					<span class="text-gray-400">Direction</span>
					<span class="font-bold uppercase {selectedTx.direction === 'credit' ? 'text-emerald-600' : 'text-rose-600'}">
						{selectedTx.direction}
					</span>
				</div>
				<div class="flex items-center justify-between border-b border-gray-100 pb-2 dark:border-gray-800">
					<span class="text-gray-400">Amount</span>
					<span class="font-mono font-bold text-sm {selectedTx.direction === 'credit' ? 'text-emerald-600' : 'text-rose-600'}">
						{selectedTx.direction === 'credit' ? '+' : '-'}{formatCurrency(selectedTx.amount)}
					</span>
				</div>
				<div class="flex items-center justify-between border-b border-gray-100 pb-2 dark:border-gray-800">
					<span class="text-gray-400">Balance Before</span>
					<span class="font-mono text-gray-700 dark:text-gray-300">{formatCurrency(selectedTx.balance_before)}</span>
				</div>
				<div class="flex items-center justify-between border-b border-gray-100 pb-2 dark:border-gray-800">
					<span class="text-gray-400">Balance After</span>
					<span class="font-mono font-semibold text-gray-900 dark:text-white">{formatCurrency(selectedTx.balance_after)}</span>
				</div>
				<div class="flex items-center justify-between border-b border-gray-100 pb-2 dark:border-gray-800">
					<span class="text-gray-400">Reference Type</span>
					<span class="font-mono text-gray-700 dark:text-gray-300">{selectedTx.reference}</span>
				</div>
				<div class="flex items-start justify-between border-b border-gray-100 pb-2 dark:border-gray-800">
					<span class="text-gray-400">Description</span>
					<span class="font-medium text-gray-800 dark:text-gray-200 text-right max-w-[240px]">
						{selectedTx.description || '—'}
					</span>
				</div>
				<div class="flex items-center justify-between">
					<span class="text-gray-400">Timestamp</span>
					<span class="font-mono text-gray-700 dark:text-gray-300">{selectedTx.created_at}</span>
				</div>
			</div>

			<div class="mt-6 flex justify-end">
				<button
					type="button"
					onclick={() => (selectedTx = null)}
					class="rounded-lg bg-gray-900 px-4 py-2 text-xs font-semibold text-white hover:bg-gray-800 dark:bg-white dark:text-gray-900"
				>
					Close
				</button>
			</div>
		</div>
	</div>
{/if}
