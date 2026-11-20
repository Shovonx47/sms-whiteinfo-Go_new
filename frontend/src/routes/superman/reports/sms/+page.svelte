<script lang="ts">
	import { onMount } from 'svelte';
	import { supermanApi } from '$lib/superman-api.js';
	import Pagination from '$lib/components/common/Pagination.svelte';
	import { exportToExcel, exportToPDF, type ExportColumn } from '$lib/utils/export.js';

	interface MessageItem {
		id: number;
		customer: string;
		recipient: string;
		body: string;
		status: string;
		cost: number;
		parts: number;
		source: string;
		gateway?: string | null;
		sender_id?: string | null;
		sender_type?: string | null;
		bill_msisdn: string;
		sms_type: string;
		channel: string;
		operator: string;
		tariff: string;
		api_user?: string | null;
		source_ip: string;
		request_id: string;
		server_txn_id: string;
		server_response_code: string;
		mno_txn_id: string;
		mno_response_code: string;
		mno_response_message: string;
		previous_balance?: string | null;
		next_balance?: string | null;
		failure_reason: string;
		submit_time: string;
		delivery_time?: string | null;
		created_at: string;
	}

	interface Meta {
		page: number;
		per_page: number;
		total: number;
	}

	let messages = $state<MessageItem[]>([]);
	let meta = $state<Meta>({ page: 1, per_page: 10, total: 0 });
	let loading = $state(true);
	let error = $state<string | null>(null);
	let exporting = $state<'excel' | 'pdf' | null>(null);

	let search = $state('');
	let statusFilter = $state('');
	let sourceFilter = $state('');
	let perPageChoice = $state(10);
	let searchTimer: ReturnType<typeof setTimeout> | null = null;

	const columnCount = 25;

	function queryParams() {
		const params = new URLSearchParams();
		if (search.trim()) params.set('search', search.trim());
		if (statusFilter) params.set('status', statusFilter);
		if (sourceFilter) params.set('source', sourceFilter);
		return params;
	}

	async function loadMessages(targetPage = 1) {
		loading = true;
		error = null;
		try {
			const params = queryParams();
			params.set('page', String(targetPage));
			params.set('per_page', String(perPageChoice));

			const res = await supermanApi.get<{ data: MessageItem[]; meta?: Meta }>(
				`/api/v1/superman/messages?${params.toString()}`
			);
			messages = res.data ?? [];
			meta = res.meta ?? { page: targetPage, per_page: perPageChoice, total: messages.length };
		} catch (err: unknown) {
			error = err instanceof Error ? err.message : 'Failed to load SMS reports.';
			messages = [];
		} finally {
			loading = false;
		}
	}

	function applyFilters() {
		void loadMessages(1);
	}

	function handleSearchInput() {
		if (searchTimer) clearTimeout(searchTimer);
		searchTimer = setTimeout(() => void loadMessages(1), 350);
	}

	onMount(() => {
		void loadMessages(1);
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

	function money(value?: string | number | null): string {
		if (value === null || value === undefined || value === '') return '—';
		const parsed = Number(value);
		if (Number.isNaN(parsed)) return '—';
		return `BDT ${parsed.toFixed(2)}`;
	}

	function text(value?: string | null): string {
		return value && value.trim() !== '' ? value : '—';
	}

	function truncate(value: string, limit = 50): string {
		if (!value) return '—';
		return value.length > limit ? `${value.slice(0, limit)}…` : value;
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

	const exportColumns: ExportColumn<MessageItem>[] = [
		{ header: 'User Name', getValue: (m) => text(m.customer) },
		{ header: 'Recipient Number', getValue: (m) => m.recipient },
		{ header: 'Content', getValue: (m) => m.body },
		{ header: 'Sender ID', getValue: (m) => text(m.sender_id) },
		{ header: 'Bill MSISDN', getValue: (m) => text(m.bill_msisdn) },
		{ header: 'SMS Type', getValue: (m) => text(m.sms_type) },
		{ header: 'SMS Count', getValue: (m) => m.parts },
		{ header: 'Cost', getValue: (m) => money(m.cost) },
		{ header: 'Previous Balance', getValue: (m) => money(m.previous_balance) },
		{ header: 'Next Balance', getValue: (m) => money(m.next_balance) },
		{ header: 'Status', getValue: (m) => statusLabel(m.status) },
		{ header: 'Failure Reason', getValue: (m) => text(m.failure_reason) },
		{ header: 'Operator', getValue: (m) => text(m.operator) },
		{ header: 'Channel', getValue: (m) => text(m.channel) },
		{ header: 'Submit Time', getValue: (m) => dateTime(m.submit_time) },
		{ header: 'Delivery Time', getValue: (m) => dateTime(m.delivery_time) },
		{ header: 'Tariff', getValue: (m) => text(m.tariff) },
		{ header: 'API User', getValue: (m) => text(m.api_user) },
		{ header: 'Source IP', getValue: (m) => text(m.source_ip) },
		{ header: 'Request ID', getValue: (m) => text(m.request_id) },
		{ header: 'Server TxnId', getValue: (m) => text(m.server_txn_id) },
		{ header: 'ServerResponseCode', getValue: (m) => text(m.server_response_code) },
		{ header: 'MNO TxnId', getValue: (m) => text(m.mno_txn_id) },
		{ header: 'MNO Response Code', getValue: (m) => text(m.mno_response_code) },
		{ header: 'MNO Response Message', getValue: (m) => text(m.mno_response_message) }
	];

	async function getAllMessagesForExport(): Promise<MessageItem[]> {
		const params = queryParams();
		params.set('export', '1');
		const res = await supermanApi.get<{ data: MessageItem[] }>(
			`/api/v1/superman/messages?${params.toString()}`
		);
		return res.data ?? [];
	}

	async function runExport(kind: 'excel' | 'pdf') {
		if (exporting) return;
		exporting = kind;
		error = null;
		try {
			const rows = await getAllMessagesForExport();
			if (rows.length === 0) {
				error = 'No records available to export.';
				return;
			}
			if (kind === 'excel') {
				exportToExcel(exportColumns, rows, { title: 'SMS Reports' });
			} else {
				exportToPDF(exportColumns, rows, { title: 'SMS Reports' });
			}
		} catch (err: unknown) {
			error = err instanceof Error ? err.message : 'Export failed.';
		} finally {
			exporting = null;
		}
	}
</script>

<svelte:head>
	<title>SMS Reports — Superman Administrator</title>
</svelte:head>

<div class="space-y-5">
	<div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
		<h1 class="text-2xl font-bold tracking-tight text-gray-950 dark:text-white">SMS Reports</h1>
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
				<select
					bind:value={sourceFilter}
					onchange={applyFilters}
					aria-label="Source filter"
					class="rounded-lg border border-gray-200 bg-white px-3 py-1.5 text-xs text-gray-700 outline-none focus:border-emerald-500 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-300"
				>
					<option value="">All Sources</option>
					<option value="api">API</option>
					<option value="web">Web</option>
				</select>
			</div>
			<div class="relative w-full sm:w-64">
				<svg class="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-gray-400" fill="none" viewBox="0 0 24 24" stroke-width="1.8" stroke="currentColor">
					<path stroke-linecap="round" stroke-linejoin="round" d="m21 21-4.35-4.35M17 10.5a6.5 6.5 0 1 1-13 0 6.5 6.5 0 0 1 13 0Z" />
				</svg>
				<input
					type="search"
					bind:value={search}
					oninput={handleSearchInput}
					onkeydown={(event) => {
						if (event.key === 'Enter') {
							event.preventDefault();
							applyFilters();
						}
					}}
					placeholder="Search"
					aria-label="Search SMS reports"
					class="w-full rounded-lg border border-gray-200 bg-white py-1.5 pl-9 pr-3 text-xs text-gray-800 outline-none placeholder:text-gray-400 focus:border-emerald-500 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-200"
				/>
			</div>
		</div>

		{#if error}
			<div class="border-b border-rose-100 bg-rose-50 px-4 py-2 text-xs text-rose-700 dark:border-rose-900 dark:bg-rose-950/40 dark:text-rose-300">
				{error}
			</div>
		{/if}

		<div class="overflow-x-auto">
			<table class="w-full min-w-[2500px] border-collapse text-left text-[13px] text-gray-700 dark:text-gray-300">
				<thead>
					<tr class="border-b border-gray-200 bg-white text-[10px] font-bold uppercase tracking-wider text-gray-500 dark:border-gray-800 dark:bg-gray-900 dark:text-gray-400">
						<th class="whitespace-nowrap px-4 py-3.5">User Name</th>
						<th class="whitespace-nowrap px-4 py-3.5">Recipient Number</th>
						<th class="whitespace-nowrap px-4 py-3.5">Content</th>
						<th class="whitespace-nowrap px-4 py-3.5">Sender ID</th>
						<th class="whitespace-nowrap px-4 py-3.5">Bill MSISDN</th>
						<th class="whitespace-nowrap px-4 py-3.5">SMS Type</th>
						<th class="whitespace-nowrap px-4 py-3.5">SMS Count</th>
						<th class="whitespace-nowrap px-4 py-3.5">Cost</th>
						<th class="whitespace-nowrap px-4 py-3.5">Previous Balance</th>
						<th class="whitespace-nowrap px-4 py-3.5">Next Balance</th>
						<th class="whitespace-nowrap px-4 py-3.5">Status</th>
						<th class="whitespace-nowrap px-4 py-3.5">Failure Reason</th>
						<th class="whitespace-nowrap px-4 py-3.5">Operator</th>
						<th class="whitespace-nowrap px-4 py-3.5">Channel</th>
						<th class="whitespace-nowrap px-4 py-3.5">Submit Time</th>
						<th class="whitespace-nowrap px-4 py-3.5">Delivery Time</th>
						<th class="whitespace-nowrap px-4 py-3.5">Tariff</th>
						<th class="whitespace-nowrap px-4 py-3.5">API User</th>
						<th class="whitespace-nowrap px-4 py-3.5">Source IP</th>
						<th class="whitespace-nowrap px-4 py-3.5">Request ID</th>
						<th class="whitespace-nowrap px-4 py-3.5">Server TxnId</th>
						<th class="whitespace-nowrap px-4 py-3.5">ServerResponseCode</th>
						<th class="whitespace-nowrap px-4 py-3.5">MNO TxnId</th>
						<th class="whitespace-nowrap px-4 py-3.5">MNO Response Code</th>
						<th class="whitespace-nowrap px-4 py-3.5">MNO Response Message</th>
					</tr>
				</thead>
				<tbody class="divide-y divide-gray-100 dark:divide-gray-800 {loading ? 'opacity-60' : ''}">
					{#if loading && messages.length === 0}
						<tr>
							<td colspan={columnCount} class="px-4 py-16 text-center text-xs text-gray-500">
								<svg class="mr-2 inline h-4 w-4 animate-spin" fill="none" viewBox="0 0 24 24"><circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle><path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"></path></svg>
								Loading SMS reports…
							</td>
						</tr>
					{:else if messages.length === 0}
						<tr>
							<td colspan={columnCount} class="px-4 py-16 text-center text-xs text-gray-500">No SMS reports found.</td>
						</tr>
					{:else}
						{#each messages as m (m.id)}
							<tr class="align-middle transition-colors hover:bg-gray-50/60 dark:hover:bg-gray-800/30">
								<td class="whitespace-nowrap px-4 py-4 font-semibold text-gray-950 dark:text-white">{text(m.customer)}</td>
								<td class="whitespace-nowrap px-4 py-4 text-gray-800 dark:text-gray-200">+{m.recipient.replace(/^\+/, '')}</td>
								<td class="max-w-[280px] truncate px-4 py-4 text-gray-600 dark:text-gray-400" title={m.body}>{truncate(m.body)}</td>
								<td class="whitespace-nowrap px-4 py-4">
									{#if m.sender_id}
										<span class="inline-flex rounded-md border border-emerald-200 bg-emerald-50 px-2 py-0.5 text-[11px] font-medium text-emerald-700 dark:border-emerald-900 dark:bg-emerald-950/40 dark:text-emerald-300">{m.sender_id}</span>
									{:else}
										<span class="text-gray-400">—</span>
									{/if}
								</td>
								<td class="whitespace-nowrap px-4 py-4 text-gray-700 dark:text-gray-300">{text(m.bill_msisdn)}</td>
								<td class="whitespace-nowrap px-4 py-4">
									{#if m.sms_type}
										<span class="inline-flex h-6 w-6 items-center justify-center rounded-md border border-emerald-200 bg-emerald-50 text-[11px] font-semibold text-emerald-700 dark:border-emerald-900 dark:bg-emerald-950/40 dark:text-emerald-300">{m.sms_type}</span>
									{:else}
										<span class="text-gray-400">—</span>
									{/if}
								</td>
								<td class="whitespace-nowrap px-4 py-4 text-gray-700 dark:text-gray-300">{m.parts ?? '—'}</td>
								<td class="whitespace-nowrap px-4 py-4 text-gray-700 dark:text-gray-300">{money(m.cost)}</td>
								<td class="whitespace-nowrap px-4 py-4 text-gray-700 dark:text-gray-300">{money(m.previous_balance)}</td>
								<td class="whitespace-nowrap px-4 py-4 text-gray-700 dark:text-gray-300">{money(m.next_balance)}</td>
								<td class="whitespace-nowrap px-4 py-4">
									<span class="inline-flex items-center gap-1 rounded-full border px-2.5 py-0.5 text-[11px] font-semibold {statusClasses(m.status)}">
										{#if m.status === 'delivered'}
											<svg class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="M9 12.75 11.25 15 15 9.75M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z" /></svg>
										{:else if m.status === 'sent'}
											<svg class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="M6 12 3.269 3.125A59.769 59.769 0 0 1 21.485 12 59.768 59.768 0 0 1 3.27 20.875L5.999 12Zm0 0h7.5" /></svg>
										{:else if m.status === 'pending' || m.status === 'sending'}
											<svg class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="M12 6v6h4.5M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z" /></svg>
										{:else}
											<svg class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" d="M9.75 9.75 14.25 14.25M14.25 9.75 9.75 14.25M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z" /></svg>
										{/if}
										{statusLabel(m.status)}
									</span>
								</td>
								<td class="max-w-[240px] truncate px-4 py-4 text-gray-600 dark:text-gray-400" title={m.failure_reason || ''}>{text(m.failure_reason)}</td>
								<td class="whitespace-nowrap px-4 py-4 text-gray-700 dark:text-gray-300">{text(m.operator)}</td>
								<td class="whitespace-nowrap px-4 py-4">
									{#if m.channel}
										<span class="inline-flex rounded-md border border-emerald-200 bg-emerald-50 px-2 py-0.5 text-[11px] font-medium text-emerald-700 dark:border-emerald-900 dark:bg-emerald-950/40 dark:text-emerald-300">{m.channel}</span>
									{:else}
										<span class="text-gray-400">—</span>
									{/if}
								</td>
								<td class="whitespace-nowrap px-4 py-4 text-gray-700 dark:text-gray-300">{dateTime(m.submit_time)}</td>
								<td class="whitespace-nowrap px-4 py-4 text-gray-700 dark:text-gray-300">{dateTime(m.delivery_time)}</td>
								<td class="whitespace-nowrap px-4 py-4 text-gray-700 dark:text-gray-300">{text(m.tariff)}</td>
								<td class="whitespace-nowrap px-4 py-4 text-gray-700 dark:text-gray-300">{text(m.api_user)}</td>
								<td class="whitespace-nowrap px-4 py-4 font-mono text-[12px] text-gray-600 dark:text-gray-400">{text(m.source_ip)}</td>
								<td class="whitespace-nowrap px-4 py-4 font-mono text-[12px] text-gray-600 dark:text-gray-400">{text(m.request_id)}</td>
								<td class="whitespace-nowrap px-4 py-4 font-mono text-[12px] text-gray-600 dark:text-gray-400">{text(m.server_txn_id)}</td>
								<td class="whitespace-nowrap px-4 py-4 text-gray-700 dark:text-gray-300">{text(m.server_response_code)}</td>
								<td class="whitespace-nowrap px-4 py-4 font-mono text-[12px] text-gray-600 dark:text-gray-400">{text(m.mno_txn_id)}</td>
								<td class="whitespace-nowrap px-4 py-4 text-gray-700 dark:text-gray-300">{text(m.mno_response_code)}</td>
								<td class="max-w-[220px] truncate px-4 py-4 text-gray-600 dark:text-gray-400" title={m.mno_response_message || ''}>{text(m.mno_response_message)}</td>
							</tr>
						{/each}
					{/if}
				</tbody>
			</table>
		</div>

		<Pagination
			total={meta.total}
			page={meta.page}
			perPage={perPageChoice}
			{loading}
			onPageChange={(p) => void loadMessages(p)}
			onPerPageChange={(size) => {
				perPageChoice = size;
				void loadMessages(1);
			}}
		/>
	</div>
</div>
