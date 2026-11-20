<script lang="ts">
	import { onMount } from 'svelte';
	import { supermanApi } from '$lib/superman-api.js';
	import Pagination from '$lib/components/common/Pagination.svelte';
	import { exportToExcel, exportToPDF, type ExportColumn } from '$lib/utils/export.js';

	interface CustomerItem {
		id: number;
		name: string;
		email: string;
		phone: string;
		balance: number;
		messages_count: number;
		messages_spend: number;
		created_at: string;
	}

	let customers = $state<CustomerItem[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let exporting = $state<'excel' | 'pdf' | null>(null);
	let search = $state('');
	let currentPage = $state(1);
	let perPage = $state(10);
	let meta = $state({ page: 1, per_page: 10, total: 0 });
	let searchTimer: ReturnType<typeof setTimeout> | null = null;

	function queryParams() {
		const params = new URLSearchParams();
		if (search.trim()) params.set('search', search.trim());
		return params;
	}

	async function loadCustomers(targetPage = 1) {
		currentPage = targetPage;
		loading = true;
		error = null;
		try {
			const params = queryParams();
			params.set('page', String(targetPage));
			params.set('per_page', String(perPage));

			const res = await supermanApi.get<{ data: CustomerItem[]; meta?: { page: number; per_page: number; total: number } }>(
				`/api/v1/superman/customers?${params.toString()}`
			);
			customers = res.data ?? [];
			meta = res.meta ?? { page: targetPage, per_page: perPage, total: customers.length };
		} catch (err: unknown) {
			error = err instanceof Error ? err.message : 'Failed to load customer reports.';
			customers = [];
		} finally {
			loading = false;
		}
	}

	function handleSearch() {
		if (searchTimer) clearTimeout(searchTimer);
		searchTimer = setTimeout(() => void loadCustomers(1), 350);
	}

	onMount(() => {
		void loadCustomers(1);
		return () => {
			if (searchTimer) clearTimeout(searchTimer);
		};
	});

	function money(value?: number | null): string {
		const parsed = Number(value ?? 0);
		return `BDT ${(Number.isNaN(parsed) ? 0 : parsed).toFixed(2)}`;
	}

	function joined(value?: string | null): string {
		if (!value) return '—';
		const parsed = new Date(value);
		if (Number.isNaN(parsed.getTime())) return '—';
		return parsed.toLocaleDateString('en-US', { month: 'short', day: '2-digit', year: 'numeric' });
	}

	const exportColumns: ExportColumn<CustomerItem>[] = [
		{ header: 'Customer', getValue: (row) => row.name },
		{ header: 'Email', getValue: (row) => row.email },
		{ header: 'SMS', getValue: (row) => row.messages_count ?? 0 },
		{ header: 'SMS Spend', getValue: (row) => money(row.messages_spend) },
		{ header: 'Balance', getValue: (row) => money(row.balance) },
		{ header: 'Joined', getValue: (row) => joined(row.created_at) }
	];

	async function getAllForExport(): Promise<CustomerItem[]> {
		const params = queryParams();
		params.set('export', '1');
		const res = await supermanApi.get<{ data: CustomerItem[] }>(`/api/v1/superman/customers?${params.toString()}`);
		return res.data ?? [];
	}

	async function runExport(kind: 'excel' | 'pdf') {
		if (exporting) return;
		exporting = kind;
		error = null;
		try {
			const rows = await getAllForExport();
			if (rows.length === 0) {
				error = 'No records available to export.';
				return;
			}
			if (kind === 'excel') {
				exportToExcel(exportColumns, rows, { title: 'Customer Report' });
			} else {
				exportToPDF(exportColumns, rows, { title: 'Customer Report' });
			}
		} catch (err: unknown) {
			error = err instanceof Error ? err.message : 'Export failed.';
		} finally {
			exporting = null;
		}
	}
</script>

<svelte:head>
	<title>Customer Report — Superman Administrator</title>
</svelte:head>

<div class="space-y-5">
	<div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
		<h1 class="text-2xl font-bold tracking-tight text-gray-950 dark:text-white">Customer Report</h1>
		<div class="flex items-center gap-2">
			<button
				type="button"
				onclick={() => void runExport('excel')}
				disabled={exporting !== null}
				class="inline-flex items-center gap-1.5 rounded-lg border border-emerald-500 bg-white px-3 py-2 text-xs font-semibold text-emerald-700 shadow-xs hover:bg-emerald-50 disabled:cursor-not-allowed disabled:opacity-60 dark:border-emerald-600 dark:bg-gray-900 dark:text-emerald-400 dark:hover:bg-emerald-950/40"
			>
				<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke-width="1.6" stroke="currentColor">
					<path stroke-linecap="round" stroke-linejoin="round" d="M3 16.5v2.25A2.25 2.25 0 0 0 5.25 21h13.5A2.25 2.25 0 0 0 21 18.75V16.5M16.5 12 12 16.5m0 0L7.5 12m4.5 4.5V3" />
				</svg>
				{exporting === 'excel' ? 'Exporting…' : 'Export Excel'}
			</button>
			<button
				type="button"
				onclick={() => void runExport('pdf')}
				disabled={exporting !== null}
				class="inline-flex items-center gap-1.5 rounded-lg border border-gray-300 bg-white px-3 py-2 text-xs font-semibold text-gray-700 shadow-xs hover:bg-gray-50 disabled:cursor-not-allowed disabled:opacity-60 dark:border-gray-700 dark:bg-gray-900 dark:text-gray-300 dark:hover:bg-gray-800"
			>
				<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke-width="1.6" stroke="currentColor">
					<path stroke-linecap="round" stroke-linejoin="round" d="M19.5 14.25v-2.625a3.375 3.375 0 0 0-3.375-3.375h-1.5A1.125 1.125 0 0 1 13.5 7.125v-1.5A3.375 3.375 0 0 0 10.125 2.25H8.25m2.25 0H5.625c-.621 0-1.125.504-1.125 1.125v17.25c0 .621.504 1.125 1.125 1.125h12.75c.621 0 1.125-.504 1.125-1.125V11.25a9 9 0 0 0-9-9Z" />
				</svg>
				{exporting === 'pdf' ? 'Exporting…' : 'Export PDF'}
			</button>
		</div>
	</div>

	<div class="overflow-hidden rounded-xl border border-gray-200 bg-white shadow-xs dark:border-gray-800 dark:bg-gray-900">
		<div class="flex items-center justify-end border-b border-gray-100 px-4 py-3 dark:border-gray-800">
			<div class="relative w-full sm:w-64">
				<svg class="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-gray-400" fill="none" viewBox="0 0 24 24" stroke-width="1.8" stroke="currentColor">
					<path stroke-linecap="round" stroke-linejoin="round" d="m21 21-4.35-4.35M17 10.5a6.5 6.5 0 1 1-13 0 6.5 6.5 0 0 1 13 0Z" />
				</svg>
				<input
					type="search"
					bind:value={search}
					oninput={handleSearch}
					placeholder="Search"
					aria-label="Search customers"
					class="w-full rounded-lg border border-gray-200 bg-white py-1.5 pl-9 pr-3 text-xs text-gray-800 outline-none placeholder:text-gray-400 focus:border-emerald-500 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-200"
				/>
			</div>
		</div>

		{#if error}
			<div class="border-b border-rose-100 bg-rose-50 px-4 py-2 text-xs text-rose-700 dark:border-rose-900 dark:bg-rose-950/40 dark:text-rose-300">{error}</div>
		{/if}

		<div class="overflow-x-auto">
			<table class="w-full text-left text-[13px] text-gray-700 dark:text-gray-300">
				<thead>
					<tr class="border-b border-gray-200 text-[10px] font-bold uppercase tracking-wider text-gray-500 dark:border-gray-800 dark:text-gray-400">
						<th class="whitespace-nowrap px-4 py-3.5">Customer</th>
						<th class="whitespace-nowrap px-4 py-3.5">Email</th>
						<th class="whitespace-nowrap px-4 py-3.5">SMS</th>
						<th class="whitespace-nowrap px-4 py-3.5">SMS Spend</th>
						<th class="whitespace-nowrap px-4 py-3.5">Balance</th>
						<th class="whitespace-nowrap px-4 py-3.5">Joined</th>
					</tr>
				</thead>
				<tbody class="divide-y divide-gray-100 dark:divide-gray-800 {loading ? 'opacity-60' : ''}">
					{#if loading && customers.length === 0}
						<tr><td colspan={6} class="px-4 py-16 text-center text-xs text-gray-500">Loading customer report…</td></tr>
					{:else if customers.length === 0}
						<tr><td colspan={6} class="px-4 py-16 text-center text-xs text-gray-500">No customer records found.</td></tr>
					{:else}
						{#each customers as row (row.id)}
							<tr class="transition-colors hover:bg-gray-50/60 dark:hover:bg-gray-800/30">
								<td class="whitespace-nowrap px-4 py-4 font-semibold text-gray-950 dark:text-white">{row.name}</td>
								<td class="whitespace-nowrap px-4 py-4 text-gray-600 dark:text-gray-400">{row.email || '—'}</td>
								<td class="whitespace-nowrap px-4 py-4 text-orange-600 dark:text-orange-400">{row.messages_count ?? 0}</td>
								<td class="whitespace-nowrap px-4 py-4 text-gray-800 dark:text-gray-200">{money(row.messages_spend)}</td>
								<td class="whitespace-nowrap px-4 py-4 text-gray-800 dark:text-gray-200">{money(row.balance)}</td>
								<td class="whitespace-nowrap px-4 py-4 text-gray-700 dark:text-gray-300">{joined(row.created_at)}</td>
							</tr>
						{/each}
					{/if}
				</tbody>
			</table>
		</div>

		<Pagination
			total={meta.total}
			page={meta.page}
			{perPage}
			{loading}
			onPageChange={(page) => void loadCustomers(page)}
			onPerPageChange={(size) => {
				perPage = size;
				void loadCustomers(1);
			}}
		/>
	</div>
</div>
