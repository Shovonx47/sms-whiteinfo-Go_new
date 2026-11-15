<script lang="ts">
	import { api } from '$lib/api.js';
	import DataTable from '$lib/components/DataTable.svelte';
	import { onDestroy, onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { toast } from '$lib/stores/toast.svelte.js';

	interface Message {
		id: number;
		to: string;
		sender_id: string;
		body: string;
		status: string;
		parts: number;
		cost: string;
		operator: string;
		sms_type?: string;
		channel?: string;
		tariff_name?: string;
		previous_balance?: string | number | null;
		next_balance?: string | number | null;
		error_message?: string | null;
		source_ip?: string;
		request_id?: string;
		sent_at?: string | null;
		api_user?: string | null;
		failed_reason?: string | null;
		submitted_at: string;
		delivered_at: string | null;
		failed_at?: string | null;
	}

	interface Meta {
		total: number;
		per_page: number;
		current_page: number;
	}

	interface SenderOption {
		id: number;
		sender_id: string;
	}

	interface MessageDetail extends Message {
		uid?: string | null;
		msg_id: string;
		batch_id: number | null;
		sender_db_id: number | null;
		delivery_status: string;
		source: string;
		source_ip?: string;
		request_id?: string;
		error_message: string;
		created_at: string;
		updated_at: string;
		sent_at: string | null;
		failed_at: string | null;
	}

	let messages = $state<Message[]>([]);
	let senders = $state<SenderOption[]>([]);
	let meta = $state<Meta>({ total: 0, per_page: 10, current_page: 1 });
	let perPage = $state(10);
	let loading = $state(true);
	let search = $state('');
	let statusFilter = $state('');
	let senderFilter = $state('');
	let sortKey = $state('id');
	let sortDir = $state<'asc' | 'desc'>('desc');
	let dateFrom = $state('');
	let dateTo = $state('');
	let detail = $state<MessageDetail | null>(null);
	let detailLoading = $state(false);
	let exporting = $state(false);
	let showExportModal = $state(false);
	let exportFormat = $state<'csv' | 'pdf'>('csv');
	let pollTimer: ReturnType<typeof setInterval> | null = null;

	async function loadSenders() {
		try {
			const res = await api.get<{ data: SenderOption[] }>('/api/v1/customer/senders?per_page=100');
			senders = res.data ?? [];
		} catch {
			senders = [];
		}
	}

	async function openMessage(row: Message) {
		detailLoading = true;
		detail = { ...row, msg_id: `MSG-${String(row.id).padStart(6, '0')}`, batch_id: null, sender_db_id: null, delivery_status: '', source: '', error_message: '', created_at: row.submitted_at, updated_at: row.submitted_at, sent_at: null, failed_at: null };
		try {
			const res = await api.get<{ data: MessageDetail }>(`/api/v1/customer/messages/${row.id}`);
			detail = res.data;
		} catch (err) {
			detail = null;
			toast.error('Message', err instanceof Error ? err.message : 'Unable to load message details.');
		} finally {
			detailLoading = false;
		}
	}

	function openMessageById(id: number) {
		void openMessage({
			id,
			to: '',
			sender_id: '',
			body: '',
			status: '',
			parts: 0,
			cost: '0',
			operator: '',
			submitted_at: new Date().toISOString(),
			delivered_at: null
		});
	}

	async function exportMessages(format: 'csv' | 'pdf' = exportFormat) {
		exporting = true;
		try {
			const params = new URLSearchParams();
			params.set('format', format);
			if (search) params.set('search', search);
			if (senderFilter) params.set('sender_id', senderFilter);
			if (statusFilter) params.set('status', statusFilter);
			if (dateFrom) params.set('date_from', dateFrom);
			if (dateTo) params.set('date_to', dateTo);
			const blob = await api.download(`/api/v1/customer/messages/export?${params}`);
			const url = URL.createObjectURL(blob);
			const link = document.createElement('a');
			link.href = url;
			link.download = `sent-messages-${new Date().toISOString().slice(0, 10)}.${format}`;
			link.click();
			URL.revokeObjectURL(url);
			showExportModal = false;
			toast.success(
				'Export ready',
				`The ${format.toUpperCase()} report has been downloaded.`
			);
		} catch (err) {
			toast.error('Export failed', err instanceof Error ? err.message : 'Unable to export messages.');
		} finally {
			exporting = false;
		}
	}

	async function load(page = 1, silent = false) {
		if (!silent) loading = true;
		try {
			const params = new URLSearchParams({
				page: String(page),
				per_page: String(perPage),
				sort: sortKey,
				dir: sortDir
			});
			if (search) params.set('search', search);
			if (senderFilter) params.set('sender_id', senderFilter);
			if (statusFilter) params.set('status', statusFilter);
			if (dateFrom) params.set('date_from', dateFrom);
			if (dateTo) params.set('date_to', dateTo);

			const data = await api.get<{ data: Message[]; meta: Meta }>(
				`/api/v1/customer/messages?${params}`
			);
			messages = data.data;
			meta = data.meta;
			perPage = data.meta.per_page ?? perPage;
		} catch {
			if (!silent) messages = [];
		} finally {
			if (!silent) loading = false;
		}
	}

	onMount(() => {
		const sp = page.url.searchParams;
		const statusParam = sp.get('status');
		const f0 = sp.get('tableFilters[status][values][0]');
		const f1 = sp.get('tableFilters[status][values][1]');
		if (statusParam) {
			statusFilter = statusParam;
		} else if (f0) {
			statusFilter = f1 ? `${f0},${f1}` : f0;
		}
		loadSenders();
		load(1);
		const viewId = Number(sp.get('view'));
		if (Number.isInteger(viewId) && viewId > 0) openMessageById(viewId);
		pollTimer = setInterval(() => load(meta.current_page || 1, true), 30000);
	});

	onDestroy(() => {
		if (pollTimer) clearInterval(pollTimer);
	});

	function handleSort(key: string) {
		if (sortKey === key) {
			sortDir = sortDir === 'asc' ? 'desc' : 'asc';
		} else {
			sortKey = key;
			sortDir = 'asc';
		}
		load();
	}

	function handleSearch(val: string) {
		search = val;
		load(1);
	}

	function statusBadgeClass(row: Message): string {
		const map: Record<string, string> = {
			pending: 'badge-pending',
			sent: 'badge-sent',
			delivered: 'badge-delivered',
			failed: 'badge-failed',
			dlr_failed: 'badge-failed',
			processing: 'badge-processing'
		};
		return map[row.status] ?? 'badge-processing';
	}

	function formatDate(iso: string | null): string {
		if (!iso) return '—';
		return new Date(iso).toLocaleString('en-GB', {
			day: '2-digit',
			month: 'short',
			year: 'numeric',
			hour: '2-digit',
			minute: '2-digit',
			second: '2-digit',
			hour12: true
		});
	}

	function formatTime(iso: string | null, withSeconds = false): string {
		if (!iso) return '—';
		return new Date(iso).toLocaleTimeString('en-GB', {
			hour: '2-digit',
			minute: '2-digit',
			...(withSeconds ? { second: '2-digit' as const } : {}),
			hour12: true
		});
	}

	function money(value: string | number | null | undefined): string {
		if (value === null || value === undefined || value === '') return '—';
		return Number(value).toFixed(2);
	}

	const columns = [
		{
			key: 'id',
			label: 'Message ID',
			class: 'w-24 whitespace-nowrap',
			render: (row: Message) => `MSG-${String(row.id).padStart(6, '0')}`
		},
		{ key: 'to', label: 'Recipient Number', class: 'font-mono whitespace-nowrap' },
		{
			key: 'body',
			label: 'Content',
			class: 'max-w-[220px]',
			render: (row: Message) => (row.body?.length > 50 ? row.body.slice(0, 50) + '…' : row.body)
		},
		{ key: 'sender_id', label: 'Sender ID', render: (row: Message) => row.sender_id || '—' },
		{ key: 'sms_type', label: 'SMS Type', render: (row: Message) => row.sms_type || '—' },
		{ key: 'parts', label: 'SMS Count' },
		{ key: 'cost', label: 'Cost', money: true },
		{ key: 'previous_balance', label: 'Previous Balance', render: (row: Message) => money(row.previous_balance) },
		{ key: 'next_balance', label: 'Next Balance', render: (row: Message) => money(row.next_balance) },
		{
			key: 'status',
			label: 'Status',
			badge: true,
			badgeClass: statusBadgeClass
		},
		{ key: 'error_message', label: 'Failure Reason', render: (row: Message) => row.error_message || '—' },
		{ key: 'operator', label: 'Operator', render: (row: Message) => row.operator || '—' },
		{ key: 'channel', label: 'Channel', render: (row: Message) => row.channel || '—' },
		{
			key: 'submitted_at',
			label: 'Submit Time',
			render: (row: Message) => formatTime(row.sent_at ?? row.submitted_at)
		},
		{
			key: 'delivered_at',
			label: 'Delivery Time',
			render: (row: Message) => formatTime(row.delivered_at, true)
		},
		{ key: 'tariff_name', label: 'Tariff', render: (row: Message) => row.tariff_name || '—' },
		{ key: 'api_user', label: 'API User', render: (row: Message) => row.api_user || '—' },
		{ key: 'source_ip', label: 'Source IP', render: (row: Message) => row.source_ip || '—' },
		{ key: 'request_id', label: 'Request ID', render: (row: Message) => row.request_id || '—' }
	];
</script>

<svelte:head>
	<title>Sent Message Details - White INFO</title>
	<meta name="description" content="View all your sent SMS messages and delivery statuses." />
</svelte:head>

<div class="space-y-4">
	<div class="flex items-start justify-between gap-4">
		<div>
			<h2 class="text-lg font-semibold text-gray-950 dark:text-gray-100">Sent Message Details</h2>
			<p class="mt-0.5 text-sm text-gray-500">All outgoing messages and delivery reports.</p>
		</div>
		<div class="flex gap-2">
			<button
				type="button"
				onclick={() => (showExportModal = true)}
				disabled={exporting}
				class="flex items-center gap-1.5 rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm font-medium text-gray-700 shadow-sm transition hover:bg-gray-50 disabled:opacity-50 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-200"
			>
				<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
					<path stroke-linecap="round" stroke-linejoin="round" d="M3 16.5v2.25A2.25 2.25 0 0 0 5.25 21h13.5A2.25 2.25 0 0 0 21 18.75V16.5M16.5 12 12 16.5m0 0L7.5 12m4.5 4.5V3" />
				</svg>
				{exporting ? 'Exporting…' : 'Export As'}
			</button>
			<a
				href="/messaging/messages/quick"
				class="flex items-center gap-1.5 rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm font-medium text-gray-700 shadow-sm transition hover:bg-gray-50 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-200 dark:hover:bg-gray-700"
			>
				<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
					<path stroke-linecap="round" stroke-linejoin="round" d="M6 12 3.269 3.125A59.769 59.769 0 0 1 21.485 12 59.768 59.768 0 0 1 3.27 20.875L5.999 12Zm0 0h7.5" />
				</svg>
				Quick Message
			</a>
			<a
				href="/messaging/messages/smart"
				class="flex items-center gap-1.5 rounded-lg bg-teal-600 px-3 py-2 text-sm font-medium text-white shadow-sm transition hover:bg-teal-500"
			>
				<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
					<path stroke-linecap="round" stroke-linejoin="round" d="M9.813 15.904 9 18.75l-.813-2.846a4.5 4.5 0 0 0-3.09-3.09L2.25 12l2.846-.813a4.5 4.5 0 0 0 3.09-3.09L9 5.25l.813 2.846a4.5 4.5 0 0 0 3.09 3.09L15.75 12l-2.846.813a4.5 4.5 0 0 0-3.09 3.09Z" />
				</svg>
				Bulk Message
			</a>
		</div>
	</div>

	<!-- Filters -->
	<div class="flex flex-wrap gap-3">
		<select
			bind:value={senderFilter}
			onchange={() => load(1)}
			class="rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 focus:border-teal-600 focus:outline-none focus:ring-1 focus:ring-teal-600 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-200"
		>
			<option value="">All Senders</option>
			{#each senders as s (s.id)}
				<option value={String(s.id)}>{s.sender_id}</option>
			{/each}
		</select>
		<select
			bind:value={statusFilter}
			onchange={() => load(1)}
			class="rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 focus:border-teal-600 focus:outline-none focus:ring-1 focus:ring-teal-600 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-200"
		>
			<option value="">All Status</option>
			<option value="pending">Pending</option>
			<option value="sending">Sending</option>
			<option value="sent">Sent</option>
			<option value="delivered">Delivered</option>
			<option value="failed">Failed</option>
			<option value="dlr_failed">DLR Failed</option>
		</select>
		<input
			type="date"
			bind:value={dateFrom}
			onchange={() => load(1)}
			class="rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 focus:border-teal-600 focus:outline-none focus:ring-1 focus:ring-teal-600 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-200"
			placeholder="From date"
		/>
		<input
			type="date"
			bind:value={dateTo}
			onchange={() => load(1)}
			class="rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 focus:border-teal-600 focus:outline-none focus:ring-1 focus:ring-teal-600 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-200"
			placeholder="To date"
		/>
	</div>

	<DataTable
		{columns}
		rows={messages}
		{loading}
		total={meta.total}
		{perPage}
		perPageOptions={[10, 25, 50]}
		onPerPageChange={(value) => { perPage = value; load(1); }}
		currentPage={meta.current_page}
		onPageChange={(p) => load(p)}
		onSort={handleSort}
		{sortKey}
		{sortDir}
		bind:searchValue={search}
		onSearch={handleSearch}
		searchPlaceholder="Search by message ID, recipient, sender…"
		emptyMessage="No messages found matching your filters."
		onRowClick={openMessage}
		groupBy={(row) => row.status}
		collapsedGroups
	/>
</div>

{#if detail}
	<div class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4 backdrop-blur-xs">
		<div class="relative max-h-[90vh] w-full max-w-2xl overflow-y-auto rounded-2xl border border-gray-100 bg-white p-6 shadow-xl dark:border-gray-800 dark:bg-gray-900">
			<div class="flex items-center justify-between border-b border-gray-100 pb-3 dark:border-gray-800">
				<h3 class="text-base font-bold text-gray-900 dark:text-white">Sent Message Detail</h3>
				<button type="button" onclick={() => (detail = null)} class="text-gray-400 hover:text-gray-600">✕</button>
			</div>
			{#if detailLoading}
				<p class="py-8 text-center text-xs text-gray-400">Loading…</p>
			{:else}
				<div class="mt-4 grid gap-3 text-xs sm:grid-cols-2">
					{#each [{ label: 'UID', value: detail.uid || '—' }, { label: 'Sender', value: detail.sender_id || '—' }, { label: 'Recipient', value: detail.to }, { label: 'Status', value: detail.status }, { label: 'Cost', value: `BDT ${Number(detail.cost).toFixed(2)}` }, { label: 'Parts', value: String(detail.parts) }, ...(detail.sent_at ? [{ label: 'Sent At', value: formatDate(detail.sent_at) }] : []), ...((detail.error_message || detail.failed_reason) ? [{ label: 'Failed Reason', value: detail.error_message || detail.failed_reason || '' }] : []), { label: 'Created At', value: formatDate(detail.created_at) }, { label: 'Updated At', value: formatDate(detail.updated_at) }] as field}
						<div class="rounded-lg border border-gray-100 p-2.5 dark:border-gray-800">
							<p class="text-[10px] uppercase text-gray-400">{field.label}</p>
							<p class="mt-0.5 font-semibold text-gray-900 dark:text-white">{field.value}</p>
						</div>
					{/each}
				</div>
				<div class="mt-4">
					<p class="mb-1 text-[10px] uppercase text-gray-400">Body</p>
					<div class="whitespace-pre-wrap rounded-lg border border-gray-200 bg-gray-50 p-3 text-xs text-gray-800 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-200">{detail.body}</div>
				</div>
			{/if}
		</div>
	</div>
{/if}

{#if showExportModal}
	<div class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4 backdrop-blur-xs">
		<div class="w-full max-w-md rounded-2xl border border-gray-100 bg-white p-6 shadow-xl dark:border-gray-800 dark:bg-gray-900">
			<div class="flex items-center justify-between border-b border-gray-100 pb-3 dark:border-gray-800">
				<h3 class="text-base font-bold text-gray-900 dark:text-white">Export Messages</h3>
				<button type="button" onclick={() => (showExportModal = false)} class="text-gray-400 hover:text-gray-600">✕</button>
			</div>
			<p class="mt-2 text-xs text-gray-500">Choose an export format for your filtered sent message details.</p>
			<div class="mt-4 space-y-2.5">
				<label class="flex cursor-pointer items-start gap-3 rounded-xl border border-gray-200 p-3 hover:border-teal-500 dark:border-gray-700">
					<input type="radio" bind:group={exportFormat} value="csv" class="mt-0.5 text-teal-600 focus:ring-teal-500" />
					<div class="text-xs">
						<p class="font-semibold text-gray-900 dark:text-white">CSV (.csv)</p>
						<p class="text-gray-500">Full matching records for Excel or spreadsheet analysis.</p>
					</div>
				</label>
				<label class="flex cursor-pointer items-start gap-3 rounded-xl border border-gray-200 p-3 hover:border-teal-500 dark:border-gray-700">
					<input type="radio" bind:group={exportFormat} value="pdf" class="mt-0.5 text-teal-600 focus:ring-teal-500" />
					<div class="text-xs">
						<p class="font-semibold text-gray-900 dark:text-white">PDF (.pdf)</p>
						<p class="text-gray-500">Formatted printable report (limited to first 500 records).</p>
					</div>
				</label>
			</div>
			<div class="mt-5 flex justify-end gap-2">
				<button
					type="button"
					onclick={() => (showExportModal = false)}
					class="rounded-lg border border-gray-300 px-3 py-2 text-xs font-medium text-gray-700 hover:bg-gray-50 dark:border-gray-700 dark:text-gray-200"
				>
					Cancel
				</button>
				<button
					type="button"
					disabled={exporting}
					onclick={() => exportMessages(exportFormat)}
					class="rounded-lg bg-teal-600 px-4 py-2 text-xs font-semibold text-white hover:bg-teal-500 disabled:opacity-50"
				>
					{exporting ? 'Exporting…' : `Download ${exportFormat.toUpperCase()}`}
				</button>
			</div>
		</div>
	</div>
{/if}
