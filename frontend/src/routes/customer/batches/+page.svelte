<script lang="ts">
	import { api } from '$lib/api.js';
	import { toast } from '$lib/stores/toast.svelte.js';
	import { onMount } from 'svelte';

	interface MessageLog {
		id: number;
		to: string;
		body: string;
		sender_id: string;
		status: string;
		cost: string;
		parts: number;
		previous_balance: string;
		sms_type: string;
		created_at: string;
		sent_at?: string;
		delivered_at?: string;
		failed_reason?: string;
	}

	interface Sender {
		id: number;
		sender_id: string;
	}

	interface Meta {
		total: number;
		per_page: number;
		current_page: number;
	}

	let logs = $state<MessageLog[]>([]);
	let senders = $state<Sender[]>([]);
	let meta = $state<Meta>({ total: 0, per_page: 10, current_page: 1 });
	let loading = $state(true);
	let perPage = $state(10);
	let currentPage = $state(1);

	// Filters
	let search = $state('');
	let selectedSender = $state('');
	let selectedStatus = $state('');
	let createdFrom = $state('');
	let createdUntil = $state('');

	// Export modal
	let exportModalOpen = $state(false);
	let exportFormat = $state<'excel' | 'pdf'>('excel');
	let exporting = $state(false);

	// View detail modal
	let selectedLog = $state<MessageLog | null>(null);

	async function loadSenders() {
		try {
			const res = await api.get<{ data: Sender[] }>('/api/v1/customer/senders');
			senders = res.data || [];
		} catch {
			senders = [];
		}
	}

	async function load(p = 1) {
		loading = true;
		currentPage = p;
		try {
			const params = new URLSearchParams({
				page: String(p),
				per_page: String(perPage)
			});
			if (search) params.set('search', search);
			if (selectedSender) params.set('sender_id', selectedSender);
			if (selectedStatus && selectedStatus !== 'all') params.set('status', selectedStatus);
			if (createdFrom) params.set('created_from', createdFrom);
			if (createdUntil) params.set('created_until', createdUntil);

			const res = await api.get<{ data: MessageLog[]; meta: Meta }>(`/api/v1/customer/batches?${params}`);
			logs = res.data || [];
			meta = res.meta || { total: 0, per_page: perPage, current_page: p };
		} catch {
			logs = [];
			meta = { total: 0, per_page: perPage, current_page: p };
		} finally {
			loading = false;
		}
	}

	function resetFilters() {
		search = '';
		selectedSender = '';
		selectedStatus = '';
		createdFrom = '';
		createdUntil = '';
		load(1);
	}

	async function handleExport() {
		if (exporting) return;
		exporting = true;
		try {
			const fmt = exportFormat === 'pdf' ? 'pdf' : 'csv';
			const params = new URLSearchParams({ format: fmt });
			if (search.trim()) params.set('search', search.trim());
			if (selectedSender) params.set('sender_id', selectedSender);
			if (selectedStatus && selectedStatus !== 'all') params.set('status', selectedStatus);
			if (createdFrom) params.set('created_from', createdFrom);
			if (createdUntil) params.set('created_until', createdUntil);

			const blob = await api.download(`/api/v1/customer/batches/export?${params}`);
			const url = URL.createObjectURL(blob);
			const link = document.createElement('a');
			link.href = url;
			link.download = `sent-message-logs-${new Date().toISOString().slice(0, 10)}.${fmt}`;
			link.click();
			URL.revokeObjectURL(url);
			exportModalOpen = false;
			toast.success('Export Complete', `Downloaded Sent Message Logs ${fmt.toUpperCase()}.`);
		} catch (err) {
			toast.error('Export Failed', err instanceof Error ? err.message : 'Could not export sent message logs.');
		} finally {
			exporting = false;
		}
	}

	onMount(() => {
		loadSenders();
		load(1);
	});

	let totalPages = $derived(Math.max(1, Math.ceil((meta.total || 0) / perPage)));

	// Pagination buttons calculator
	let pageButtons = $derived.by(() => {
		const pages: (number | string)[] = [];
		const total = totalPages;
		const cur = currentPage;

		if (total <= 7) {
			for (let i = 1; i <= total; i++) pages.push(i);
		} else {
			pages.push(1);
			if (cur > 3) pages.push('...');
			const start = Math.max(2, cur - 1);
			const end = Math.min(total - 1, cur + 1);
			for (let i = start; i <= end; i++) pages.push(i);
			if (cur < total - 2) pages.push('...');
			pages.push(total);
		}
		return pages;
	});
</script>

<svelte:head>
	<title>Sent Message Logs - White INFO</title>
</svelte:head>

<div class="space-y-6">
	<!-- Breadcrumb -->
	<nav class="flex items-center gap-2 text-xs font-medium text-gray-500">
		<a href="/messaging/batches" class="hover:text-gray-700">Sent Message Logs</a>
		<svg class="h-3 w-3 text-gray-400" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
			<path stroke-linecap="round" stroke-linejoin="round" d="m8.25 4.5 7.5 7.5-7.5 7.5" />
		</svg>
		<span class="text-gray-900 font-semibold">List</span>
	</nav>

	<!-- Page Heading & Export As button -->
	<div class="flex items-center justify-between">
		<h1 class="text-2xl font-bold tracking-tight text-gray-950">Sent Message Logs</h1>
		<button
			type="button"
			onclick={() => (exportModalOpen = true)}
			class="inline-flex items-center gap-2 rounded-lg border border-gray-300 bg-white px-3.5 py-2 text-xs font-semibold text-gray-700 shadow-sm transition hover:bg-gray-50 cursor-pointer"
		>
			<svg class="h-4 w-4 text-gray-500" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
				<path stroke-linecap="round" stroke-linejoin="round" d="M3 16.5v2.25A2.25 2.25 0 0 0 5.25 21h13.5A2.25 2.25 0 0 0 21 18.75V16.5M16.5 12 12 16.5m0 0L7.5 12m4.5 4.5V3" />
			</svg>
			Export As
		</button>
	</div>

	<!-- Main Table Container Card -->
	<div class="rounded-xl border border-gray-200 bg-white shadow-sm overflow-hidden">
		<!-- Filter Bar Above Content (Matching Screenshot 9 & 10) -->
		<div class="border-b border-gray-100 bg-white p-4">
			<div class="grid grid-cols-1 gap-4 sm:grid-cols-2 md:grid-cols-5 items-end">
				<!-- Sender filter -->
				<div>
					<label for="filter-sender" class="block text-xs font-semibold text-gray-700 mb-1">Sender</label>
					<select
						id="filter-sender"
						bind:value={selectedSender}
						onchange={() => load(1)}
						class="w-full rounded-lg border border-gray-300 bg-white py-1.5 px-3 text-xs text-gray-900 focus:border-teal-600 focus:outline-none focus:ring-1 focus:ring-teal-600"
					>
						<option value="">All Senders</option>
						{#each senders as s}
							<option value={s.sender_id}>{s.sender_id}</option>
						{/each}
					</select>
				</div>

				<!-- Status filter -->
				<div>
					<label for="filter-status" class="block text-xs font-semibold text-gray-700 mb-1">Status</label>
					<select
						id="filter-status"
						bind:value={selectedStatus}
						onchange={() => load(1)}
						class="w-full rounded-lg border border-gray-300 bg-white py-1.5 px-3 text-xs text-gray-900 focus:border-teal-600 focus:outline-none focus:ring-1 focus:ring-teal-600"
					>
						<option value="">All Status</option>
						<option value="sent">Sent</option>
						<option value="delivered">Delivered</option>
						<option value="pending">Pending</option>
						<option value="failed">Failed</option>
					</select>
				</div>

				<!-- Created From -->
				<div>
					<label for="filter-from" class="block text-xs font-semibold text-gray-700 mb-1">Created From</label>
					<input
						id="filter-from"
						type="date"
						bind:value={createdFrom}
						onchange={() => load(1)}
						class="w-full rounded-lg border border-gray-300 bg-white py-1.5 px-3 text-xs text-gray-900 focus:border-teal-600 focus:outline-none focus:ring-1 focus:ring-teal-600"
					/>
				</div>

				<!-- Created Until -->
				<div>
					<label for="filter-until" class="block text-xs font-semibold text-gray-700 mb-1">Created Until</label>
					<input
						id="filter-until"
						type="date"
						bind:value={createdUntil}
						onchange={() => load(1)}
						class="w-full rounded-lg border border-gray-300 bg-white py-1.5 px-3 text-xs text-gray-900 focus:border-teal-600 focus:outline-none focus:ring-1 focus:ring-teal-600"
					/>
				</div>

				<!-- Reset Button -->
				<div>
					<button
						type="button"
						onclick={resetFilters}
						class="w-full rounded-lg border border-red-200 bg-white py-1.5 px-3 text-xs font-semibold text-red-600 shadow-2xs hover:bg-red-50 transition cursor-pointer"
					>
						Reset
					</button>
				</div>
			</div>
		</div>

		<!-- Search Input Row -->
		<div class="flex items-center justify-end gap-2 border-b border-gray-100 p-3 bg-white">
			<div class="relative w-72">
				<div class="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-2.5">
					<svg class="h-4 w-4 text-gray-400" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
						<path stroke-linecap="round" stroke-linejoin="round" d="m21 21-5.197-5.197m0 0A7.5 7.5 0 1 0 5.196 5.196a7.5 7.5 0 0 0 10.607 10.607Z" />
					</svg>
				</div>
				<input
					type="text"
					bind:value={search}
					onkeydown={(e) => { if (e.key === 'Enter') load(1); }}
					placeholder="Search recipient or content..."
					class="w-full rounded-lg border border-gray-300 bg-white py-1.5 pr-3 pl-8 text-xs text-gray-900 placeholder-gray-400 focus:border-teal-600 focus:outline-none focus:ring-1 focus:ring-teal-600"
				/>
			</div>
			<button
				type="button"
				onclick={() => load(1)}
				class="rounded-lg bg-gray-100 px-3 py-1.5 text-xs font-semibold text-gray-700 hover:bg-gray-200 cursor-pointer"
			>
				Filter
			</button>
		</div>

		<!-- Table -->
		<div class="overflow-x-auto">
			<table class="w-full text-left border-collapse text-xs">
				<thead>
					<tr class="border-b border-gray-200 bg-gray-50/70 text-gray-600 font-semibold tracking-wider uppercase text-[11px]">
						<th class="py-3 px-4">RECIPIENT NUMBER</th>
						<th class="py-3 px-4">CONTENT</th>
						<th class="py-3 px-4">SENDER ID</th>
						<th class="py-3 px-4">SMS TYPE</th>
						<th class="py-3 px-4 text-right">COST</th>
						<th class="py-3 px-4 text-right">PREVIOUS BALANCE</th>
					</tr>
				</thead>
				<tbody class="divide-y divide-gray-100 bg-white">
					{#if loading}
						<tr>
							<td colspan="6" class="py-12 text-center text-gray-500">
								<div class="inline-block h-6 w-6 animate-spin rounded-full border-2 border-gray-300 border-t-teal-600"></div>
								<p class="mt-2 text-xs">Loading sent message logs...</p>
							</td>
						</tr>
					{:else if logs.length === 0}
						<tr>
							<td colspan="6" class="py-12 text-center text-gray-500">
								<p class="text-sm font-medium text-gray-700">No message logs found</p>
								<p class="mt-1 text-xs text-gray-400">Sent messages will appear here once dispatched.</p>
							</td>
						</tr>
					{:else}
						{#each logs as log (log.id)}
							<tr
								class="hover:bg-gray-50/70 transition cursor-pointer"
								onclick={() => (selectedLog = log)}
							>
								<!-- Recipient Number -->
								<td class="py-3 px-4 font-mono font-medium text-gray-900 whitespace-nowrap">
									{log.to}
								</td>

								<!-- Content (Bengali / English with tooltip) -->
								<td class="py-3 px-4 max-w-xs truncate text-gray-700" title={log.body}>
									{log.body}
								</td>

								<!-- Sender ID -->
								<td class="py-3 px-4 font-mono text-gray-700 whitespace-nowrap">
									{log.sender_id || '—'}
								</td>

								<!-- SMS Type (Transactional green pill / Promotional amber pill) -->
								<td class="py-3 px-4 whitespace-nowrap">
									{#if log.sms_type === 'Promotional'}
										<span class="inline-flex items-center rounded-md bg-amber-50 px-2.5 py-0.5 text-xs font-medium text-amber-700 border border-amber-200">
											Promotional
										</span>
									{:else}
										<span class="inline-flex items-center rounded-md bg-emerald-50 px-2.5 py-0.5 text-xs font-medium text-emerald-700 border border-emerald-200">
											Transactional
										</span>
									{/if}
								</td>

								<!-- Cost -->
								<td class="py-3 px-4 font-mono font-medium text-right text-gray-900 whitespace-nowrap">
									BDT {Number(log.cost).toFixed(2)}
								</td>

								<!-- Previous Balance -->
								<td class="py-3 px-4 font-mono font-medium text-right text-gray-900 whitespace-nowrap">
									BDT {Number(log.previous_balance).toFixed(2)}
								</td>
							</tr>
						{/each}
					{/if}
				</tbody>
			</table>
		</div>

		<!-- Filament Pagination (Matching Screenshot 9 & 10) -->
		<div class="flex flex-col sm:flex-row items-center justify-between border-t border-gray-200 px-4 py-3 bg-white text-xs text-gray-600 gap-3">
			<div>
				Showing {logs.length > 0 ? (currentPage - 1) * perPage + 1 : 0} to {Math.min(currentPage * perPage, meta.total)} of {meta.total} results
			</div>

			<div class="flex items-center gap-3">
				<!-- Per page dropdown -->
				<div class="flex items-center gap-1.5">
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

				<!-- Page Number Buttons (1 2 3 ... 31 32 >) -->
				<div class="flex items-center gap-1">
					{#each pageButtons as btn}
						{#if btn === '...'}
							<span class="px-2 py-1 text-gray-400">...</span>
						{:else}
							<button
								type="button"
								onclick={() => load(Number(btn))}
								class="min-w-7 h-7 rounded px-1.5 text-xs font-medium transition cursor-pointer {btn === currentPage
									? 'bg-teal-600 text-white font-bold'
									: 'border border-gray-200 text-gray-700 hover:bg-gray-100'}"
							>
								{btn}
							</button>
						{/if}
					{/each}
					{#if currentPage < totalPages}
						<button
							type="button"
							onclick={() => load(currentPage + 1)}
							class="min-w-7 h-7 rounded border border-gray-200 px-1.5 text-xs font-medium text-gray-700 hover:bg-gray-100 transition cursor-pointer"
						>
							&gt;
						</button>
					{/if}
				</div>
			</div>
		</div>
	</div>
</div>

<!-- Export As Modal -->
{#if exportModalOpen}
	<div class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4 animate-fade-in backdrop-blur-xs">
		<div class="relative w-full max-w-md rounded-2xl bg-white p-6 shadow-xl border border-gray-100">
			<h3 class="text-lg font-bold text-gray-900">Export Sent Message Logs</h3>
			<p class="mt-1 text-xs text-gray-500">
				Select your preferred report format to download.
			</p>

			<div class="mt-5 space-y-3">
				<label class="flex items-center gap-3 rounded-lg border border-gray-200 p-3 hover:bg-gray-50 cursor-pointer">
					<input
						type="radio"
						name="export-format"
						value="excel"
						bind:group={exportFormat}
						class="text-teal-600 focus:ring-teal-500"
					/>
					<div>
						<span class="text-xs font-semibold text-gray-900">Excel / CSV Spreadsheet (.csv)</span>
						<p class="text-[11px] text-gray-500">Compatible with Microsoft Excel, Google Sheets</p>
					</div>
				</label>
				<label class="flex items-center gap-3 rounded-lg border border-gray-200 p-3 hover:bg-gray-50 cursor-pointer">
					<input
						type="radio"
						name="export-format"
						value="pdf"
						bind:group={exportFormat}
						class="text-teal-600 focus:ring-teal-500"
					/>
					<div>
						<span class="text-xs font-semibold text-gray-900">PDF Document (.pdf)</span>
						<p class="text-[11px] text-gray-500">Printable formatted summary statement</p>
					</div>
				</label>
			</div>

			<div class="mt-6 flex items-center justify-end gap-3">
				<button
					type="button"
					onclick={() => (exportModalOpen = false)}
					class="rounded-lg border border-gray-300 bg-white px-4 py-2 text-xs font-semibold text-gray-700 hover:bg-gray-50 transition cursor-pointer"
				>
					Cancel
				</button>
				<button
					type="button"
					onclick={handleExport}
					disabled={exporting}
					class="rounded-lg bg-teal-600 px-4 py-2 text-xs font-semibold text-white shadow-sm hover:bg-teal-700 disabled:opacity-50 transition cursor-pointer"
				>
					{exporting ? 'Exporting…' : 'Download Report'}
				</button>
			</div>
		</div>
	</div>
{/if}

<!-- View Message Detail Modal -->
{#if selectedLog}
	<div class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4 animate-fade-in backdrop-blur-xs">
		<div class="relative w-full max-w-lg rounded-2xl bg-white p-6 shadow-xl border border-gray-100">
			<div class="flex items-center justify-between pb-3 border-b border-gray-200">
				<h3 class="text-base font-bold text-gray-900">Sent Message Detail</h3>
				<button
					type="button"
					onclick={() => (selectedLog = null)}
					class="text-gray-400 hover:text-gray-600 cursor-pointer"
				>
					<svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
						<path stroke-linecap="round" stroke-linejoin="round" d="M6 18 18 6M6 6l12 12" />
					</svg>
				</button>
			</div>

			<div class="mt-4 space-y-3 text-xs">
				<div class="flex justify-between py-1 border-b border-gray-50">
					<span class="text-gray-500 font-medium">Recipient Number</span>
					<span class="font-mono font-semibold text-gray-900">{selectedLog.to}</span>
				</div>
				<div class="flex justify-between py-1 border-b border-gray-50">
					<span class="text-gray-500 font-medium">Sender ID</span>
					<span class="font-mono text-gray-900">{selectedLog.sender_id}</span>
				</div>
				<div class="flex justify-between py-1 border-b border-gray-50">
					<span class="text-gray-500 font-medium">SMS Type</span>
					<span class="font-semibold text-teal-700">{selectedLog.sms_type}</span>
				</div>
				<div class="flex justify-between py-1 border-b border-gray-50">
					<span class="text-gray-500 font-medium">Cost</span>
					<span class="font-semibold text-gray-900">BDT {Number(selectedLog.cost).toFixed(2)}</span>
				</div>
				<div class="flex justify-between py-1 border-b border-gray-50">
					<span class="text-gray-500 font-medium">Previous Balance</span>
					<span class="font-semibold text-gray-900">BDT {Number(selectedLog.previous_balance).toFixed(2)}</span>
				</div>
				<div class="flex justify-between py-1 border-b border-gray-50">
					<span class="text-gray-500 font-medium">Status</span>
					<span class="capitalize font-semibold text-teal-700">{selectedLog.status}</span>
				</div>
				<div class="flex justify-between py-1 border-b border-gray-50">
					<span class="text-gray-500 font-medium">Date & Time</span>
					<span class="text-gray-700">{new Date(selectedLog.created_at).toLocaleString()}</span>
				</div>
				<div class="py-2">
					<span class="block text-gray-500 font-medium mb-1">Message Content</span>
					<div class="rounded-lg bg-gray-50 p-3 text-xs text-gray-800 leading-relaxed border border-gray-200">
						{selectedLog.body}
					</div>
				</div>
			</div>

			<div class="mt-6 flex justify-end">
				<button
					type="button"
					onclick={() => (selectedLog = null)}
					class="rounded-lg bg-gray-100 px-4 py-2 text-xs font-semibold text-gray-700 hover:bg-gray-200 transition cursor-pointer"
				>
					Close
				</button>
			</div>
		</div>
	</div>
{/if}
