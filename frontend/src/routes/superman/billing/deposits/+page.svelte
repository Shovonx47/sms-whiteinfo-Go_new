<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { supermanApi } from '$lib/superman-api.js';
	import ExportDropdown from '$lib/components/common/ExportDropdown.svelte';
	import type { ExportColumn } from '$lib/utils/export.js';

	interface Deposit {
		id: number;
		user_id: number;
		customer: string;
		customer_email: string;
		customer_phone: string;
		company: string;
		amount: number;
		currency: string;
		payment_method: string;
		gateway: string | null;
		transaction_id: string | null;
		deposited_by: string | null;
		status: string;
		paid_at: string | null;
		created_at: string;
	}

	interface FinanceTotals {
		add: number;
		minus: number;
		net: number;
		transactions: number;
	}

	interface FinanceChannel {
		key: string;
		label: string;
		add: number;
		minus: number;
	}

	interface FinanceDay {
		date: string;
		add: number;
		minus: number;
		net: number;
		transactions: number;
	}

	interface FinanceReport {
		period: string;
		from: string;
		to: string;
		totals: FinanceTotals;
		channels: FinanceChannel[];
		daily: FinanceDay[];
	}

	const periods = [
		{ key: 'today', label: 'Today' },
		{ key: 'yesterday', label: 'Yesterday' },
		{ key: '7days', label: 'Last 7 Days' },
		{ key: 'week', label: 'This Week' },
		{ key: 'month', label: 'This Month' },
		{ key: 'custom', label: '▶ Custom Date' }
	];

	let deposits = $state<Deposit[]>([]);
	let loading = $state(true);
	let search = $state('');
	let statusFilter = $state('all');
	let methodFilter = $state('all');

	// Row action menu
	let openMenuId = $state<number | null>(null);
	let actionLoading = $state<number | null>(null);
	let notice = $state<{ type: 'success' | 'error'; text: string } | null>(null);

	// Pagination
	let currentPage = $state(1);
	let perPage = $state(10);
	let totalRecords = $state(0);
	let totalPages = $derived(Math.max(1, Math.ceil(totalRecords / perPage)));
	// Negative entries render as an ellipsis gap, matching the old pager.
	let pageNumbers = $derived.by(() => {
		if (totalPages <= 7) {
			return Array.from({ length: totalPages }, (_, index) => index + 1);
		}

		if (currentPage <= 4) {
			return [1, 2, 3, 4, -1, totalPages - 1, totalPages];
		}

		if (currentPage >= totalPages - 3) {
			return [1, 2, -1, totalPages - 3, totalPages - 2, totalPages - 1, totalPages];
		}

		return [1, -1, currentPage - 1, currentPage, currentPage + 1, -2, totalPages];
	});

	// Finance report
	let period = $state('today');
	let customFrom = $state(new Date().toISOString().slice(0, 10));
	let customTo = $state(new Date().toISOString().slice(0, 10));
	let finance = $state<FinanceReport | null>(null);
	let financeLoading = $state(true);
	let financeError = $state<string | null>(null);

	async function loadDeposits() {
		loading = true;
		try {
			const query = new URLSearchParams({
				page: String(currentPage),
				per_page: String(perPage)
			});
			if (search) query.set('search', search);
			if (statusFilter !== 'all') query.set('status', statusFilter);
			if (methodFilter !== 'all') query.set('method', methodFilter);

			const res = await supermanApi.get<{
				data: Deposit[];
				meta: { page: number; per_page: number; total: number };
			}>(`/api/v1/superman/deposits?${query.toString()}`);

			deposits = res.data || [];
			totalRecords = res.meta?.total || 0;
			currentPage = res.meta?.page || currentPage;
		} catch (err) {
			console.error('Failed to load deposits:', err);
			notice = { type: 'error', text: 'Could not load deposits. Please retry.' };
		} finally {
			loading = false;
		}
	}

	async function loadFinance() {
		financeLoading = true;
		financeError = null;
		try {
			const query = new URLSearchParams({ period });
			if (period === 'custom') {
				query.set('from', customFrom);
				query.set('to', customTo);
			}

			const res = await supermanApi.get<{ data: FinanceReport }>(
				`/api/v1/superman/deposits/finance-report?${query.toString()}`
			);
			finance = res.data;
		} catch (err) {
			console.error('Failed to load finance report:', err);
			financeError = 'Could not load the finance report for this period.';
		} finally {
			financeLoading = false;
		}
	}

	function selectPeriod(key: string) {
		period = key;
		if (key === 'custom') return;
		void loadFinance();
	}

	function applyCustomRange(event: SubmitEvent) {
		event.preventDefault();
		void loadFinance();
	}

	function goToPage(target: number) {
		if (target < 1 || target > totalPages || target === currentPage) return;
		currentPage = target;
		void loadDeposits();
	}

	function changePerPage(value: number) {
		perPage = value;
		currentPage = 1;
		void loadDeposits();
	}

	function applyFilters() {
		currentPage = 1;
		void loadDeposits();
	}

	async function approveDeposit(deposit: Deposit) {
		openMenuId = null;
		if (!window.confirm(`Approve deposit of ${formatCurrency(deposit.amount)} for ${deposit.customer}? This credits their balance immediately.`)) {
			return;
		}

		actionLoading = deposit.id;
		try {
			await supermanApi.post(`/api/v1/superman/deposits/${deposit.id}/approve`, {});
			notice = { type: 'success', text: `Deposit #${deposit.id} approved and credited.` };
			await Promise.all([loadDeposits(), loadFinance()]);
		} catch (err: unknown) {
			notice = { type: 'error', text: err instanceof Error ? err.message : 'Failed to approve deposit.' };
		} finally {
			actionLoading = null;
		}
	}

	async function rejectDeposit(deposit: Deposit) {
		openMenuId = null;
		if (!window.confirm(`Reject deposit #${deposit.id} of ${formatCurrency(deposit.amount)} for ${deposit.customer}?`)) {
			return;
		}

		actionLoading = deposit.id;
		try {
			await supermanApi.post(`/api/v1/superman/deposits/${deposit.id}/reject`, {});
			notice = { type: 'success', text: `Deposit #${deposit.id} rejected.` };
			await Promise.all([loadDeposits(), loadFinance()]);
		} catch (err: unknown) {
			notice = { type: 'error', text: err instanceof Error ? err.message : 'Failed to reject deposit.' };
		} finally {
			actionLoading = null;
		}
	}

	onMount(() => {
		const paramStatus = $page.url.searchParams.get('status');
		if (paramStatus) statusFilter = paramStatus;

		void loadDeposits();
		void loadFinance();
	});

	function formatCurrency(val: number | null | undefined): string {
		return '৳' + (val || 0).toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 });
	}

	function formatDay(value: string): string {
		const parsed = new Date(`${value}T00:00:00`);
		if (Number.isNaN(parsed.getTime())) return value;
		return parsed.toLocaleDateString('en-GB', { day: '2-digit', month: 'short', year: 'numeric' });
	}

	function formatDateTime(value: string | null): string {
		if (!value) return '—';
		return new Date(value).toLocaleString([], { dateStyle: 'medium', timeStyle: 'short' });
	}

	function methodLabel(value: string | null): string {
		const raw = (value || 'manual').toLowerCase();
		if (raw.includes('pipra')) return 'PipraPay';
		if (raw === 'manual' || raw === 'admin' || raw === 'bank') return 'Manual';
		return raw.charAt(0).toUpperCase() + raw.slice(1);
	}

	function methodChipClass(value: string | null): string {
		const raw = (value || 'manual').toLowerCase();
		if (raw.includes('pipra')) return 'bg-emerald-50 text-emerald-700 border-emerald-200 dark:bg-emerald-950/40 dark:text-emerald-300 dark:border-emerald-900';
		if (raw === 'manual' || raw === 'admin' || raw === 'bank') return 'bg-amber-50 text-amber-700 border-amber-200 dark:bg-amber-950/40 dark:text-amber-300 dark:border-amber-900';
		return 'bg-gray-50 text-gray-700 border-gray-200 dark:bg-gray-800 dark:text-gray-300 dark:border-gray-700';
	}

	function statusChipClass(status: string): string {
		if (status === 'completed') return 'bg-emerald-50 text-emerald-700 border-emerald-200 dark:bg-emerald-950/40 dark:text-emerald-300 dark:border-emerald-900';
		if (status === 'pending') return 'bg-amber-50 text-amber-700 border-amber-200 dark:bg-amber-950/40 dark:text-amber-300 dark:border-amber-900';
		return 'bg-rose-50 text-rose-700 border-rose-200 dark:bg-rose-950/40 dark:text-rose-300 dark:border-rose-900';
	}

	const exportColumns: ExportColumn<Deposit>[] = [
		{ header: 'Deposit ID', getValue: (d) => `#${d.id}` },
		{ header: 'Customer', getValue: (d) => d.customer },
		{ header: 'Email', getValue: (d) => d.customer_email || '—' },
		{ header: 'Phone', getValue: (d) => d.customer_phone || '—' },
		{ header: 'Company', getValue: (d) => d.company || '—' },
		{ header: 'Amount (BDT)', getValue: (d) => `${d.currency} ${d.amount.toFixed(2)}` },
		{ header: 'Method', getValue: (d) => methodLabel(d.payment_method) },
		{ header: 'Deposited By', getValue: (d) => d.deposited_by || '—' },
		{ header: 'Trx ID / Ref', getValue: (d) => d.transaction_id || '—' },
		{ header: 'Status', getValue: (d) => d.status.toUpperCase() },
		{ header: 'Paid At', getValue: (d) => (d.paid_at ? new Date(d.paid_at).toLocaleString() : '—') },
		{ header: 'Created', getValue: (d) => new Date(d.created_at).toLocaleString() }
	];
</script>

<svelte:window onclick={() => (openMenuId = null)} />

<svelte:head>
	<title>Deposits — SMS Whiteinfo Administrator</title>
</svelte:head>

<div class="space-y-5">
	<!-- Page Header -->
	<div class="space-y-2">
		<nav class="flex items-center gap-2 text-xs text-gray-500 dark:text-gray-400">
			<a href="/superman/billing" class="font-medium hover:text-emerald-600 dark:hover:text-emerald-400">Deposits</a>
			<span>›</span>
			<span class="font-semibold text-gray-900 dark:text-white">List</span>
		</nav>

		<div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
			<h1 class="text-2xl font-bold tracking-tight text-gray-950 dark:text-white">Deposits</h1>
			<div class="flex items-center gap-2">
				<ExportDropdown
					title="Customer Deposits"
					columns={exportColumns}
					getData={() => deposits}
					disabled={deposits.length === 0}
					class="inline-flex items-center gap-1.5 rounded-lg border border-emerald-600 bg-white px-3.5 py-2 text-xs font-semibold text-emerald-700 transition hover:bg-emerald-50 dark:border-emerald-500 dark:bg-gray-900 dark:text-emerald-400 dark:hover:bg-emerald-950/40"
				/>
			</div>
		</div>
	</div>

	{#if notice}
		<div class="rounded-lg border px-3 py-2 text-xs font-semibold {notice.type === 'success'
			? 'border-emerald-200 bg-emerald-50 text-emerald-800 dark:border-emerald-900 dark:bg-emerald-950/40 dark:text-emerald-300'
			: 'border-rose-200 bg-rose-50 text-rose-800 dark:border-rose-900 dark:bg-rose-950/40 dark:text-rose-300'}">
			{notice.text}
		</div>
	{/if}

	<!-- Finance Report -->
	<div class="overflow-hidden rounded-xl border border-gray-200 bg-white shadow-xs dark:border-gray-800 dark:bg-gray-900">
		<div class="border-b border-gray-100 px-5 py-4 dark:border-gray-800">
			<h2 class="text-base font-bold text-gray-900 dark:text-white">Finance Report</h2>
			<p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">Select reporting period</p>
		</div>

		<div class="space-y-4 p-5">
			<!-- Period tabs -->
			<div class="grid grid-cols-2 gap-2 md:grid-cols-3 xl:grid-cols-6">
				{#each periods as option (option.key)}
					<button
						type="button"
						onclick={() => selectPeriod(option.key)}
						class="rounded-lg border px-3 py-2 text-xs font-semibold transition {period === option.key
							? 'border-emerald-500 bg-emerald-50 text-emerald-700 dark:border-emerald-500 dark:bg-emerald-950/40 dark:text-emerald-300'
							: 'border-gray-200 bg-white text-gray-600 hover:bg-gray-50 dark:border-gray-700 dark:bg-gray-900 dark:text-gray-300 dark:hover:bg-gray-800'}"
					>
						{option.label}
					</button>
				{/each}
			</div>

			{#if period === 'custom'}
				<form onsubmit={applyCustomRange} class="flex flex-wrap items-end gap-2">
					<label class="text-xs font-semibold text-gray-600 dark:text-gray-300">
						From
						<input type="date" bind:value={customFrom} class="mt-1 block rounded-lg border border-gray-300 px-2 py-1.5 text-xs dark:border-gray-700 dark:bg-gray-800 dark:text-white" />
					</label>
					<label class="text-xs font-semibold text-gray-600 dark:text-gray-300">
						To
						<input type="date" bind:value={customTo} class="mt-1 block rounded-lg border border-gray-300 px-2 py-1.5 text-xs dark:border-gray-700 dark:bg-gray-800 dark:text-white" />
					</label>
					<button type="submit" class="rounded-lg bg-emerald-600 px-3 py-1.5 text-xs font-semibold text-white hover:bg-emerald-500">
						Apply
					</button>
				</form>
			{/if}

			{#if financeError}
				<div class="rounded-lg border border-rose-200 bg-rose-50 px-3 py-2 text-xs font-medium text-rose-700 dark:border-rose-900 dark:bg-rose-950/40 dark:text-rose-300">
					{financeError}
				</div>
			{/if}

			{#if finance}
				<!-- Totals -->
				<div class="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-4">
					<div class="rounded-lg border border-gray-200 px-4 py-3 dark:border-gray-800">
						<p class="text-[11px] text-gray-500 dark:text-gray-400">Total Add / Credit</p>
						<p class="mt-1 text-lg font-bold text-emerald-600 dark:text-emerald-400">{formatCurrency(finance.totals.add)}</p>
					</div>
					<div class="rounded-lg border border-gray-200 px-4 py-3 dark:border-gray-800">
						<p class="text-[11px] text-gray-500 dark:text-gray-400">Total Minus / Debit</p>
						<p class="mt-1 text-lg font-bold text-rose-600 dark:text-rose-400">{formatCurrency(finance.totals.minus)}</p>
					</div>
					<div class="rounded-lg border border-gray-200 px-4 py-3 dark:border-gray-800">
						<p class="text-[11px] text-gray-500 dark:text-gray-400">Net Amount</p>
						<p class="mt-1 text-lg font-bold text-emerald-600 dark:text-emerald-400">{formatCurrency(finance.totals.net)}</p>
					</div>
					<div class="rounded-lg border border-gray-200 px-4 py-3 dark:border-gray-800">
						<p class="text-[11px] text-gray-500 dark:text-gray-400">Transactions</p>
						<p class="mt-1 text-lg font-bold text-gray-900 dark:text-white">{finance.totals.transactions.toLocaleString()}</p>
					</div>
				</div>

				<!-- Channel split -->
				{#if finance.channels.length > 0}
					<div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
						{#each finance.channels as channel (channel.key)}
							<div class="flex items-center justify-between gap-3 rounded-lg border border-gray-200 px-4 py-3 dark:border-gray-800">
								<span class="text-[11px] font-bold uppercase tracking-wider text-gray-500 dark:text-gray-400">{channel.label}</span>
								<span class="flex items-center gap-4 font-mono text-xs font-semibold">
									<span class="text-emerald-600 dark:text-emerald-400">+{formatCurrency(channel.add)}</span>
									<span class="text-rose-600 dark:text-rose-400">-{formatCurrency(channel.minus)}</span>
								</span>
							</div>
						{/each}
					</div>
				{/if}

				<!-- Daily accounting -->
				<div class="overflow-hidden rounded-lg border border-gray-200 dark:border-gray-800">
					<div class="bg-gray-50 px-4 py-2 text-[10px] font-bold uppercase tracking-wider text-gray-500 dark:bg-gray-800/60 dark:text-gray-400">
						Daily Accounting
					</div>
					<div class="overflow-x-auto">
						<table class="w-full text-left text-xs">
							<thead>
								<tr class="border-b border-gray-100 text-[10px] font-semibold uppercase text-gray-400 dark:border-gray-800">
									<th class="px-4 py-2.5">Date</th>
									<th class="px-4 py-2.5 text-right">Add</th>
									<th class="px-4 py-2.5 text-right">Minus</th>
									<th class="px-4 py-2.5 text-right">Net</th>
									<th class="px-4 py-2.5 text-right">Transactions</th>
								</tr>
							</thead>
							<tbody class="divide-y divide-gray-100 dark:divide-gray-800">
								{#each finance.daily as day (day.date)}
									<tr class="hover:bg-gray-50/60 dark:hover:bg-gray-800/30">
										<td class="px-4 py-2.5 font-medium text-gray-900 dark:text-white">{formatDay(day.date)}</td>
										<td class="px-4 py-2.5 text-right font-mono text-emerald-600 dark:text-emerald-400">+{formatCurrency(day.add)}</td>
										<td class="px-4 py-2.5 text-right font-mono text-rose-600 dark:text-rose-400">-{formatCurrency(day.minus)}</td>
										<td class="px-4 py-2.5 text-right font-mono font-semibold text-emerald-600 dark:text-emerald-400">{formatCurrency(day.net)}</td>
										<td class="px-4 py-2.5 text-right font-mono text-gray-700 dark:text-gray-300">{day.transactions}</td>
									</tr>
								{:else}
									<tr>
										<td colspan="5" class="px-4 py-6 text-center text-xs text-gray-400">
											{financeLoading ? 'Loading report…' : 'No deposits recorded for this period.'}
										</td>
									</tr>
								{/each}
							</tbody>
						</table>
					</div>
				</div>
			{:else}
				<div class="py-10 text-center text-xs text-gray-400">{financeLoading ? 'Loading finance report…' : 'No finance report available.'}</div>
			{/if}
		</div>
	</div>

	<!-- Deposits Table -->
	<div class="overflow-hidden rounded-xl border border-gray-200 bg-white shadow-xs dark:border-gray-800 dark:bg-gray-900">
		<div class="flex flex-col gap-3 border-b border-gray-100 p-3 dark:border-gray-800 sm:flex-row sm:items-center sm:justify-between">
			<div class="flex flex-wrap items-center gap-2">
				<select
					bind:value={statusFilter}
					onchange={applyFilters}
					class="rounded-lg border border-gray-200 bg-white px-2.5 py-1.5 text-xs text-gray-700 focus:border-emerald-500 focus:outline-hidden dark:border-gray-700 dark:bg-gray-800 dark:text-gray-300"
				>
					<option value="all">All Statuses</option>
					<option value="completed">Completed</option>
					<option value="pending">Pending</option>
					<option value="failed">Failed</option>
					<option value="rejected">Rejected</option>
				</select>

				<select
					bind:value={methodFilter}
					onchange={applyFilters}
					class="rounded-lg border border-gray-200 bg-white px-2.5 py-1.5 text-xs text-gray-700 focus:border-emerald-500 focus:outline-hidden dark:border-gray-700 dark:bg-gray-800 dark:text-gray-300"
				>
					<option value="all">All Methods</option>
					<option value="manual">Manual / Admin</option>
					<option value="piprapay">PipraPay</option>
					<option value="bkash">bKash</option>
					<option value="nagad">Nagad</option>
					<option value="rocket">Rocket</option>
				</select>

				<button
					type="button"
					onclick={() => {
						void loadDeposits();
						void loadFinance();
					}}
					class="rounded-lg border border-gray-200 px-2.5 py-1.5 text-xs font-semibold text-gray-700 hover:bg-gray-50 dark:border-gray-700 dark:text-gray-300 dark:hover:bg-gray-800"
				>
					Refresh
				</button>
			</div>

			<div class="relative w-full sm:w-64">
				<svg class="pointer-events-none absolute left-2.5 top-1/2 h-4 w-4 -translate-y-1/2 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
					<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
				</svg>
				<input
					type="text"
					bind:value={search}
					onkeydown={(e) => {
						if (e.key === 'Enter') applyFilters();
					}}
					placeholder="Search customer, phone, trx id…"
					class="w-full rounded-lg border border-gray-200 bg-white py-1.5 pl-9 pr-3 text-xs text-gray-900 focus:border-emerald-500 focus:outline-hidden dark:border-gray-700 dark:bg-gray-800 dark:text-white"
				/>
			</div>
		</div>

		<div class="overflow-x-auto">
			<table class="w-full text-left text-xs">
				<thead>
					<tr class="border-b border-gray-100 bg-gray-50/50 text-[10px] font-semibold uppercase tracking-wider text-gray-400 dark:border-gray-800 dark:bg-gray-800/40">
						<th class="px-4 py-3">Customer</th>
						<th class="px-3 py-3">Amount</th>
						<th class="px-3 py-3">Method</th>
						<th class="px-3 py-3">Deposited By</th>
						<th class="px-3 py-3">Status</th>
						<th class="px-3 py-3">Transaction ID</th>
						<th class="px-3 py-3">Paid At</th>
						<th class="px-3 py-3">Email</th>
						<th class="px-3 py-3">Phone</th>
						<th class="px-3 py-3">Company</th>
						<th class="px-4 py-3 text-right">Action</th>
					</tr>
				</thead>
				<tbody class="divide-y divide-gray-100 dark:divide-gray-800">
					{#each deposits as d (d.id)}
						<tr class="transition-colors hover:bg-gray-50/50 dark:hover:bg-gray-800/30">
							<td class="px-4 py-3">
								<p class="font-semibold text-gray-900 dark:text-white">{d.customer}</p>
								<p class="text-[10px] text-gray-400">
									{d.customer_phone || d.customer_email || '—'} · {formatDateTime(d.created_at)}
								</p>
							</td>
							<td class="px-3 py-3 font-mono font-semibold text-gray-900 dark:text-white">
								{d.currency} {d.amount.toFixed(2)}
							</td>
							<td class="px-3 py-3">
								<span class="inline-flex items-center gap-1 rounded-md border px-2 py-0.5 text-[10px] font-semibold {methodChipClass(d.payment_method)}">
									<span class="h-1 w-1 rounded-full bg-current"></span>
									{methodLabel(d.payment_method)}
								</span>
							</td>
							<td class="px-3 py-3 text-gray-600 dark:text-gray-300">{d.deposited_by || '—'}</td>
							<td class="px-3 py-3">
								<span class="inline-flex items-center gap-1 rounded-full border px-2 py-0.5 text-[10px] font-semibold capitalize {statusChipClass(d.status)}">
									{d.status === 'completed' ? '✓' : d.status === 'pending' ? '◷' : '✕'}
									{d.status}
								</span>
							</td>
							<td class="max-w-40 truncate px-3 py-3 font-mono text-[11px] text-gray-600 dark:text-gray-300" title={d.transaction_id || ''}>{d.transaction_id || '—'}</td>
							<td class="px-3 py-3 text-gray-400">{d.paid_at ? formatDateTime(d.paid_at) : '—'}</td>
							<td class="px-3 py-3 text-gray-600 dark:text-gray-300">{d.customer_email || '—'}</td>
							<td class="px-3 py-3 text-gray-600 dark:text-gray-300">{d.customer_phone || '—'}</td>
							<td class="px-3 py-3 text-gray-600 dark:text-gray-300">{d.company || '—'}</td>
							<td class="px-4 py-3 text-right">
								<div class="relative inline-block">
									<button
										type="button"
										onclick={(e) => {
											e.stopPropagation();
											openMenuId = openMenuId === d.id ? null : d.id;
										}}
										disabled={actionLoading === d.id}
										class="inline-flex h-7 w-7 items-center justify-center rounded-lg border border-gray-200 text-gray-500 hover:bg-gray-50 disabled:opacity-50 dark:border-gray-700 dark:text-gray-400 dark:hover:bg-gray-800"
										title="Deposit actions"
									>
										<svg class="h-4 w-4" fill="currentColor" viewBox="0 0 20 20">
											<circle cx="10" cy="4" r="1.6" />
											<circle cx="10" cy="10" r="1.6" />
											<circle cx="10" cy="16" r="1.6" />
										</svg>
									</button>

									{#if openMenuId === d.id}
										<div class="absolute right-0 z-20 mt-1 w-36 overflow-hidden rounded-lg border border-gray-200 bg-white text-left shadow-lg dark:border-gray-700 dark:bg-gray-900">
											<a
												href="/superman/billing/deposits/{d.id}"
												class="block px-3 py-2 text-xs font-medium text-gray-700 hover:bg-gray-50 dark:text-gray-300 dark:hover:bg-gray-800"
											>
												View
											</a>
											{#if d.status === 'pending'}
												<button
													type="button"
													onclick={(e) => {
														e.stopPropagation();
														void approveDeposit(d);
													}}
													class="block w-full px-3 py-2 text-left text-xs font-medium text-emerald-700 hover:bg-emerald-50 dark:text-emerald-400 dark:hover:bg-emerald-950/40"
												>
													Approve
												</button>
												<button
													type="button"
													onclick={(e) => {
														e.stopPropagation();
														void rejectDeposit(d);
													}}
													class="block w-full px-3 py-2 text-left text-xs font-medium text-rose-700 hover:bg-rose-50 dark:text-rose-400 dark:hover:bg-rose-950/40"
												>
													Reject
												</button>
											{/if}
										</div>
									{/if}
								</div>
							</td>
						</tr>
					{:else}
						<tr>
							<td colspan="7" class="px-4 py-10 text-center text-xs text-gray-400">
								{loading ? 'Loading deposits…' : 'No deposits found matching criteria.'}
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>

		<!-- Pagination -->
		<div class="flex flex-col gap-3 border-t border-gray-100 px-4 py-3 sm:flex-row sm:items-center sm:justify-between dark:border-gray-800">
			<p class="text-xs text-gray-500 dark:text-gray-400">
				Showing {totalRecords === 0 ? 0 : (currentPage - 1) * perPage + 1} to {Math.min(currentPage * perPage, totalRecords)} of {totalRecords} results
			</p>

			<div class="flex flex-wrap items-center gap-3">
				<div class="flex items-center gap-1.5">
					<span class="text-xs text-gray-500 dark:text-gray-400">Per page</span>
					<select
						value={perPage}
						onchange={(e) => changePerPage(Number(e.currentTarget.value))}
						class="rounded-lg border border-gray-200 bg-white px-2 py-1 text-xs text-gray-700 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-300"
					>
						{#each [10, 25, 50] as size (size)}
							<option value={size}>{size}</option>
						{/each}
					</select>
				</div>

				<div class="flex items-center gap-1">
					{#each pageNumbers as pageNumber, index (index)}
						{#if pageNumber < 0}
							<span class="px-1.5 text-xs text-gray-400">…</span>
						{:else}
							<button
								type="button"
								onclick={() => goToPage(pageNumber)}
								disabled={loading}
								class="min-w-7 rounded-md border px-2.5 py-1 text-xs font-semibold transition {pageNumber === currentPage
									? 'border-emerald-600 bg-emerald-50 text-emerald-700 dark:border-emerald-500 dark:bg-emerald-950/40 dark:text-emerald-300'
									: 'border-gray-200 text-gray-700 hover:bg-gray-50 dark:border-gray-700 dark:text-gray-300 dark:hover:bg-gray-800'}"
							>
								{pageNumber}
							</button>
						{/if}
					{/each}
					<button
						type="button"
						onclick={() => goToPage(currentPage + 1)}
						disabled={currentPage >= totalPages || loading}
						class="inline-flex h-7 w-7 items-center justify-center rounded-md border border-gray-200 text-gray-600 hover:bg-gray-50 disabled:opacity-40 dark:border-gray-700 dark:text-gray-300 dark:hover:bg-gray-800"
						title="Next page"
					>
						›
					</button>
				</div>
			</div>
		</div>
	</div>
</div>
