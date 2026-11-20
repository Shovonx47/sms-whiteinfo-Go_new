<script lang="ts">
	import { onMount } from 'svelte';
	import { supermanApi } from '$lib/superman-api.js';
	import ExportDropdown from '$lib/components/common/ExportDropdown.svelte';
	import Pagination from '$lib/components/common/Pagination.svelte';
	import type { ExportColumn } from '$lib/utils/export.js';

	interface BatchApprovalItem {
		id: number;
		customer: string;
		customer_balance: number;
		sender_id?: string | null;
		body?: string;
		source: string;
		total_recipients: number;
		status: string;
		approval?: string | null;
		created_at: string;
	}

	let batches = $state<BatchApprovalItem[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let flash = $state<{ type: 'success' | 'error'; text: string } | null>(null);
	let flashTimer: ReturnType<typeof setTimeout> | undefined;

	let search = $state('');
	let approvalFilter = $state('pending');
	let sortKey = $state<'created_at' | 'customer' | 'customer_balance'>('created_at');
	let sortDir = $state<'asc' | 'desc'>('desc');
	let currentPage = $state(1);
	let perPage = $state(10);
	let total = $state(0);
	let actingId = $state<number | null>(null);

	let requestSeq = 0;

	function notify(type: 'success' | 'error', text: string) {
		flash = { type, text };
		clearTimeout(flashTimer);
		flashTimer = setTimeout(() => (flash = null), 5000);
	}

	function buildParams(): URLSearchParams {
		const params = new URLSearchParams({
			approval: approvalFilter,
			sort: sortKey,
			dir: sortDir
		});
		if (search.trim()) params.set('search', search.trim());
		return params;
	}

	async function loadApprovals(targetPage = currentPage) {
		const seq = ++requestSeq;
		currentPage = targetPage;
		loading = true;
		error = null;
		try {
			const params = buildParams();
			params.set('page', String(currentPage));
			params.set('per_page', String(perPage));

			const res = await supermanApi.get<{ data: BatchApprovalItem[]; meta?: { total: number } }>(
				`/api/v1/superman/batches?${params.toString()}`
			);
			if (seq !== requestSeq) return;
			batches = res.data ?? [];
			total = res.meta?.total ?? batches.length;

			const lastPage = Math.max(1, Math.ceil(total / perPage));
			if (currentPage > lastPage) {
				currentPage = lastPage;
				void loadApprovals(lastPage);
			}
		} catch (err: unknown) {
			if (seq !== requestSeq) return;
			error = err instanceof Error ? err.message : 'Failed to load approvals queue.';
		} finally {
			if (seq === requestSeq) loading = false;
		}
	}

	let searchTimer: ReturnType<typeof setTimeout> | undefined;
	function onSearchInput() {
		clearTimeout(searchTimer);
		searchTimer = setTimeout(() => void loadApprovals(1), 350);
	}

	function toggleSort(key: 'created_at' | 'customer' | 'customer_balance') {
		if (sortKey === key) {
			sortDir = sortDir === 'asc' ? 'desc' : 'asc';
		} else {
			sortKey = key;
			sortDir = key === 'customer' ? 'asc' : 'desc';
		}
		void loadApprovals(1);
	}

	async function approve(batch: BatchApprovalItem) {
		if (!window.confirm(`Approve BATCH-${batch.id} for ${batch.customer} (${batch.total_recipients} recipients)?`)) return;
		actingId = batch.id;
		try {
			const res = await supermanApi.post<{ message: string }>(`/api/v1/superman/batches/${batch.id}/approve`, {});
			notify('success', res.message || `BATCH-${batch.id} approved.`);
			void loadApprovals(currentPage);
		} catch (err: unknown) {
			notify('error', err instanceof Error ? err.message : 'Failed to approve batch.');
		} finally {
			actingId = null;
		}
	}

	async function reject(batch: BatchApprovalItem) {
		if (!window.confirm(`Reject BATCH-${batch.id} for ${batch.customer}?`)) return;
		actingId = batch.id;
		try {
			const res = await supermanApi.post<{ message: string }>(`/api/v1/superman/batches/${batch.id}/reject`, {});
			notify('success', res.message || `BATCH-${batch.id} rejected.`);
			void loadApprovals(currentPage);
		} catch (err: unknown) {
			notify('error', err instanceof Error ? err.message : 'Failed to reject batch.');
		} finally {
			actingId = null;
		}
	}

	async function fetchAllForExport(): Promise<BatchApprovalItem[]> {
		const params = buildParams();
		params.set('export', '1');
		const res = await supermanApi.get<{ data: BatchApprovalItem[] }>(
			`/api/v1/superman/batches?${params.toString()}`
		);
		return res.data ?? [];
	}

	onMount(() => {
		void loadApprovals(1);
	});

	const exportColumns: ExportColumn<BatchApprovalItem>[] = [
		{ header: 'Batch ID', getValue: (b) => `#${b.id}` },
		{ header: 'Customer', getValue: (b) => b.customer },
		{ header: 'Customer Balance', getValue: (b) => `BDT ${Number(b.customer_balance || 0).toFixed(2)}` },
		{ header: 'Sender ID', getValue: (b) => b.sender_id || 'Default' },
		{ header: 'Message', getValue: (b) => b.body || '—' },
		{ header: 'Recipients', getValue: (b) => b.total_recipients },
		{ header: 'Source', getValue: (b) => (b.source || 'direct').toUpperCase() },
		{ header: 'Approval', getValue: (b) => (b.approval || b.status || 'pending').toUpperCase() },
		{ header: 'Submitted', getValue: (b) => (b.created_at ? new Date(b.created_at).toLocaleString() : '—') }
	];

	function approvalBadge(state: string | null | undefined): string {
		const s = (state || 'pending').toLowerCase();
		if (s === 'approved') return 'bg-emerald-50 text-emerald-700 dark:bg-emerald-950/40 dark:text-emerald-400';
		if (s === 'rejected') return 'bg-rose-50 text-rose-700 dark:bg-rose-950/40 dark:text-rose-400';
		return 'bg-amber-50 text-amber-700 dark:bg-amber-950/40 dark:text-amber-400';
	}
</script>

<svelte:head>
	<title>Messaging Approvals — Superman Administrator</title>
</svelte:head>

<div class="space-y-6">
	<div class="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
		<div>
			<h1 class="text-2xl font-bold tracking-tight text-gray-950 dark:text-white">Message &amp; Campaign Approvals</h1>
			<p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
				Review high-volume promotional batches and outbound campaigns waiting for manual compliance sign-off.
			</p>
		</div>
		<div class="flex items-center gap-2">
			<button
				type="button"
				onclick={() => void loadApprovals(currentPage)}
				class="rounded-lg border border-gray-200 bg-white px-3 py-2 text-xs font-medium text-gray-700 shadow-xs hover:bg-gray-50 dark:border-gray-800 dark:bg-gray-900 dark:text-gray-300"
			>
				Refresh Queue
			</button>
			<ExportDropdown
				title="Messaging Approvals"
				columns={exportColumns}
				getData={fetchAllForExport}
				disabled={total === 0}
			/>
		</div>
	</div>

	{#if flash}
		<div
			role="status"
			class="flex items-center justify-between rounded-lg border px-4 py-2.5 text-xs font-medium {flash.type === 'success' ? 'border-emerald-200 bg-emerald-50 text-emerald-800 dark:border-emerald-900 dark:bg-emerald-950/30 dark:text-emerald-300' : 'border-rose-200 bg-rose-50 text-rose-800 dark:border-rose-900 dark:bg-rose-950/30 dark:text-rose-300'}"
		>
			<span>{flash.text}</span>
			<button type="button" onclick={() => (flash = null)} aria-label="Dismiss" class="ml-3 text-base leading-none opacity-60 hover:opacity-100">×</button>
		</div>
	{/if}

	<!-- Filter & Search -->
	<form
		onsubmit={(e) => {
			e.preventDefault();
			void loadApprovals(1);
		}}
		class="flex flex-wrap items-center gap-3 rounded-xl border border-gray-200 bg-white p-4 shadow-xs dark:border-gray-800 dark:bg-gray-900"
	>
		<input
			type="search"
			bind:value={search}
			oninput={onSearchInput}
			placeholder="Search batch ID, customer, sender ID, or message..."
			aria-label="Search approvals"
			class="w-full max-w-xs rounded-lg border border-gray-200 bg-gray-50/70 px-3 py-1.5 text-xs outline-none focus:border-emerald-500 dark:border-gray-700 dark:bg-gray-800 dark:text-white"
		/>
		<select
			bind:value={approvalFilter}
			onchange={() => void loadApprovals(1)}
			aria-label="Approval status filter"
			class="rounded-lg border border-gray-200 bg-white px-3 py-1.5 text-xs text-gray-700 outline-none focus:border-emerald-500 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-300"
		>
			<option value="pending">Pending Approval</option>
			<option value="approved">Approved</option>
			<option value="rejected">Rejected</option>
			<option value="any">All Approval Records</option>
		</select>
	</form>

	<div class="overflow-hidden rounded-xl border border-gray-200 bg-white shadow-xs dark:border-gray-800 dark:bg-gray-900">
		{#if loading && batches.length === 0}
			<div class="py-16 text-center text-xs text-gray-500">Scanning approval queue...</div>
		{:else if error}
			<div class="p-6 text-center text-xs text-rose-600">{error}</div>
		{:else if batches.length === 0}
			<div class="py-16 text-center text-xs text-gray-500">
				{approvalFilter === 'pending'
					? 'No campaigns or batches pending review. All dispatches approved.'
					: 'No approval records match your filters.'}
			</div>
		{:else}
			<div class="overflow-x-auto">
				<table class="w-full text-left text-xs">
					<thead>
						<tr class="border-b border-gray-100 bg-gray-50/50 text-[10px] font-semibold uppercase tracking-wider text-gray-500 dark:border-gray-800 dark:bg-gray-800/50 dark:text-gray-400">
							<th class="whitespace-nowrap px-4 py-3">Batch ID</th>
							<th class="whitespace-nowrap px-4 py-3">
								<button type="button" onclick={() => toggleSort('customer')} class="inline-flex items-center gap-1 uppercase">
									Customer
								</button>
							</th>
							<th class="whitespace-nowrap px-4 py-3 text-right">
								<button type="button" onclick={() => toggleSort('customer_balance')} class="inline-flex items-center gap-1 uppercase">
									Customer Balance
								</button>
							</th>
							<th class="whitespace-nowrap px-4 py-3">Sender ID</th>
							<th class="px-4 py-3">Message</th>
							<th class="whitespace-nowrap px-4 py-3 text-right">Recipients</th>
							<th class="whitespace-nowrap px-4 py-3">Source</th>
							<th class="whitespace-nowrap px-4 py-3">Approval</th>
							<th class="whitespace-nowrap px-4 py-3 text-right">
								<button type="button" onclick={() => toggleSort('created_at')} class="inline-flex items-center gap-1 uppercase">
									Submitted
								</button>
							</th>
							<th class="whitespace-nowrap px-4 py-3 text-right">Actions</th>
						</tr>
					</thead>
					<tbody class="divide-y divide-gray-100 dark:divide-gray-800 {loading ? 'opacity-60' : ''}">
						{#each batches as b (b.id)}
							<tr class="hover:bg-gray-50/50 dark:hover:bg-gray-800/30">
								<td class="whitespace-nowrap px-4 py-3 font-mono font-bold text-gray-900 dark:text-white">BATCH-{b.id}</td>
								<td class="whitespace-nowrap px-4 py-3 font-semibold text-gray-900 dark:text-white">{b.customer}</td>
								<td class="whitespace-nowrap px-4 py-3 text-right tabular-nums text-gray-700 dark:text-gray-300">
									BDT {Number(b.customer_balance || 0).toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 })}
								</td>
								<td class="whitespace-nowrap px-4 py-3 font-mono font-medium text-blue-600 dark:text-blue-400">{b.sender_id || 'Default'}</td>
								<td class="max-w-xs truncate px-4 py-3 text-gray-600 dark:text-gray-400" title={b.body || ''}>{b.body || '—'}</td>
								<td class="whitespace-nowrap px-4 py-3 text-right font-bold tabular-nums text-gray-900 dark:text-white">
									{Number(b.total_recipients || 0).toLocaleString()}
								</td>
								<td class="whitespace-nowrap px-4 py-3">
									<span class="inline-flex rounded-md bg-gray-100 px-2 py-0.5 text-[10px] font-semibold uppercase text-gray-700 dark:bg-gray-800 dark:text-gray-300">
										{b.source || 'direct'}
									</span>
								</td>
								<td class="whitespace-nowrap px-4 py-3">
									<span class="inline-flex whitespace-nowrap rounded-full px-2.5 py-0.5 text-[10px] font-bold uppercase {approvalBadge(b.approval)}">
										{b.approval || b.status || 'Pending'}
									</span>
								</td>
								<td class="whitespace-nowrap px-4 py-3 text-right text-gray-400">
									{b.created_at ? new Date(b.created_at).toLocaleString() : '—'}
								</td>
								<td class="whitespace-nowrap px-4 py-3 text-right">
									{#if (b.approval || '').toLowerCase() === 'pending'}
										<div class="inline-flex items-center gap-1.5">
											<button
												type="button"
												disabled={actingId === b.id}
												onclick={() => void approve(b)}
												class="rounded-md border border-emerald-600 px-2.5 py-1 text-[11px] font-semibold text-emerald-700 hover:bg-emerald-50 disabled:opacity-50 dark:text-emerald-400 dark:hover:bg-emerald-950/30"
											>
												Approve
											</button>
											<button
												type="button"
												disabled={actingId === b.id}
												onclick={() => void reject(b)}
												class="rounded-md border border-rose-300 px-2.5 py-1 text-[11px] font-semibold text-rose-600 hover:bg-rose-50 disabled:opacity-50 dark:border-rose-900 dark:text-rose-400 dark:hover:bg-rose-950/30"
											>
												Reject
											</button>
										</div>
									{:else}
										<span class="text-gray-400">—</span>
									{/if}
								</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</div>
		{/if}

		<Pagination
			{total}
			page={currentPage}
			{perPage}
			{loading}
			onPageChange={(p) => void loadApprovals(p)}
			onPerPageChange={(size) => {
				perPage = size;
				void loadApprovals(1);
			}}
		/>
	</div>
</div>
