<script lang="ts">
	import { onMount } from 'svelte';
	import { supermanApi } from '$lib/superman-api.js';
	import ExportDropdown from '$lib/components/common/ExportDropdown.svelte';
	import Pagination from '$lib/components/common/Pagination.svelte';
	import type { ExportColumn } from '$lib/utils/export.js';

	interface MessageItem {
		id: number;
		customer: string;
		recipient: string;
		body: string;
		status: string;
		cost: number;
		operator_cost?: number;
		primary_dipping_cost?: number;
		failure_reason?: string | null;
		source_ip?: string | null;
		gateway?: string | null;
		sender_id?: string | null;
		created_at: string;
	}

	let messages = $state<MessageItem[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let search = $state('');
	let statusFilter = $state('all');
	let currentPage = $state(1);
	let perPage = $state(10);
	let total = $state(0);

	let requestSeq = 0;

	function buildParams(): URLSearchParams {
		const params = new URLSearchParams();
		if (search.trim()) params.set('search', search.trim());
		if (statusFilter !== 'all') params.set('status', statusFilter);
		return params;
	}

	async function loadMessages(targetPage = currentPage) {
		const seq = ++requestSeq;
		currentPage = targetPage;
		loading = true;
		error = null;
		try {
			const params = buildParams();
			params.set('page', String(currentPage));
			params.set('per_page', String(perPage));

			const res = await supermanApi.get<{ data: MessageItem[]; meta?: { total: number } }>(
				`/api/v1/superman/messages?${params.toString()}`
			);
			if (seq !== requestSeq) return;
			messages = res.data ?? [];
			total = res.meta?.total ?? messages.length;

			const lastPage = Math.max(1, Math.ceil(total / perPage));
			if (currentPage > lastPage) {
				currentPage = lastPage;
				void loadMessages(lastPage);
			}
		} catch (err: unknown) {
			if (seq !== requestSeq) return;
			error = err instanceof Error ? err.message : 'Failed to load messages.';
		} finally {
			if (seq === requestSeq) loading = false;
		}
	}

	let searchTimer: ReturnType<typeof setTimeout> | undefined;
	function onSearchInput() {
		clearTimeout(searchTimer);
		searchTimer = setTimeout(() => void loadMessages(1), 350);
	}

	async function fetchAllForExport(): Promise<MessageItem[]> {
		const params = buildParams();
		params.set('export', '1');
		const res = await supermanApi.get<{ data: MessageItem[] }>(
			`/api/v1/superman/messages?${params.toString()}`
		);
		return res.data ?? [];
	}

	onMount(() => {
		void loadMessages(1);
	});

	const exportColumns: ExportColumn<MessageItem>[] = [
		{ header: 'Message ID', getValue: (m) => `#${m.id}` },
		{ header: 'Customer', getValue: (m) => m.customer },
		{ header: 'Recipient', getValue: (m) => m.recipient },
		{ header: 'Message Body', getValue: (m) => m.body },
		{ header: 'Status', getValue: (m) => m.status.toUpperCase() },
		{ header: 'Failure Reason', getValue: (m) => m.failure_reason || '—' },
		{ header: 'Cost (BDT)', getValue: (m) => `BDT ${Number(m.cost).toFixed(4)}` },
		{ header: 'Operator Cost (BDT)', getValue: (m) => `BDT ${Number(m.operator_cost ?? 0).toFixed(4)}` },
		{ header: 'Dipping Cost (BDT)', getValue: (m) => `BDT ${Number(m.primary_dipping_cost ?? 0).toFixed(4)}` },
		{ header: 'Source IP', getValue: (m) => m.source_ip || '—' },
		{ header: 'Timestamp', getValue: (m) => new Date(m.created_at).toLocaleString() }
	];
</script>

<svelte:head>
	<title>Messages — Superman Administrator</title>
</svelte:head>

<div class="space-y-6">
	<div class="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
		<div>
			<h1 class="text-2xl font-bold tracking-tight text-gray-950 dark:text-white">Messages Directory</h1>
			<p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
				Inspect, trace, and audit all live and archived SMS traffic across the platform.
			</p>
		</div>
		<div class="flex items-center gap-2">
			<button
				type="button"
				onclick={() => void loadMessages(currentPage)}
				class="rounded-lg border border-gray-200 bg-white px-3 py-2 text-xs font-medium text-gray-700 shadow-xs hover:bg-gray-50 dark:border-gray-800 dark:bg-gray-900 dark:text-gray-300"
			>
				Refresh Messages
			</button>
			<ExportDropdown
				title="Messages Directory"
				columns={exportColumns}
				getData={fetchAllForExport}
				disabled={total === 0}
			/>
		</div>
	</div>

	<!-- Filter / Search -->
	<form
		onsubmit={(e) => {
			e.preventDefault();
			void loadMessages(1);
		}}
		class="flex flex-wrap items-center gap-3 rounded-xl border border-gray-200 bg-white p-4 shadow-xs dark:border-gray-800 dark:bg-gray-900"
	>
		<input
			type="search"
			bind:value={search}
			oninput={onSearchInput}
			placeholder="Search recipient, customer, body, or gateway..."
			aria-label="Search messages"
			class="w-full max-w-xs rounded-lg border border-gray-200 bg-gray-50/70 px-3 py-1.5 text-xs outline-none focus:border-emerald-500 dark:border-gray-700 dark:bg-gray-800 dark:text-white"
		/>
		<select
			bind:value={statusFilter}
			onchange={() => void loadMessages(1)}
			aria-label="Status filter"
			class="rounded-lg border border-gray-200 bg-white px-3 py-1.5 text-xs text-gray-700 outline-none focus:border-emerald-500 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-300"
		>
			<option value="all">All Statuses</option>
			<option value="delivered">Delivered</option>
			<option value="sent">Sent</option>
			<option value="pending">Pending</option>
			<option value="sending">Sending</option>
			<option value="failed">Failed</option>
			<option value="dlr_failed">DLR Failed</option>
		</select>
	</form>

	<!-- Table -->
	<div class="overflow-hidden rounded-xl border border-gray-200 bg-white shadow-xs dark:border-gray-800 dark:bg-gray-900">
		{#if loading && messages.length === 0}
			<div class="py-16 text-center text-xs text-gray-500">Loading messages...</div>
		{:else if error}
			<div class="p-6 text-center text-xs text-rose-600">{error}</div>
		{:else if messages.length === 0}
			<div class="py-16 text-center text-xs text-gray-500">No messages found.</div>
		{:else}
			<div class="overflow-x-auto">
				<table class="w-full text-left text-xs">
					<thead>
						<tr class="border-b border-gray-100 bg-gray-50/50 text-[10px] font-semibold uppercase tracking-wider text-gray-500 dark:border-gray-800 dark:bg-gray-800/50 dark:text-gray-400">
							<th class="px-4 py-3">Customer</th>
							<th class="px-4 py-3">Recipient</th>
							<th class="px-4 py-3">Body</th>
							<th class="px-4 py-3">Status</th>
							<th class="px-4 py-3">Failure Reason</th>
							<th class="px-4 py-3 text-right">Cost</th>
							<th class="px-4 py-3 text-right">Operator Cost</th>
							<th class="px-4 py-3 text-right">Dipping Cost</th>
							<th class="px-4 py-3">Source IP</th>
							<th class="px-4 py-3 text-right">Created At</th>
						</tr>
					</thead>
					<tbody class="divide-y divide-gray-100 dark:divide-gray-800 {loading ? 'opacity-60' : ''}">
						{#each messages as m (m.id)}
							<tr class="hover:bg-gray-50/50 dark:hover:bg-gray-800/30">
								<td class="whitespace-nowrap px-4 py-3 font-semibold text-gray-950 dark:text-white">{m.customer}</td>
								<td class="whitespace-nowrap px-4 py-3 font-mono font-medium">{m.recipient}</td>
								<td class="max-w-xs truncate px-4 py-3 text-gray-600 dark:text-gray-400" title={m.body}>{m.body}</td>
								<td class="whitespace-nowrap px-4 py-3">
									<span class="inline-flex whitespace-nowrap rounded-full px-2 py-0.5 text-[10px] font-bold uppercase {m.status === 'delivered' ? 'bg-emerald-50 text-emerald-700 dark:bg-emerald-950/40 dark:text-emerald-400' : m.status === 'sent' ? 'bg-blue-50 text-blue-700 dark:bg-blue-950/40 dark:text-blue-400' : m.status === 'pending' || m.status === 'sending' ? 'bg-amber-50 text-amber-700 dark:bg-amber-950/40 dark:text-amber-400' : 'bg-rose-50 text-rose-700 dark:bg-rose-950/40 dark:text-rose-400'}">
										{m.status}
									</span>
								</td>
								<td class="max-w-50 truncate px-4 py-3 text-gray-500" title={m.failure_reason || ''}>{m.failure_reason || '—'}</td>
								<td class="whitespace-nowrap px-4 py-3 text-right font-medium tabular-nums text-gray-900 dark:text-white">৳{Number(m.cost).toFixed(4)}</td>
								<td class="whitespace-nowrap px-4 py-3 text-right font-medium tabular-nums text-gray-600 dark:text-gray-400">৳{Number(m.operator_cost ?? 0).toFixed(4)}</td>
								<td class="whitespace-nowrap px-4 py-3 text-right font-medium tabular-nums text-gray-600 dark:text-gray-400">৳{Number(m.primary_dipping_cost ?? 0).toFixed(4)}</td>
								<td class="whitespace-nowrap px-4 py-3 font-mono text-gray-500">{m.source_ip || '—'}</td>
								<td class="whitespace-nowrap px-4 py-3 text-right text-gray-400">{new Date(m.created_at).toLocaleString()}</td>
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
			onPageChange={(p) => void loadMessages(p)}
			onPerPageChange={(size) => {
				perPage = size;
				void loadMessages(1);
			}}
		/>
	</div>
</div>
