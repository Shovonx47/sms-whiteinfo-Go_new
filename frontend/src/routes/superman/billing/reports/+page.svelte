<script lang="ts">
	import { onMount } from 'svelte';
	import { supermanApi } from '$lib/superman-api.js';
	import ExportDropdown from '$lib/components/common/ExportDropdown.svelte';
	import Pagination from '$lib/components/common/Pagination.svelte';
	import { exportToPDF, type ExportColumn } from '$lib/utils/export.js';

	interface TransactionRecord {
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

	interface ReportResponse {
		data: TransactionRecord[];
		meta: { page: number; per_page: number; total: number };
		summary: ReportSummary;
	}

	let records = $state<TransactionRecord[]>([]);
	let summary = $state<ReportSummary>({
		total_credits: 0,
		total_debits: 0,
		net_flow: 0,
		total_tx_count: 0
	});

	let loading = $state(true);
	let generating = $state(false);
	let loadError = $state<string | null>(null);
	let notice = $state<{ type: 'success' | 'error'; text: string } | null>(null);
	let noticeTimer: ReturnType<typeof setTimeout> | undefined;

	let search = $state('');
	let directionFilter = $state('all');
	let fromDate = $state('');
	let untilDate = $state('');

	// Pagination
	let currentPage = $state(1);
	let perPage = $state(10);
	let totalRecords = $state(0);

	let requestSeq = 0;

	function showNotice(type: 'success' | 'error', text: string) {
		notice = { type, text };
		clearTimeout(noticeTimer);
		noticeTimer = setTimeout(() => (notice = null), 6000);
	}

	function filterQuery(): URLSearchParams {
		const query = new URLSearchParams();
		if (search.trim()) query.set('search', search.trim());
		if (directionFilter !== 'all') query.set('direction', directionFilter);
		if (fromDate) query.set('from', fromDate);
		if (untilDate) query.set('until', untilDate);
		return query;
	}

	function dateRangeError(): string | null {
		if (fromDate && untilDate && fromDate > untilDate) {
			return 'The "from" date must be on or before the "to" date.';
		}
		return null;
	}

	async function loadReports() {
		const invalid = dateRangeError();
		if (invalid) {
			loadError = invalid;
			return;
		}

		const seq = ++requestSeq;
		loading = true;
		loadError = null;
		try {
			const query = filterQuery();
			query.set('page', String(currentPage));
			query.set('per_page', String(perPage));

			const res = await supermanApi.get<ReportResponse>(`/api/v1/superman/billing/reports?${query.toString()}`);
			if (seq !== requestSeq) return;

			records = res.data || [];
			totalRecords = res.meta?.total || 0;
			if (res.summary) summary = res.summary;
		} catch (err) {
			if (seq !== requestSeq) return;
			loadError = err instanceof Error ? err.message : 'Failed to load billing reports.';
		} finally {
			if (seq === requestSeq) loading = false;
		}
	}

	function applyFilters() {
		currentPage = 1;
		void loadReports();
	}

	function goToPage(page: number) {
		currentPage = page;
		void loadReports();
	}

	function changePerPage(size: number) {
		perPage = size;
		currentPage = 1;
		void loadReports();
	}

	async function fetchAllRecords(): Promise<ReportResponse> {
		const query = filterQuery();
		query.set('export', '1');
		return supermanApi.get<ReportResponse>(`/api/v1/superman/billing/reports?${query.toString()}`);
	}

	function filterSummaryText(): string {
		const parts: string[] = [];
		if (search.trim()) parts.push(`search "${search.trim()}"`);
		if (directionFilter !== 'all') parts.push(directionFilter === 'credit' ? 'credits only' : 'debits only');
		if (fromDate || untilDate) parts.push(`period ${fromDate || 'beginning'} to ${untilDate || 'today'}`);
		return parts.length > 0 ? parts.join(', ') : 'none (all movements)';
	}

	function formatBdt(val: number): string {
		return 'BDT ' + (val || 0).toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 });
	}

	/** Builds the full audit report for the current filters and downloads it as a PDF. */
	async function generateReport() {
		if (generating) return;
		const invalid = dateRangeError();
		if (invalid) {
			showNotice('error', invalid);
			return;
		}

		generating = true;
		notice = null;
		try {
			const res = await fetchAllRecords();
			const rows = res.data || [];
			if (rows.length === 0) {
				showNotice('error', 'No transactions match the selected filters, so there is nothing to report.');
				return;
			}

			const totals = res.summary ?? summary;
			exportToPDF(exportColumns, rows, {
				title: 'Financial & Billing Audit Report',
				filterSummary: filterSummaryText(),
				summary: [
					{ label: 'Total credits', value: `+${formatBdt(totals.total_credits)}` },
					{ label: 'Total debits', value: `-${formatBdt(totals.total_debits)}` },
					{ label: 'Net movement', value: formatBdt(totals.net_flow) },
					{ label: 'Transactions', value: totals.total_tx_count.toLocaleString() }
				]
			});

			showNotice('success', `Report generated: ${rows.length.toLocaleString()} transactions downloaded as PDF.`);
			void loadReports();
		} catch (err) {
			showNotice('error', err instanceof Error ? err.message : 'Unable to generate the report.');
		} finally {
			generating = false;
		}
	}

	const exportColumns: ExportColumn<TransactionRecord>[] = [
		{ header: 'TX ID', getValue: (r) => `#${r.id}` },
		{ header: 'Date', getValue: (r) => r.created_at },
		{ header: 'Customer', getValue: (r) => r.customer_name },
		{ header: 'Phone', getValue: (r) => r.customer_phone || '—' },
		{ header: 'Type', getValue: (r) => (r.direction === 'credit' ? 'Credit' : 'Debit') },
		{ header: 'Amount', getValue: (r) => `${r.direction === 'credit' ? '+' : '-'}${formatBdt(r.amount)}` },
		{ header: 'Balance Before', getValue: (r) => formatBdt(r.balance_before) },
		{ header: 'Balance After', getValue: (r) => formatBdt(r.balance_after) },
		{ header: 'Description', getValue: (r) => r.description || '—' }
	];

	onMount(() => {
		void loadReports();
	});

	function formatCurrency(val: number): string {
		return '৳' + (val || 0).toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 });
	}
</script>

<svelte:head>
	<title>Financial & Billing Audit Reports — SMS Whiteinfo Administrator</title>
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
				<span class="text-xs font-semibold text-gray-900 dark:text-white">Reports</span>
			</div>
			<h1 class="mt-1 text-xl font-bold tracking-tight text-gray-900 dark:text-white sm:text-2xl">
				Financial & Billing Audit Reports
			</h1>
			<p class="text-xs text-gray-500 dark:text-gray-400">
				Detailed ledger transactions audit trail, custom period balance movements, and downloadable financial reports.
			</p>
		</div>

		<div class="flex items-center gap-2">
			<ExportDropdown
				title="Financial and Billing Audit Reports"
				columns={exportColumns}
				getData={async () => (await fetchAllRecords()).data || []}
				disabled={totalRecords === 0}
			/>
			<a
				href="/superman/billing"
				class="inline-flex items-center gap-1.5 rounded-lg bg-emerald-600 px-3.5 py-2 text-xs font-semibold text-white shadow-xs hover:bg-emerald-500"
			>
				<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8c-1.657 0-3 .895-3 2s1.343 2 3 2 3 .895 3 2-1.343 2-3 2m0-8c1.11 0 2.08.402 2.599 1M12 8V7m0 1v8m0 0v1m0-1c-1.11 0-2.08-.402-2.599-1M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
				</svg>
				Billing Dashboard
			</a>
		</div>
	</div>

	<!-- Summary Totals -->
	<div class="grid grid-cols-2 gap-3 sm:grid-cols-4">
		<div class="rounded-xl border border-gray-200 bg-white p-4 shadow-xs dark:border-gray-800 dark:bg-gray-900">
			<span class="text-[11px] font-medium text-gray-400 uppercase tracking-wider">Total Credits</span>
			<p class="mt-1 text-lg font-bold text-emerald-600 dark:text-emerald-400">+{formatCurrency(summary.total_credits)}</p>
			<p class="text-[10px] text-gray-400">Top-ups and manual additions</p>
		</div>

		<div class="rounded-xl border border-gray-200 bg-white p-4 shadow-xs dark:border-gray-800 dark:bg-gray-900">
			<span class="text-[11px] font-medium text-gray-400 uppercase tracking-wider">Total Debits</span>
			<p class="mt-1 text-lg font-bold text-rose-600 dark:text-rose-400">-{formatCurrency(summary.total_debits)}</p>
			<p class="text-[10px] text-gray-400">Outbound SMS consumption</p>
		</div>

		<div class="rounded-xl border border-gray-200 bg-white p-4 shadow-xs dark:border-gray-800 dark:bg-gray-900">
			<span class="text-[11px] font-medium text-gray-400 uppercase tracking-wider">Net Movement</span>
			<p class="mt-1 text-lg font-bold {summary.net_flow >= 0 ? 'text-emerald-600 dark:text-emerald-400' : 'text-rose-600'}">
				{formatCurrency(summary.net_flow)}
			</p>
			<p class="text-[10px] text-gray-400">Credits minus debits</p>
		</div>

		<div class="rounded-xl border border-gray-200 bg-white p-4 shadow-xs dark:border-gray-800 dark:bg-gray-900">
			<span class="text-[11px] font-medium text-gray-400 uppercase tracking-wider">Transactions Analyzed</span>
			<p class="mt-1 text-lg font-bold text-gray-900 dark:text-white">{summary.total_tx_count.toLocaleString()}</p>
			<p class="text-[10px] text-gray-400">Audit entries in query</p>
		</div>
	</div>

	{#if notice}
		<div
			role="status"
			data-testid="report-notice"
			class="flex items-center justify-between rounded-lg border px-4 py-3 text-xs font-medium {notice.type === 'success'
				? 'border-emerald-200 bg-emerald-50 text-emerald-800 dark:border-emerald-900 dark:bg-emerald-950/30 dark:text-emerald-300'
				: 'border-rose-200 bg-rose-50 text-rose-800 dark:border-rose-900 dark:bg-rose-950/30 dark:text-rose-300'}"
		>
			<span>{notice.text}</span>
			<button type="button" onclick={() => (notice = null)} class="ml-4 text-base leading-none opacity-60 hover:opacity-100" aria-label="Dismiss">×</button>
		</div>
	{/if}

	<!-- Filter Controls -->
	<div class="flex flex-col gap-3 rounded-xl border border-gray-200 bg-white p-3 shadow-xs dark:border-gray-800 dark:bg-gray-900 lg:flex-row lg:items-center lg:justify-between">
		<div class="relative max-w-sm flex-1">
			<svg class="pointer-events-none absolute left-3 top-2.5 h-4 w-4 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
				<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
			</svg>
			<input
				type="text"
				bind:value={search}
				onkeydown={(e) => {
					if (e.key === 'Enter') applyFilters();
				}}
				placeholder="Search customer, note, phone… (Enter)"
				class="w-full rounded-lg border border-gray-200 bg-gray-50 py-1.5 pl-9 pr-3 text-xs text-gray-900 focus:border-emerald-500 focus:bg-white focus:outline-hidden dark:border-gray-800 dark:bg-gray-800 dark:text-white"
			/>
		</div>

		<div class="flex flex-wrap items-center gap-2">
			<select
				bind:value={directionFilter}
				onchange={applyFilters}
				aria-label="Movement type"
				class="rounded-lg border border-gray-200 bg-white px-2.5 py-1.5 text-xs text-gray-700 focus:border-emerald-500 focus:outline-hidden dark:border-gray-800 dark:bg-gray-800 dark:text-gray-300"
			>
				<option value="all">All Movements</option>
				<option value="credit">Credit Only (+)</option>
				<option value="debit">Debit Only (-)</option>
			</select>

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
				onclick={() => void generateReport()}
				disabled={generating}
				data-testid="generate-report"
				class="inline-flex items-center gap-1.5 rounded-lg bg-gray-900 px-3.5 py-1.5 text-xs font-semibold text-white transition hover:bg-gray-800 disabled:cursor-wait disabled:opacity-70 dark:bg-white dark:text-gray-900 dark:hover:bg-gray-100"
			>
				{#if generating}
					<svg class="h-3.5 w-3.5 animate-spin" fill="none" viewBox="0 0 24 24"><circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle><path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"></path></svg>
					Generating…
				{:else}
					Generate Report
				{/if}
			</button>
		</div>
	</div>

	{#if loadError}
		<div class="rounded-lg border border-rose-200 bg-rose-50 p-3 text-xs text-rose-700 dark:border-rose-900 dark:bg-rose-950/30 dark:text-rose-300">
			{loadError}
		</div>
	{/if}

	<!-- Report Table -->
	<div class="overflow-hidden rounded-xl border border-gray-200 bg-white shadow-xs dark:border-gray-800 dark:bg-gray-900">
		<div class="overflow-x-auto">
			<table class="w-full text-left text-xs">
				<thead>
					<tr class="border-b border-gray-100 bg-gray-50/50 text-[10px] font-semibold uppercase tracking-wider text-gray-400 dark:border-gray-800 dark:bg-gray-800/40">
						<th class="py-3 px-4">Timestamp</th>
						<th class="py-3 px-3">Customer</th>
						<th class="py-3 px-3">Type</th>
						<th class="py-3 px-3 text-right">Amount</th>
						<th class="py-3 px-3 text-right">Balance Before</th>
						<th class="py-3 px-3 text-right">Balance After</th>
						<th class="py-3 px-4">Reference / Audit Details</th>
					</tr>
				</thead>
				<tbody class="divide-y divide-gray-100 dark:divide-gray-800">
					{#each records as r}
						<tr class="hover:bg-gray-50/50 dark:hover:bg-gray-800/30 transition-colors">
							<td class="py-3 px-4 font-mono text-[11px] text-gray-500 whitespace-nowrap">
								{r.created_at}
							</td>
							<td class="py-3 px-3">
								<p class="font-bold text-gray-900 dark:text-white">{r.customer_name}</p>
								<p class="font-mono text-[10px] text-gray-400">{r.customer_phone}</p>
							</td>
							<td class="py-3 px-3">
								<span class="inline-flex rounded-md px-1.5 py-0.5 text-[9px] font-bold uppercase tracking-wider {r.direction === 'credit' ? 'bg-emerald-100 text-emerald-800 dark:bg-emerald-950 dark:text-emerald-300' : 'bg-rose-100 text-rose-800 dark:bg-rose-950 dark:text-rose-300'}">
									{r.direction}
								</span>
							</td>
							<td class="py-3 px-3 text-right font-mono font-bold {r.direction === 'credit' ? 'text-emerald-600 dark:text-emerald-400' : 'text-rose-600 dark:text-rose-400'}">
								{r.direction === 'credit' ? '+' : '-'}{formatCurrency(r.amount)}
							</td>
							<td class="py-3 px-3 text-right font-mono text-gray-500">
								{formatCurrency(r.balance_before)}
							</td>
							<td class="py-3 px-3 text-right font-mono font-semibold text-gray-900 dark:text-white">
								{formatCurrency(r.balance_after)}
							</td>
							<td class="py-3 px-4 max-w-sm truncate text-gray-600 dark:text-gray-300" title={r.description || ''}>
								{r.description || '—'}
							</td>
						</tr>
					{:else}
						<tr>
							<td colspan="7" class="py-8 text-center text-xs text-gray-400">
								{loading ? 'Compiling financial audit reports…' : 'No records found matching filters.'}
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
</div>\n