<script lang="ts">
	import { onMount } from 'svelte';
	import { supermanApi } from '$lib/superman-api.js';
	import Pagination from '$lib/components/common/Pagination.svelte';
	import { exportToExcel, exportToPDF, type ExportColumn } from '$lib/utils/export.js';

	interface Props {
		title: string;
		senderCategory: 'masking' | 'non_masking' | '';
	}

	let { title, senderCategory }: Props = $props();

	interface MessageRow {
		id: number;
		customer: string;
		recipient: string;
		body: string;
		status: string;
		cost: number;
		parts: number;
		gateway?: string | null;
		sender_id?: string | null;
		failure_reason: string;
		submit_time: string;
		created_at: string;
	}

	let messages = $state<MessageRow[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let exporting = $state<'excel' | 'pdf' | null>(null);

	let search = $state('');
	let statusFilter = $state('');
	let dateFrom = $state('');
	let dateTo = $state('');
	let currentPage = $state(1);
	let perPage = $state(10);
	let meta = $state({ page: 1, per_page: 10, total: 0 });
	let searchTimer: ReturnType<typeof setTimeout> | null = null;

	const columnCount = 9;

	function queryParams() {
		const params = new URLSearchParams();
		if (senderCategory) params.set('sender_category', senderCategory);
		if (search.trim()) params.set('search', search.trim());
		if (statusFilter) params.set('status', statusFilter);
		if (dateFrom) params.set('date_from', dateFrom);
		if (dateTo) params.set('date_to', dateTo);
		return params;
	}

	async function loadReport(targetPage = 1) {
		currentPage = targetPage;
		loading = true;
		error = null;
		try {
			const params = queryParams();
			params.set('page', String(targetPage));
			params.set('per_page', String(perPage));

			const res = await supermanApi.get<{ data: MessageRow[]; meta?: { page: number; per_page: number; total: number } }>(
				`/api/v1/superman/messages?${params.toString()}`
			);
			messages = res.data ?? [];
			meta = res.meta ?? { page: targetPage, per_page: perPage, total: messages.length };
		} catch (err: unknown) {
			error = err instanceof Error ? err.message : `Failed to load ${title.toLowerCase()}.`;
			messages = [];
		} finally {
			loading = false;
		}
	}

	function applyFilters() {
		void loadReport(1);
	}

	function handleSearch() {
		if (searchTimer) clearTimeout(searchTimer);
		searchTimer = setTimeout(applyFilters, 350);
	}

	onMount(() => {
		void loadReport(1);
		return () => {
			if (searchTimer) clearTimeout(searchTimer);
		};
	});

	function dateTime(value?: string | null): string {
		if (!value) return '—';
		const parsed = new Date(value);
		if (Number.isNaN(parsed.getTime())) return '—';
		return parsed.toLocaleString('en-US', {
			month: 'short',
			day: '2-digit',
			year: 'numeric',
			hour: '2-digit',
			minute: '2-digit',
			second: '2-digit',
			hour12: false
		});
	}

	function charged(value: number): string {
		const parsed = Number(value);
		return `BDT ${(Number.isNaN(parsed) ? 0 : parsed).toFixed(2)}`;
	}

	function text(value?: string | null): string {
		return value && value.trim() !== '' ? value : '—';
	}

	function statusLabel(status: string): string {
		switch (status) {
			case 'delivered':
				return 'Delivered';
			case 'sent':
				return 'Sent';
			case 'pending':
				return 'Pending';
			case 'sending':
				return 'Sending';
			case 'dlr_failed':
				return 'DLR Failed';
			default:
				return 'Failed';
		}
	}

	function statusClasses(status: string): string {
		switch (status) {
			case 'delivered':
				return 'border-emerald-200 bg-emerald-50 text-emerald-700 dark:border-emerald-900 dark:bg-emerald-950/40 dark:text-emerald-300';
			case 'sent':
				return 'border-blue-200 bg-blue-50 text-blue-700 dark:border-blue-900 dark:bg-blue-950/40 dark:text-blue-300';
			case 'pending':
			case 'sending':
				return 'border-amber-200 bg-amber-50 text-amber-700 dark:border-amber-900 dark:bg-amber-950/40 dark:text-amber-300';
			default:
				return 'border-rose-200 bg-rose-50 text-rose-700 dark:border-rose-900 dark:bg-rose-950/40 dark:text-rose-300';
		}
	}

	const exportColumns: ExportColumn<MessageRow>[] = [
		{ header: 'Customer', getValue: (row) => text(row.customer) },
		{ header: 'Recipient', getValue: (row) => row.recipient },
		{ header: 'Sender ID', getValue: (row) => text(row.sender_id) },
		{ header: 'Provider', getValue: (row) => text(row.gateway) },
		{ header: 'SMS Parts', getValue: (row) => row.parts },
		{ header: 'Charged', getValue: (row) => charged(row.cost) },
		{ header: 'Status', getValue: (row) => statusLabel(row.status) },
		{ header: 'Failure Reason', getValue: (row) => text(row.failure_reason) },
		{ header: 'Submitted', getValue: (row) => dateTime(row.submit_time) }
	];

	async function getAllForExport(): Promise<MessageRow[]> {
		const params = queryParams();
		params.set('export', '1');
		const res = await supermanApi.get<{ data: MessageRow[] }>(
			`/api/v1/superman/messages?${params.toString()}`
		);
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
				exportToExcel(exportColumns, rows, { title });
			} else {
				exportToPDF(exportColumns, rows, { title });
			}
		} catch (err: unknown) {
			error = err instanceof Error ? err.message : 'Export failed.';
		} finally {
			exporting = null;
		}
	}
</script>

<div class="space-y-5">
	<div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
		<h1 class="text-2xl font-bold tracking-tight text-gray-950 dark:text-white">{title}</h1>
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
		<div class="flex flex-col gap-3 border-b border-gray-100 px-4 py-3 sm:flex-row sm:items-center sm:justify-between dark:border-gray-800">
			<div class="flex flex-wrap items-center gap-2">
				<select
					bind:value={statusFilter}
					onchange={applyFilters}
					aria-label="Status filter"
					class="rounded-lg border border-gray-200 bg-white px-3 py-1.5 text-xs text-gray-700 outline-none focus:border-emerald-500 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-300"
				>
					<option value="">All Statuses</option>
					<option value="delivered">Delivered</option>
					<option value="sent">Sent</option>
					<option value="pending">Pending</option>
					<option value="sending">Sending</option>
					<option value="failed">Failed</option>
					<option value="dlr_failed">DLR Failed</option>
				</select>
				<input
					type="date"
					bind:value={dateFrom}
					onchange={applyFilters}
					aria-label="From date"
					class="rounded-lg border border-gray-200 bg-white px-2 py-1.5 text-xs text-gray-700 outline-none focus:border-emerald-500 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-300"
				/>
				<input
					type="date"
					bind:value={dateTo}
					onchange={applyFilters}
					aria-label="Until date"
					class="rounded-lg border border-gray-200 bg-white px-2 py-1.5 text-xs text-gray-700 outline-none focus:border-emerald-500 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-300"
				/>
			</div>
			<div class="relative w-full sm:w-64">
				<svg class="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-gray-400" fill="none" viewBox="0 0 24 24" stroke-width="1.8" stroke="currentColor">
					<path stroke-linecap="round" stroke-linejoin="round" d="m21 21-4.35-4.35M17 10.5a6.5 6.5 0 1 1-13 0 6.5 6.5 0 0 1 13 0Z" />
				</svg>
				<input
					type="search"
					bind:value={search}
					oninput={handleSearch}
					onkeydown={(event) => {
						if (event.key === 'Enter') {
							event.preventDefault();
							applyFilters();
						}
					}}
					placeholder="Search"
					aria-label="Search report"
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
						<th class="whitespace-nowrap px-4 py-3.5">Recipient</th>
						<th class="whitespace-nowrap px-4 py-3.5">Sender ID</th>
						<th class="whitespace-nowrap px-4 py-3.5">Provider</th>
						<th class="whitespace-nowrap px-4 py-3.5">SMS Parts</th>
						<th class="whitespace-nowrap px-4 py-3.5">Charged</th>
						<th class="whitespace-nowrap px-4 py-3.5">Status</th>
						<th class="whitespace-nowrap px-4 py-3.5">Failure Reason</th>
						<th class="whitespace-nowrap px-4 py-3.5">Submitted</th>
					</tr>
				</thead>
				<tbody class="divide-y divide-gray-100 dark:divide-gray-800 {loading ? 'opacity-60' : ''}">
					{#if loading && messages.length === 0}
						<tr>
							<td colspan={columnCount} class="px-4 py-16 text-center text-xs text-gray-500">
								<svg class="mr-2 inline h-4 w-4 animate-spin" fill="none" viewBox="0 0 24 24"><circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle><path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"></path></svg>
								Loading {title.toLowerCase()}…
							</td>
						</tr>
					{:else if messages.length === 0}
						<tr>
							<td colspan={columnCount} class="px-4 py-16 text-center text-xs text-gray-500">No records found.</td>
						</tr>
					{:else}
						{#each messages as row (row.id)}
							<tr class="align-middle transition-colors hover:bg-gray-50/60 dark:hover:bg-gray-800/30">
								<td class="whitespace-nowrap px-4 py-4 font-semibold text-gray-950 dark:text-white">{text(row.customer)}</td>
								<td class="whitespace-nowrap px-4 py-4 text-gray-800 dark:text-gray-200">+{row.recipient.replace(/^\+/, '')}</td>
								<td class="whitespace-nowrap px-4 py-4">
									{#if row.sender_id}
										<span class="inline-flex rounded-md border border-emerald-200 bg-emerald-50 px-2 py-0.5 text-[11px] font-medium text-emerald-700 dark:border-emerald-900 dark:bg-emerald-950/40 dark:text-emerald-300">{row.sender_id}</span>
									{:else}
										<span class="text-gray-400">—</span>
									{/if}
								</td>
								<td class="whitespace-nowrap px-4 py-4 text-gray-700 dark:text-gray-300">{text(row.gateway)}</td>
								<td class="whitespace-nowrap px-4 py-4 text-gray-700 dark:text-gray-300">{row.parts ?? '—'}</td>
								<td class="whitespace-nowrap px-4 py-4 text-gray-700 dark:text-gray-300">{charged(row.cost)}</td>
								<td class="whitespace-nowrap px-4 py-4">
									<span class="inline-flex items-center gap-1 rounded-full border px-2.5 py-0.5 text-[11px] font-semibold {statusClasses(row.status)}">
										{#if row.status === 'delivered'}
											<svg class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="M9 12.75 11.25 15 15 9.75M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z" /></svg>
										{:else if row.status === 'sent'}
											<svg class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="M6 12 3.269 3.125A59.769 59.769 0 0 1 21.485 12 59.768 59.768 0 0 1 3.27 20.875L5.999 12Zm0 0h7.5" /></svg>
										{:else if row.status === 'pending' || row.status === 'sending'}
											<svg class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="M12 6v6h4.5M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z" /></svg>
										{:else}
											<svg class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="M9.75 9.75 14.25 14.25M14.25 9.75 9.75 14.25M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z" /></svg>
										{/if}
										{statusLabel(row.status)}
									</span>
								</td>
								<td class="max-w-[320px] truncate px-4 py-4 text-gray-600 dark:text-gray-400" title={row.failure_reason || ''}>{text(row.failure_reason)}</td>
								<td class="whitespace-nowrap px-4 py-4 text-gray-700 dark:text-gray-300">{dateTime(row.submit_time ?? row.created_at)}</td>
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
			onPageChange={(page) => void loadReport(page)}
			onPerPageChange={(size) => {
				perPage = size;
				void loadReport(1);
			}}
		/>
	</div>
</div>
