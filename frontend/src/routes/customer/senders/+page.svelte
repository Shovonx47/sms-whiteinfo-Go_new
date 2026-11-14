<script lang="ts">
	import { api } from '$lib/api.js';
	import Modal from '$lib/components/Modal.svelte';
	import { toast } from '$lib/components/Toast.svelte';
	import { exportToCSV, type ExportColumn } from '$lib/utils/export.js';
	import { onMount } from 'svelte';

	interface Sender {
		id: number;
		sender_id: string;
		type: string;
		status: string;
		created_at: string;
	}

	interface Meta {
		total: number;
		per_page: number;
		current_page: number;
		last_page?: number;
	}

	let senders = $state<Sender[]>([]);
	let meta = $state<Meta>({ total: 0, per_page: 10, current_page: 1, last_page: 1 });
	let loading = $state(true);
	let search = $state('');
	let perPage = $state(10);
	let currentPage = $state(1);

	// Request sender modal
	let requestModalOpen = $state(false);
	let requestedId = $state('');
	let requestedType = $state('non_masking');
	let submitting = $state(false);

	async function load(p = 1) {
		loading = true;
		currentPage = p;
		try {
			const params = new URLSearchParams({ page: String(p), per_page: String(perPage) });
			if (search.trim()) params.set('search', search.trim());
			const res = await api.get<{ data: Sender[]; meta: Meta }>(`/api/v1/customer/senders?${params}`);
			senders = res.data ?? [];
			meta = res.meta ?? { total: senders.length, per_page: perPage, current_page: p, last_page: 1 };
		} catch {
			// Zero records if API fails or empty - NO mock data
			senders = [];
			meta = { total: 0, per_page: perPage, current_page: 1, last_page: 1 };
		} finally {
			loading = false;
		}
	}

	onMount(() => load(1));

	let searchTimer: ReturnType<typeof setTimeout>;
	function handleSearchInput() {
		clearTimeout(searchTimer);
		searchTimer = setTimeout(() => load(1), 300);
	}

	async function handleRequestSender() {
		if (!requestedId.trim()) {
			toast.error('Validation error', 'Sender ID name is required.');
			return;
		}

		submitting = true;
		try {
			await api.post('/api/v1/customer/senders', {
				sender_id: requestedId.trim(),
				type: requestedType
			});
			toast.success('Sender ID Requested', 'Your request has been submitted.');
			requestModalOpen = false;
			requestedId = '';
			load(1);
		} catch (e: any) {
			toast.error('Request failed', e?.message ?? 'Please try again.');
		} finally {
			submitting = false;
		}
	}

	// Export helpers
	const exportColumns: ExportColumn<Sender>[] = [
		{ header: 'Sender ID', key: 'sender_id' },
		{ header: 'Type', getValue: (r) => (r.type?.toLowerCase().includes('masking') && !r.type?.toLowerCase().includes('non') ? 'Masking' : 'Numeric') },
		{ header: 'Status', getValue: (r) => (r.status?.toLowerCase() === 'approved' || r.status?.toLowerCase() === 'active' ? 'Active' : 'Pending') }
	];

	async function handleExportCSV() {
		try {
			const data = await api.get<{ data: Sender[] }>('/api/v1/customer/senders?per_page=1000');
			const all = data.data ?? senders;
			exportToCSV(exportColumns, all, { title: 'Sender ID List', filename: 'sender_id_list.csv' });
			toast.success('Export started', 'CSV download has started.');
		} catch {
			exportToCSV(exportColumns, senders, { title: 'Sender ID List', filename: 'sender_id_list.csv' });
		}
	}

	async function handleExportExcel() {
		try {
			const data = await api.get<{ data: Sender[] }>('/api/v1/customer/senders?per_page=1000');
			const all = data.data ?? senders;
			exportToCSV(exportColumns, all, { title: 'Sender ID List', filename: 'sender_id_list.xlsx.csv' });
			toast.success('Export started', 'Excel download has started.');
		} catch {
			exportToCSV(exportColumns, senders, { title: 'Sender ID List', filename: 'sender_id_list.xlsx.csv' });
		}
	}

	function formatSenderType(type?: string): string {
		if (!type) return 'Numeric';
		const lower = type.toLowerCase();
		if (lower === 'masking') return 'Masking';
		return 'Numeric';
	}

	function formatStatus(status?: string): { label: string; active: boolean } {
		const lower = (status ?? '').toLowerCase();
		if (lower === 'approved' || lower === 'active') {
			return { label: 'Active', active: true };
		}
		return { label: lower ? lower.charAt(0).toUpperCase() + lower.slice(1) : 'Pending', active: false };
	}
</script>

<svelte:head>
	<title>Sender ID List - White INFO</title>
</svelte:head>

<div class="space-y-6">
	<!-- Breadcrumb -->
	<nav class="flex items-center gap-2 text-xs text-gray-500 dark:text-gray-400">
		<a href="/messaging/senders" class="hover:text-gray-800 dark:hover:text-gray-200 transition">Sender ID List</a>
		<svg class="h-3 w-3 text-gray-400" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
			<path stroke-linecap="round" stroke-linejoin="round" d="M8.25 4.5l7.5 7.5-7.5 7.5" />
		</svg>
		<span class="text-gray-900 dark:text-gray-100 font-medium">List</span>
	</nav>

	<!-- Header with Action Buttons -->
	<div class="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
		<div>
			<h1 class="text-2xl font-bold tracking-tight text-gray-900 dark:text-gray-100">Sender ID List</h1>
		</div>
		<div class="flex flex-wrap items-center gap-2.5">
			<!-- Download All (Excel) -->
			<button
				type="button"
				onclick={handleExportExcel}
				class="inline-flex items-center gap-1.5 rounded-lg border border-gray-300 bg-white px-3.5 py-2 text-xs font-semibold text-gray-700 shadow-xs hover:bg-gray-50 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-200 cursor-pointer"
			>
				<svg class="h-4 w-4 text-emerald-600" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
					<path stroke-linecap="round" stroke-linejoin="round" d="M19.5 14.25v-2.625a3.375 3.375 0 0 0-3.375-3.375h-1.5A1.125 1.125 0 0 1 13.5 7.125v-1.5a3.375 3.375 0 0 0-3.375-3.375H8.25m.75 12 3 3m0 0 3-3m-3 3v-6m-1.5-9H5.625c-.621 0-1.125.504-1.125 1.125v17.25c0 .621.504 1.125 1.125 1.125h12.75c.621 0 1.125-.504 1.125-1.125V11.25a9 9 0 0 0-9-9Z" />
				</svg>
				<span>Download All (Excel)</span>
			</button>

			<!-- Download All (CSV) -->
			<button
				type="button"
				onclick={handleExportCSV}
				class="inline-flex items-center gap-1.5 rounded-lg border border-gray-300 bg-white px-3.5 py-2 text-xs font-semibold text-gray-700 shadow-xs hover:bg-gray-50 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-200 cursor-pointer"
			>
				<svg class="h-4 w-4 text-blue-600" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
					<path stroke-linecap="round" stroke-linejoin="round" d="M3 16.5v2.25A2.25 2.25 0 0 0 5.25 21h13.5A2.25 2.25 0 0 0 21 18.75V16.5M16.5 12 12 16.5m0 0L7.5 12m4.5 4.5V3" />
				</svg>
				<span>Download All (CSV)</span>
			</button>

			<!-- Request Sender ID -->
			<button
				type="button"
				onclick={() => (requestModalOpen = true)}
				class="inline-flex items-center gap-1.5 rounded-lg border border-teal-600 bg-white px-3.5 py-2 text-xs font-semibold text-teal-700 shadow-xs hover:bg-teal-50 dark:bg-gray-800 dark:text-teal-400 cursor-pointer"
			>
				<svg class="h-4 w-4 text-teal-600 dark:text-teal-400" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
					<path stroke-linecap="round" stroke-linejoin="round" d="M12 4.5v15m7.5-7.5h-15" />
				</svg>
				<span>Request Sender ID</span>
			</button>
		</div>
	</div>

	<!-- Main Table Card -->
	<div class="rounded-xl border border-gray-200 bg-white shadow-xs dark:border-gray-800 dark:bg-gray-900 overflow-hidden">
		<!-- Search & Filter Toolbar -->
		<div class="flex items-center justify-end gap-2 p-3.5 border-b border-gray-100 dark:border-gray-800">
			<div class="relative w-64">
				<input
					type="text"
					bind:value={search}
					oninput={handleSearchInput}
					placeholder="Search Sender ID..."
					class="w-full rounded-lg border border-gray-200 bg-white py-1.5 pl-8 pr-3 text-xs text-gray-800 placeholder-gray-400 focus:border-teal-500 focus:outline-hidden dark:border-gray-700 dark:bg-gray-800 dark:text-gray-200"
				/>
				<svg class="h-3.5 w-3.5 text-gray-400 absolute left-2.5 top-2.5" fill="none" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor">
					<path stroke-linecap="round" stroke-linejoin="round" d="m21 21-5.197-5.197m0 0A7.5 7.5 0 1 0 5.196 5.196a7.5 7.5 0 0 0 10.607 10.607Z" />
				</svg>
			</div>

			<!-- Column visibility toggle icon -->
			<button
				type="button"
				class="rounded-lg border border-gray-200 p-1.5 text-gray-500 hover:bg-gray-50 dark:border-gray-700 dark:text-gray-400 dark:hover:bg-gray-800"
				title="Toggle columns"
			>
				<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor">
					<path stroke-linecap="round" stroke-linejoin="round" d="M9 4.5v15m6-15v15m-10.875 0h15.75c.621 0 1.125-.504 1.125-1.125V5.625c0-.621-.504-1.125-1.125-1.125H4.125C3.504 4.5 3 5.021 3 5.625v12.75c0 .621.504 1.125 1.125 1.125Z" />
				</svg>
			</button>
		</div>

		<!-- Table -->
		<div class="overflow-x-auto">
			<table class="w-full text-left text-xs">
				<thead class="bg-gray-50 dark:bg-gray-800/80 border-b border-gray-200 dark:border-gray-700 text-gray-700 dark:text-gray-300 font-semibold">
					<tr>
						<th class="px-4 py-3">Sender ID</th>
						<th class="px-4 py-3">Type</th>
						<th class="px-4 py-3">Status</th>
					</tr>
				</thead>
				<tbody class="divide-y divide-gray-100 dark:divide-gray-800">
					{#if loading}
						<tr>
							<td colspan="3" class="px-4 py-8 text-center text-gray-400">
								<div class="flex items-center justify-center gap-2">
									<div class="h-4 w-4 animate-spin rounded-full border-2 border-teal-600 border-t-transparent"></div>
									<span>Loading Sender IDs…</span>
								</div>
							</td>
						</tr>
					{:else if senders.length === 0}
						<tr>
							<td colspan="3" class="px-4 py-12 text-center text-gray-500 dark:text-gray-400">
								No Sender IDs found.
							</td>
						</tr>
					{:else}
						{#each senders as s (s.id)}
							{@const st = formatStatus(s.status)}
							<tr class="hover:bg-gray-50/70 dark:hover:bg-gray-800/50 transition">
								<td class="px-4 py-3 font-mono font-medium text-gray-900 dark:text-gray-100">
									{s.sender_id}
								</td>
								<td class="px-4 py-3 text-gray-600 dark:text-gray-300">
									{formatSenderType(s.type)}
								</td>
								<td class="px-4 py-3">
									{#if st.active}
										<span class="inline-flex items-center gap-1 rounded-full bg-emerald-50 px-2 py-0.5 text-[11px] font-medium text-emerald-700 dark:bg-emerald-950/40 dark:text-emerald-400 border border-emerald-200 dark:border-emerald-800">
											<span class="h-1.5 w-1.5 rounded-full bg-emerald-500"></span>
											Active
										</span>
									{:else}
										<span class="inline-flex items-center gap-1 rounded-full bg-amber-50 px-2 py-0.5 text-[11px] font-medium text-amber-700 dark:bg-amber-950/40 dark:text-amber-400 border border-amber-200 dark:border-amber-800">
											<span class="h-1.5 w-1.5 rounded-full bg-amber-500"></span>
											{st.label}
										</span>
									{/if}
								</td>
							</tr>
						{/each}
					{/if}
				</tbody>
			</table>
		</div>

		<!-- Footer Pagination Toolbar (Filament Style) -->
		<div class="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3 p-3.5 border-t border-gray-100 dark:border-gray-800 text-xs text-gray-600 dark:text-gray-400">
			<div>
				{#if meta.total === 1}
					Showing 1 result
				{:else if meta.total > 1}
					Showing {((currentPage - 1) * perPage) + 1} to {Math.min(currentPage * perPage, meta.total)} of {meta.total} results
				{:else}
					No results
				{/if}
			</div>

			<div class="flex items-center gap-3">
				<div class="flex items-center gap-1.5">
					<span>Per page</span>
					<select
						bind:value={perPage}
						onchange={() => load(1)}
						class="rounded-md border border-gray-300 bg-white py-1 px-2 text-xs text-gray-800 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-200"
					>
						<option value={10}>10</option>
						<option value={25}>25</option>
						<option value={50}>50</option>
					</select>
				</div>
			</div>
		</div>
	</div>
</div>

<!-- Request Sender ID Modal -->
<Modal bind:open={requestModalOpen} title="Request Sender ID" description="Submit a new alphanumeric or numeric Sender ID for registration." size="md">
	<div class="space-y-4">
		<div>
			<label class="mb-1 block text-xs font-semibold text-gray-700 dark:text-gray-300" for="req-id">
				Sender ID Name <span class="text-red-500">*</span>
			</label>
			<input
				id="req-id"
				type="text"
				bind:value={requestedId}
				placeholder="e.g. BRANDNAME or 88096..."
				class="w-full rounded-lg border border-gray-300 bg-white p-2.5 text-xs text-gray-900 focus:border-teal-500 focus:outline-hidden dark:border-gray-700 dark:bg-gray-800 dark:text-gray-100"
			/>
		</div>

		<div>
			<label class="mb-1 block text-xs font-semibold text-gray-700 dark:text-gray-300" for="req-type">
				Type <span class="text-red-500">*</span>
			</label>
			<select
				id="req-type"
				bind:value={requestedType}
				class="w-full rounded-lg border border-gray-300 bg-white p-2.5 text-xs text-gray-900 focus:border-teal-500 focus:outline-hidden dark:border-gray-700 dark:bg-gray-800 dark:text-gray-100"
			>
				<option value="non_masking">Numeric / Non-masking</option>
				<option value="masking">Alphanumeric / Masking</option>
			</select>
		</div>

		<div class="flex justify-end gap-2 pt-3 border-t border-gray-100 dark:border-gray-800">
			<button
				type="button"
				onclick={() => (requestModalOpen = false)}
				class="rounded-lg border border-gray-300 px-4 py-2 text-xs font-medium text-gray-700 hover:bg-gray-50 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-300"
			>
				Cancel
			</button>
			<button
				type="button"
				onclick={handleRequestSender}
				disabled={submitting}
				class="inline-flex items-center gap-1.5 rounded-lg bg-teal-600 px-4 py-2 text-xs font-semibold text-white hover:bg-teal-500 disabled:opacity-50 cursor-pointer"
			>
				{#if submitting}
					<div class="h-3.5 w-3.5 animate-spin rounded-full border-2 border-white border-t-transparent"></div>
				{/if}
				<span>Submit Request</span>
			</button>
		</div>
	</div>
</Modal>
